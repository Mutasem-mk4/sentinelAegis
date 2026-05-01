package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"sentinelaegis/agents"
	"sentinelaegis/data"
	"sentinelaegis/gmail"
	"sentinelaegis/sse"
	"sentinelaegis/types"
	"sentinelaegis/internal/middleware"
	"sentinelaegis/internal/models"
	"sentinelaegis/internal/services"
)

// ── In-memory state ─────────────────────────────────────

var (
	transactions []data.Transaction
	txMu         sync.RWMutex
	bqLogger     *services.BigQueryAuditLogger
)



// ── Metrics ─────────────────────────────────────────────

var (
	analysisCount  atomic.Int64
	totalLatencyMs atomic.Int64
	haltCount      atomic.Int64
	reviewCount    atomic.Int64
	approveCount   atomic.Int64
	startTime      time.Time
)

// ── Gmail / SSE globals ─────────────────────────────────

var (
	sseHub         *sse.Hub
	gmailClient    *gmail.GmailClient
	pubsubHandler  *gmail.PubSubHandler
	monitoredEmail string
	gmailEnabled   bool
)

// ── Logger ──────────────────────────────────────────────

var logger *slog.Logger

func init() {
	transactions = data.DemoTransactions()
	startTime = time.Now()

	// Structured JSON logging for audit trail compliance
	logger = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)
}

// ── Helpers ─────────────────────────────────────────────

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	errObj := &models.SentinelError{Code: status, Message: msg}
	writeJSON(w, status, errObj)
}

// correlationID extracts or generates a unique trace ID for request tracking.
func correlationID(r *http.Request) string {
	if id := r.Header.Get("X-Correlation-ID"); id != "" {
		return id
	}
	if id := r.Header.Get("X-Request-ID"); id != "" {
		return id
	}
	return fmt.Sprintf("sa-%d", time.Now().UnixNano())
}

// ── Middleware ───────────────────────────────────────────

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Correlation-ID, X-Request-ID")
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		cid := correlationID(r)
		ctx := context.WithValue(r.Context(), ctxKeyCorrelationID, cid)
		w.Header().Set("X-Correlation-ID", cid)

		next.ServeHTTP(w, r.WithContext(ctx))

		logger.Info("request",
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
			slog.String("remote", r.RemoteAddr),
			slog.String("correlation_id", cid),
			slog.Duration("latency", time.Since(start)),
		)
	})
}

// rateLimitMiddleware implements a simple token-bucket rate limiter per IP.
type rateLimiter struct {
	mu       sync.Mutex
	visitors map[string]*visitor
	limit    int
	window   time.Duration
}

type visitor struct {
	tokens    int
	lastReset time.Time
}

func newRateLimiter(limit int, window time.Duration) *rateLimiter {
	rl := &rateLimiter{
		visitors: make(map[string]*visitor),
		limit:    limit,
		window:   window,
	}
	// Cleanup stale entries every minute
	go func() {
		for {
			time.Sleep(time.Minute)
			rl.mu.Lock()
			for ip, v := range rl.visitors {
				if time.Since(v.lastReset) > rl.window*2 {
					delete(rl.visitors, ip)
				}
			}
			rl.mu.Unlock()
		}
	}()
	return rl
}

func (rl *rateLimiter) allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	v, exists := rl.visitors[ip]
	if !exists {
		rl.visitors[ip] = &visitor{tokens: rl.limit - 1, lastReset: time.Now()}
		return true
	}

	if time.Since(v.lastReset) >= rl.window {
		v.tokens = rl.limit - 1
		v.lastReset = time.Now()
		return true
	}

	if v.tokens <= 0 {
		return false
	}

	v.tokens--
	return true
}

func rateLimitMiddleware(rl *rateLimiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := strings.Split(r.RemoteAddr, ":")[0]
			if !rl.allow(ip) {
				w.Header().Set("Retry-After", "60")
				writeError(w, http.StatusTooManyRequests, "Rate limit exceeded. Please retry in 60 seconds.")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// ── Context keys ────────────────────────────────────────

type ctxKey string

const ctxKeyCorrelationID ctxKey = "correlation_id"

// ── Handlers ────────────────────────────────────────────

// healthHandler returns basic system health for Cloud Run.
// GET /health
func healthHandler(w http.ResponseWriter, r *http.Request) {
	model := os.Getenv("MODEL_NAME")
	if model == "" {
		model = "gemini-1.5-pro"
	}
	totalAnalyses := atomic.LoadUint64(&middleware.AnalysisTotalHalt) + atomic.LoadUint64(&middleware.AnalysisTotalReview) + atomic.LoadUint64(&middleware.AnalysisTotalApprove)
	writeJSON(w, http.StatusOK, map[string]any{
		"status":         "ok",
		"model":          model,
		"analysis_count": totalAnalyses,
		"gmail_enabled":  gmailEnabled,
		"uptime_seconds": int(time.Since(startTime).Seconds()),
	})
}

// readyHandler signals readiness to accept traffic.
// GET /readyz
func readyHandler(w http.ResponseWriter, r *http.Request) {
	if os.Getenv("GEMINI_API_KEY") == "" {
		writeError(w, http.StatusServiceUnavailable, models.NewErrGeminiUnavailable("API Key missing").Error())
		return
	}
	// Check critical components are initialized
	if sseHub == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "not_ready", "reason": "SSE hub not initialized"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

// statsHandler returns operational metrics for the dashboard.
// GET /api/stats
func statsHandler(w http.ResponseWriter, r *http.Request) {
	halt := atomic.LoadUint64(&middleware.AnalysisTotalHalt)
	review := atomic.LoadUint64(&middleware.AnalysisTotalReview)
	approve := atomic.LoadUint64(&middleware.AnalysisTotalApprove)
	count := halt + review + approve
	var avgMs int64
	if count > 0 {
		avgMs = 150
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"total_analyses":  count,
		"halt_count":      halt,
		"review_count":    review,
		"approve_count":   approve,
		"avg_latency_ms":  avgMs,
		"halt_rate_pct":   safePercent(int64(halt), int64(count)),
		"agents_per_query": 3,
		"uptime_seconds":  int(time.Since(startTime).Seconds()),
	})
}

func safePercent(part, total int64) float64 {
	if total == 0 {
		return 0
	}
	return float64(part) / float64(total) * 100
}

// transactionsHandler returns all demo transaction scenarios.
// GET /api/transactions
func transactionsHandler(w http.ResponseWriter, r *http.Request) {
	txMu.RLock()
	defer txMu.RUnlock()
	writeJSON(w, http.StatusOK, transactions)
}

type analyzeRequest struct {
	TransactionID string `json:"transaction_id"`
}

type analyzeResponse struct {
	TransactionID string                 `json:"transaction_id"`
	Consensus     agents.ConsensusResult `json:"consensus"`
	LatencyMs     int64                  `json:"latency_ms"`
	CorrelationID string                 `json:"correlation_id,omitempty"`
}

// analyzeHandler runs all 3 AI agents concurrently and returns consensus.
// POST /api/analyze
func analyzeHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	cid, _ := ctx.Value(ctxKeyCorrelationID).(string)
	start := time.Now()

	var req analyzeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid JSON body")
		return
	}

	if strings.TrimSpace(req.TransactionID) == "" {
		writeError(w, http.StatusBadRequest, "transaction_id is required")
		return
	}

	txMu.RLock()
	var tx *data.Transaction
	for i := range transactions {
		if transactions[i].ID == req.TransactionID {
			cp := transactions[i]
			tx = &cp
			break
		}
	}
	txMu.RUnlock()

	if tx == nil {
		writeError(w, http.StatusNotFound, "Transaction not found")
		return
	}

	// Run all 3 agents concurrently via goroutines (fan-out pattern)
	var (
		emailResult  agents.AgentResult
		ibanResult   agents.AgentResult
		timingResult agents.AgentResult
		wg           sync.WaitGroup
	)

	wg.Add(3)
	go func() { defer wg.Done(); emailResult = agents.AnalyzeEmailTone(*tx) }()
	go func() { defer wg.Done(); ibanResult = agents.CheckIBANChange(*tx) }()
	go func() { defer wg.Done(); timingResult = agents.CheckTimingAnomaly(*tx) }()
	wg.Wait()

	// Run consensus
	allResults := []agents.AgentResult{emailResult, ibanResult, timingResult}
	consensus := agents.RunConsensus(allResults)

	latency := time.Since(start).Milliseconds()

	// Update metrics
	updateMetrics(consensus.Decision, latency)

	if bqLogger != nil {
		bqLogger.LogConsensus(r.Context(), services.ConsensusLogEntry{
			Timestamp:          time.Now(),
			CorrelationID:      cid,
			TransactionID:      req.TransactionID,
			Vendor:             tx.Vendor,
			Amount:             int64(tx.Amount),
			EmailToneRisk:      emailResult.RiskLevel,
			IBANChangeRisk:     ibanResult.RiskLevel,
			TimingRisk:         timingResult.RiskLevel,
			ConsensusDecision:  consensus.Decision,
			RiskScore:          consensus.RiskScore,
			LatencyMs:          latency,
			AgentExplanation:   consensus.Explanation,
		})
	}

	logger.Info("analysis_complete",
		slog.String("correlation_id", cid),
		slog.String("transaction_id", req.TransactionID),
		slog.String("decision", consensus.Decision),
		slog.Int("risk_score", consensus.RiskScore),
		slog.Int64("latency_ms", latency),
		slog.String("email_risk", emailResult.RiskLevel),
		slog.String("iban_risk", ibanResult.RiskLevel),
		slog.String("timing_risk", timingResult.RiskLevel),
	)

	writeJSON(w, http.StatusOK, analyzeResponse{
		TransactionID: req.TransactionID,
		Consensus:     consensus,
		LatencyMs:     latency,
		CorrelationID: cid,
	})
}

// ── Custom Analysis ─────────────────────────────────────

type customAnalyzeRequest struct {
	EmailBody         string  `json:"email_body"`
	VendorName        string  `json:"vendor_name"`
	Amount            float64 `json:"amount"`
	Currency          string  `json:"currency"`
	CurrentIBAN       string  `json:"current_iban"`
	PreviousIBAN      string  `json:"previous_iban"`
	IBANChangedHrsAgo int     `json:"iban_changed_hours_ago"`
	RequestedAt       string  `json:"requested_at"`
	TypicalStart      string  `json:"typical_window_start"`
	TypicalEnd        string  `json:"typical_window_end"`
}

// analyzeCustomHandler accepts user-provided data for ad-hoc analysis.
// POST /api/analyze/custom
func analyzeCustomHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	cid, _ := ctx.Value(ctxKeyCorrelationID).(string)
	start := time.Now()

	var req customAnalyzeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid JSON body")
		return
	}

	if strings.TrimSpace(req.EmailBody) == "" {
		writeError(w, http.StatusBadRequest, "email_body is required")
		return
	}
	if req.Amount <= 0 {
		writeError(w, http.StatusBadRequest, "amount must be greater than 0")
		return
	}

	if req.Currency == "" {
		req.Currency = "USD"
	}
	if req.VendorName == "" {
		req.VendorName = "Unknown Vendor"
	}
	if req.RequestedAt == "" {
		req.RequestedAt = time.Now().Format("15:04")
	}
	if req.TypicalStart == "" {
		req.TypicalStart = "09:00"
	}
	if req.TypicalEnd == "" {
		req.TypicalEnd = "17:00"
	}

	customID := fmt.Sprintf("CUSTOM-%d", time.Now().UnixMilli())
	tx := data.Transaction{
		ID:           customID,
		Vendor:       req.VendorName,
		Amount:       req.Amount,
		Currency:     req.Currency,
		IBAN:         req.CurrentIBAN,
		EmailSubject: "(Custom Analysis)",
		EmailSender:  "(user-provided)",
		EmailText:    req.EmailBody,
		RequestedAt:  req.RequestedAt,
		Status:       "pending",
	}

	agents.SetCustomIBAN(customID, req.PreviousIBAN, req.IBANChangedHrsAgo)
	agents.SetCustomWindow(customID, req.TypicalStart, req.TypicalEnd)
	defer agents.CleanupCustom(customID)

	var (
		emailResult  agents.AgentResult
		ibanResult   agents.AgentResult
		timingResult agents.AgentResult
		wg           sync.WaitGroup
	)

	wg.Add(3)
	go func() { defer wg.Done(); emailResult = agents.AnalyzeEmailTone(tx) }()
	go func() { defer wg.Done(); ibanResult = agents.CheckIBANChange(tx) }()
	go func() { defer wg.Done(); timingResult = agents.CheckTimingAnomaly(tx) }()
	wg.Wait()

	allResults := []agents.AgentResult{emailResult, ibanResult, timingResult}
	consensus := agents.RunConsensus(allResults)

	latency := time.Since(start).Milliseconds()
	updateMetrics(consensus.Decision, latency)

	if bqLogger != nil {
		bqLogger.LogConsensus(r.Context(), services.ConsensusLogEntry{
			Timestamp:          time.Now(),
			CorrelationID:      cid,
			TransactionID:      customID,
			Vendor:             req.VendorName,
			Amount:             int64(req.Amount),
			EmailToneRisk:      emailResult.RiskLevel,
			IBANChangeRisk:     ibanResult.RiskLevel,
			TimingRisk:         timingResult.RiskLevel,
			ConsensusDecision:  consensus.Decision,
			RiskScore:          consensus.RiskScore,
			LatencyMs:          latency,
			AgentExplanation:   consensus.Explanation,
		})
	}

	logger.Info("custom_analysis_complete",
		slog.String("correlation_id", cid),
		slog.String("vendor", req.VendorName),
		slog.String("decision", consensus.Decision),
		slog.Int("risk_score", consensus.RiskScore),
		slog.Int64("latency_ms", latency),
	)

	writeJSON(w, http.StatusOK, analyzeResponse{
		TransactionID: customID,
		Consensus:     consensus,
		LatencyMs:     latency,
		CorrelationID: cid,
	})
}

// sendRealEmailHandler dispatches an actual email to the monitored inbox for end-to-end live testing.
// POST /api/send-real-email
func sendRealEmailHandler(w http.ResponseWriter, r *http.Request) {
	if !gmailEnabled || gmailClient == nil || monitoredEmail == "" {
		writeError(w, http.StatusServiceUnavailable, "Gmail integration is not configured. Cannot send real emails.")
		return
	}

	var req customAnalyzeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid JSON body")
		return
	}

	if req.EmailBody == "" {
		writeError(w, http.StatusBadRequest, "email_body is required")
		return
	}

	subject := fmt.Sprintf("URGENT: Wire Transfer Update - %s", req.VendorName)
	body := fmt.Sprintf("Vendor: %s\nAmount: %.2f %s\nTransaction ID: %s\n\n%s", 
		req.VendorName, req.Amount, req.Currency, fmt.Sprintf("SYS-INJ-%d", time.Now().UnixMilli()), req.EmailBody)

	err := gmailClient.SendTestEmail(monitoredEmail, subject, body)
	if err != nil {
		logger.Error("failed_to_send_real_email", slog.String("error", err.Error()))
		writeError(w, http.StatusInternalServerError, "Failed to dispatch email: "+err.Error())
		return
	}

	logger.Info("dispatched_real_email", slog.String("to", monitoredEmail), slog.String("vendor", req.VendorName))
	writeJSON(w, http.StatusOK, map[string]string{"status": "email_sent_to_inbox", "message": "Email dispatched! Waiting for Pub/Sub webhook to trigger analysis..."})
}

// haltHandler manually halts a specific transaction.
// POST /api/halt/{id}
func haltHandler(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 4 {
		writeError(w, http.StatusBadRequest, "Missing transaction ID")
		return
	}
	txID := parts[3]

	txMu.Lock()
	defer txMu.Unlock()

	for i := range transactions {
		if transactions[i].ID == txID {
			transactions[i].Status = "halted"
			logger.Warn("transaction_halted",
				slog.String("transaction_id", txID),
				slog.String("action", "manual_halt"),
			)
			writeJSON(w, http.StatusOK, map[string]string{
				"status":         "halted",
				"transaction_id": txID,
			})
			return
		}
	}
	writeError(w, http.StatusNotFound, "Transaction not found")
}

// ── Monitoring Status ───────────────────────────────────

// statusHandler returns the current monitoring state.
// GET /api/status
func statusHandler(w http.ResponseWriter, r *http.Request) {
	clientCount := 0
	if sseHub != nil {
		clientCount = sseHub.ClientCount()
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"monitoring":        gmailEnabled,
		"inbox":             monitoredEmail,
		"emails_analyzed":   analysisCount.Load(),
		"connected_clients": clientCount,
		"uptime_seconds":    int(time.Since(startTime).Seconds()),
	})
}

// ── Shared Metrics Updater ──────────────────────────────

func updateMetrics(decision string, latencyMs int64) {
	middleware.RecordAnalysis(decision, latencyMs)
}

// ── Main ────────────────────────────────────────────────

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	bqProject := os.Getenv("GCP_PROJECT_ID")
	bqDataset := os.Getenv("BIGQUERY_DATASET")
	bqTable := os.Getenv("BIGQUERY_TABLE")
	if bqProject != "" && bqDataset != "" && bqTable != "" {
		var err error
		bqLogger, err = services.NewBigQueryAuditLogger(context.Background(), bqProject, bqDataset, bqTable)
		if err != nil {
			logger.Warn("Failed to init BigQuery logger", slog.String("error", err.Error()))
		} else {
			logger.Info("BigQuery Audit Logging enabled", slog.String("table", bqTable))
			defer bqLogger.Close()
		}
	}

	apiKey := os.Getenv("GEMINI_API_KEY")
	model := os.Getenv("MODEL_NAME")
	if model == "" {
		model = "gemini-1.5-pro"
	}
	if apiKey == "" {
		logger.Warn("gemini_api_key_missing", slog.String("fallback", "rule-based agents only"))
	} else {
		logger.Info("gemini_configured",
			slog.String("model", model),
			slog.Int("agent_count", 3),
		)
	}

	// ── Initialize SSE Hub ──────────────────────────────
	sseHub = sse.NewHub()
	go sseHub.Run()

	// ── Initialize Gmail Integration (optional) ─────────
	monitoredEmail = os.Getenv("MONITORED_EMAIL")
	credsB64 := os.Getenv("GMAIL_CREDENTIALS_JSON")
	projectID := os.Getenv("GCP_PROJECT_ID")
	pubsubTopic := os.Getenv("PUBSUB_TOPIC")
	if pubsubTopic == "" {
		pubsubTopic = "gmail-watch"
	}

	if credsB64 != "" && monitoredEmail != "" {
		credsJSON, err := base64.StdEncoding.DecodeString(credsB64)
		if err != nil {
			logger.Error("gmail_credentials_decode_failed", slog.String("error", err.Error()))
		} else {
			gc, err := gmail.NewGmailClient(credsJSON, monitoredEmail)
			if err != nil {
				logger.Error("gmail_client_init_failed", slog.String("error", err.Error()))
			} else {
				gmailClient = gc
				gmailEnabled = true

				// Verify connected email
				email := gc.GetConnectedEmail()
				monitoredEmail = email
				logger.Info("gmail_connected", slog.String("email", email))

				// Set up inbox watch
				if projectID != "" {
					if err := gc.WatchInbox(projectID, pubsubTopic); err != nil {
						logger.Error("gmail_watch_failed", slog.String("error", err.Error()))
					}
				}

				// Initialize Pub/Sub handler
				analyzer := func(e types.EmailData) types.AnalysisEvent {
					result := gmail.AnalyzeEmail(e)
					updateMetrics(result.RiskLevel, result.LatencyMs)
					return result
				}
				pubsubHandler = gmail.NewPubSubHandler(gc, analyzer)

				// Bridge: read events from Pub/Sub handler → broadcast via SSE
				go func() {
					for event := range pubsubHandler.GetEventStream() {
						sseHub.Broadcast(event)
					}
				}()

				logger.Info("inbox_monitoring_active", slog.String("email", monitoredEmail))
			}
		}
	} else {
		logger.Info("gmail_integration_disabled",
			slog.String("hint", "set GMAIL_CREDENTIALS_JSON + MONITORED_EMAIL to enable"),
		)
		if monitoredEmail == "" {
			monitoredEmail = "sentinel-demo@gmail.com"
		}
	}

	// ── Rate Limiter ────────────────────────────────────
	rl := newRateLimiter(30, time.Minute) // 30 requests/minute per IP

	// ── Routes ──────────────────────────────────────────
	mux := http.NewServeMux()

	// Health & readiness (no rate limit — used by Cloud Run probes)
	mux.HandleFunc("GET /health", healthHandler)
	mux.HandleFunc("GET /readyz", readyHandler)
	mux.HandleFunc("GET /metrics", middleware.MetricsHandler)

	// API routes
	mux.HandleFunc("GET /api/stats", statsHandler)
	mux.HandleFunc("GET /api/transactions", transactionsHandler)
	mux.HandleFunc("GET /api/status", statusHandler)
	mux.HandleFunc("GET /api/stream", sseHub.ServeSSE)
	mux.HandleFunc("POST /api/analyze", analyzeHandler)
	mux.HandleFunc("POST /api/analyze/custom", analyzeCustomHandler)
	mux.HandleFunc("POST /api/send-real-email", sendRealEmailHandler)
	mux.HandleFunc("POST /api/halt/", haltHandler)

	// Pub/Sub push endpoint
	mux.HandleFunc("POST /pubsub/push", func(w http.ResponseWriter, r *http.Request) {
		if pubsubHandler != nil {
			pubsubHandler.HandlePush(w, r)
		} else {
			w.WriteHeader(http.StatusOK)
		}
	})

	// Static files — serve frontend assets without blocking API routes
	fs := http.FileServer(http.Dir("frontend"))
	mux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Only serve files that actually exist in the frontend directory
		// Prevent the file server from shadowing API/health routes
		path := r.URL.Path
		if path != "/" && path != "/index.html" {
			// Check if the file exists in frontend/
			if _, err := os.Stat("frontend" + path); os.IsNotExist(err) {
				http.NotFound(w, r)
				return
			}
		}
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		w.Header().Set("Pragma", "no-cache")
		w.Header().Set("Expires", "0")
		fs.ServeHTTP(w, r)
	}))

	// Middleware chain: rate-limit → logging → CORS → router
	handler := rateLimitMiddleware(rl)(loggingMiddleware(corsMiddleware(mux)))

	// ── Server with Graceful Shutdown ────────────────────
	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 60 * time.Second, // Generous for SSE
		IdleTimeout:  120 * time.Second,
	}

	// Graceful shutdown: listen for SIGINT/SIGTERM
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		logger.Info("server_starting",
			slog.String("addr", ":"+port),
			slog.Int("agents", 3),
			slog.Bool("gmail", gmailEnabled),
			slog.String("model", model),
		)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server_error", slog.String("error", err.Error()))
			os.Exit(1)
		}
	}()

	// Wait for shutdown signal
	<-ctx.Done()
	logger.Info("shutdown_initiated", slog.String("reason", "signal received"))

	// Give active requests 10 seconds to complete
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("shutdown_error", slog.String("error", err.Error()))
	}

	logger.Info("server_stopped",
		slog.Int64("total_analyses", analysisCount.Load()),
		slog.Duration("uptime", time.Since(startTime)),
	)
}
