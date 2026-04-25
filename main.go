package main

import (
	"encoding/json"
	"log"
	"net/http"
	"fmt"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"sentinelaegis/agents"
	"sentinelaegis/data"
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
	analysisCount.Add(1)
	totalLatencyMs.Add(latency)
	switch consensus.Decision {
	case "HALT":
		haltCount.Add(1)
	case "REVIEW":
		reviewCount.Add(1)
	case "APPROVE":
		approveCount.Add(1)
	}

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

	// Validate required fields
	if strings.TrimSpace(req.EmailBody) == "" {
		writeError(w, http.StatusBadRequest, "email_body is required")
		return
	}
	if req.Amount <= 0 {
		writeError(w, http.StatusBadRequest, "amount must be greater than 0")
		return
	}

	// Defaults
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

	// Build a synthetic transaction with a unique ID
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

	// Inject IBAN history and vendor window for this custom transaction
	agents.SetCustomIBAN(customID, req.PreviousIBAN, req.IBANChangedHrsAgo)
	agents.SetCustomWindow(customID, req.TypicalStart, req.TypicalEnd)
	defer agents.CleanupCustom(customID)

	// Run all 3 agents concurrently — same fan-out as /api/analyze
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

	// Run consensus
	allResults := []agents.AgentResult{emailResult, ibanResult, timingResult}
	consensus := agents.RunConsensus(allResults)

	latency := time.Since(start).Milliseconds()

	// Update metrics
	analysisCount.Add(1)
	totalLatencyMs.Add(latency)
	switch consensus.Decision {
	case "HALT":
		haltCount.Add(1)
	case "REVIEW":
		reviewCount.Add(1)
	case "APPROVE":
		approveCount.Add(1)
	}

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

	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", healthHandler)
	mux.HandleFunc("GET /api/stats", statsHandler)
	mux.HandleFunc("GET /api/transactions", transactionsHandler)
	mux.HandleFunc("POST /api/analyze", analyzeHandler)
	mux.HandleFunc("POST /api/analyze/custom", analyzeCustomHandler)
	mux.HandleFunc("POST /api/halt/", haltHandler)

	fs := http.FileServer(http.Dir("frontend"))
	mux.Handle("GET /", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		w.Header().Set("Pragma", "no-cache")
		w.Header().Set("Expires", "0")
		fs.ServeHTTP(w, r)
	}))

	handler := loggingMiddleware(corsMiddleware(mux))

	log.Printf("🛡️ SentinelAegis starting on :%s (3 AI agents, consensus engine)", port)
	if err := http.ListenAndServe(":"+port, handler); err != nil {
		log.Fatal(err)
	}
}
