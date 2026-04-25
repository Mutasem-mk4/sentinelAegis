package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"sentinelaegis/agents"
	"sentinelaegis/data"
	"sentinelaegis/gmail"
	"sentinelaegis/sse"
	"sentinelaegis/types"
)

// ── In-memory state ─────────────────────────────────────

var (
	transactions []data.Transaction
	txMu         sync.RWMutex
)

// ── Metrics ─────────────────────────────────────────────

var (
	analysisCount  atomic.Int64
	totalLatencyMs atomic.Int64
	haltCount      atomic.Int64
	reviewCount    atomic.Int64
	approveCount   atomic.Int64
)

// ── Gmail / SSE globals ─────────────────────────────────

var (
	sseHub         *sse.Hub
	gmailClient    *gmail.GmailClient
	pubsubHandler  *gmail.PubSubHandler
	monitoredEmail string
	gmailEnabled   bool
)

func init() {
	transactions = data.DemoTransactions()
}

// ── Helpers ─────────────────────────────────────────────

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// ── Middleware ───────────────────────────────────────────

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
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
		next.ServeHTTP(w, r)
		latency := time.Since(start)
		log.Printf("[%s] %s %s (%s)", r.Method, r.URL.Path, r.RemoteAddr, latency.Round(time.Millisecond))
	})
}

// ── Handlers ────────────────────────────────────────────

func healthHandler(w http.ResponseWriter, r *http.Request) {
	model := os.Getenv("MODEL_NAME")
	if model == "" {
		model = "gemini-1.5-pro"
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":         "ok",
		"model":          model,
		"analysis_count": analysisCount.Load(),
		"gmail_enabled":  gmailEnabled,
	})
}

func statsHandler(w http.ResponseWriter, r *http.Request) {
	count := analysisCount.Load()
	var avgMs int64
	if count > 0 {
		avgMs = totalLatencyMs.Load() / count
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"total_analyses":   count,
		"halt_count":       haltCount.Load(),
		"review_count":     reviewCount.Load(),
		"approve_count":    approveCount.Load(),
		"avg_latency_ms":   avgMs,
		"halt_rate_pct":    safePercent(haltCount.Load(), count),
		"agents_per_query": 3,
	})
}

func safePercent(part, total int64) float64 {
	if total == 0 {
		return 0
	}
	return float64(part) / float64(total) * 100
}

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
}

func analyzeHandler(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	var req analyzeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid JSON body")
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

	// Run all 3 agents concurrently via goroutines
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

	log.Printf("[ANALYSIS] %s → %s (score: %d, latency: %dms)", req.TransactionID, consensus.Decision, consensus.RiskScore, latency)

	writeJSON(w, http.StatusOK, analyzeResponse{
		TransactionID: req.TransactionID,
		Consensus:     consensus,
		LatencyMs:     latency,
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

func analyzeCustomHandler(w http.ResponseWriter, r *http.Request) {
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

	log.Printf("[CUSTOM] %s → %s (score: %d, latency: %dms)", req.VendorName, consensus.Decision, consensus.RiskScore, latency)

	writeJSON(w, http.StatusOK, analyzeResponse{
		TransactionID: customID,
		Consensus:     consensus,
		LatencyMs:     latency,
	})
}

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
			log.Printf("[HALT] Transaction %s HALTED", txID)
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
	})
}

// ── Shared Metrics Updater ──────────────────────────────

func updateMetrics(decision string, latencyMs int64) {
	analysisCount.Add(1)
	totalLatencyMs.Add(latencyMs)
	switch decision {
	case "HALT":
		haltCount.Add(1)
	case "REVIEW":
		reviewCount.Add(1)
	case "APPROVE":
		approveCount.Add(1)
	}
}

// ── Main ────────────────────────────────────────────────

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	apiKey := os.Getenv("GEMINI_API_KEY")
	model := os.Getenv("MODEL_NAME")
	if model == "" {
		model = "gemini-1.5-pro"
	}
	if apiKey == "" {
		log.Println("⚠️  GEMINI_API_KEY not set — all agents will use rule-based fallback")
	} else {
		log.Printf("✅ GEMINI_API_KEY configured (model: %s, 3 AI agents active)", model)
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
			log.Printf("⚠️  Failed to decode GMAIL_CREDENTIALS_JSON: %v", err)
		} else {
			gc, err := gmail.NewGmailClient(credsJSON, monitoredEmail)
			if err != nil {
				log.Printf("⚠️  Gmail client init failed: %v", err)
			} else {
				gmailClient = gc
				gmailEnabled = true

				// Verify connected email
				email := gc.GetConnectedEmail()
				monitoredEmail = email
				log.Printf("📧 Connected to Gmail: %s", email)

				// Set up inbox watch
				if projectID != "" {
					if err := gc.WatchInbox(projectID, pubsubTopic); err != nil {
						log.Printf("⚠️  Gmail watch setup failed: %v", err)
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

				log.Printf("🛡️ Monitoring inbox: %s — LIVE", monitoredEmail)
			}
		}
	} else {
		log.Println("ℹ️  Gmail integration disabled (set GMAIL_CREDENTIALS_JSON + MONITORED_EMAIL to enable)")
		if monitoredEmail == "" {
			monitoredEmail = "sentinel-demo@gmail.com"
		}
	}

	// ── Routes ──────────────────────────────────────────
	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", healthHandler)
	mux.HandleFunc("GET /api/stats", statsHandler)
	mux.HandleFunc("GET /api/transactions", transactionsHandler)
	mux.HandleFunc("GET /api/status", statusHandler)
	mux.HandleFunc("GET /api/stream", sseHub.ServeSSE)
	mux.HandleFunc("POST /api/analyze", analyzeHandler)
	mux.HandleFunc("POST /api/analyze/custom", analyzeCustomHandler)
	mux.HandleFunc("POST /api/halt/", haltHandler)

	// Pub/Sub push endpoint
	mux.HandleFunc("POST /pubsub/push", func(w http.ResponseWriter, r *http.Request) {
		if pubsubHandler != nil {
			pubsubHandler.HandlePush(w, r)
		} else {
			w.WriteHeader(http.StatusOK)
		}
	})

	// Static files
	fs := http.FileServer(http.Dir("frontend"))
	mux.Handle("GET /", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		w.Header().Set("Pragma", "no-cache")
		w.Header().Set("Expires", "0")
		fs.ServeHTTP(w, r)
	}))

	handler := loggingMiddleware(corsMiddleware(mux))

	log.Printf("🛡️ SentinelAegis starting on :%s (3 AI agents, consensus engine, SSE hub)", port)
	if err := http.ListenAndServe(":"+port, handler); err != nil {
		log.Fatal(err)
	}
}
