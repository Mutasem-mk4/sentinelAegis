package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"

	"sentinelaegis/agents"
	"sentinelaegis/data"
)

// In-memory transaction store.
var (
	transactions []data.Transaction
	txMu         sync.RWMutex
)

func init() {
	transactions = data.DemoTransactions()
}

// JSON response helpers

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// CORS middleware
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

// ──────────────────────────────────────────────
// Handlers
// ──────────────────────────────────────────────

func healthHandler(w http.ResponseWriter, r *http.Request) {
	model := os.Getenv("MODEL_NAME")
	if model == "" {
		model = "gemini-1.5-pro"
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"status": "ok",
		"model":  model,
	})
}

func transactionsHandler(w http.ResponseWriter, r *http.Request) {
	txMu.RLock()
	defer txMu.RUnlock()

	// Return transactions without full email bodies (for list view)
	type txSummary struct {
		ID           string  `json:"id"`
		Vendor       string  `json:"vendor"`
		Amount       float64 `json:"amount"`
		Currency     string  `json:"currency"`
		IBAN         string  `json:"iban"`
		EmailSubject string  `json:"email_subject"`
		EmailSender  string  `json:"email_sender"`
		EmailText    string  `json:"email_text"`
		RequestedAt  string  `json:"requested_at"`
		Status       string  `json:"status"`
	}

	summaries := make([]txSummary, len(transactions))
	for i, tx := range transactions {
		summaries[i] = txSummary{
			ID:           tx.ID,
			Vendor:       tx.Vendor,
			Amount:       tx.Amount,
			Currency:     tx.Currency,
			IBAN:         tx.IBAN,
			EmailSubject: tx.EmailSubject,
			EmailSender:  tx.EmailSender,
			EmailText:    tx.EmailText,
			RequestedAt:  tx.RequestedAt,
			Status:       tx.Status,
		}
	}
	writeJSON(w, http.StatusOK, summaries)
}

type analyzeRequest struct {
	TransactionID string `json:"transaction_id"`
}

type analyzeResponse struct {
	TransactionID string                `json:"transaction_id"`
	Consensus     agents.ConsensusResult `json:"consensus"`
}

func analyzeHandler(w http.ResponseWriter, r *http.Request) {
	var req analyzeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid JSON body")
		return
	}

	// Find the transaction
	txMu.RLock()
	var tx *data.Transaction
	for i := range transactions {
		if transactions[i].ID == req.TransactionID {
			tx = &transactions[i]
			break
		}
	}
	txMu.RUnlock()

	if tx == nil {
		writeError(w, http.StatusNotFound, "Transaction not found")
		return
	}

	// Run all 3 agents.
	// Email tone runs via Gemini (takes ~2-5s).
	// IBAN and timing are instant but we run them concurrently for the pattern.
	var (
		emailResult  agents.AgentResult
		ibanResult   agents.AgentResult
		timingResult agents.AgentResult
		wg           sync.WaitGroup
	)

	wg.Add(3)

	go func() {
		defer wg.Done()
		emailResult = agents.AnalyzeEmailTone(*tx)
	}()

	go func() {
		defer wg.Done()
		ibanResult = agents.CheckIBANChange(*tx)
	}()

	go func() {
		defer wg.Done()
		timingResult = agents.CheckTimingAnomaly(*tx)
	}()

	wg.Wait()

	// Run consensus
	allResults := []agents.AgentResult{emailResult, ibanResult, timingResult}
	consensus := agents.RunConsensus(allResults)

	log.Printf("Analysis: %s → %s (score: %d)", req.TransactionID, consensus.Decision, consensus.RiskScore)

	writeJSON(w, http.StatusOK, analyzeResponse{
		TransactionID: req.TransactionID,
		Consensus:     consensus,
	})
}

func haltHandler(w http.ResponseWriter, r *http.Request) {
	// Extract transaction ID from path: /api/halt/{id}
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
			log.Printf("Transaction %s HALTED", txID)
			writeJSON(w, http.StatusOK, map[string]string{
				"status":         "halted",
				"transaction_id": txID,
			})
			return
		}
	}

	writeError(w, http.StatusNotFound, "Transaction not found")
}

// ──────────────────────────────────────────────
// Main
// ──────────────────────────────────────────────

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// Startup checks
	apiKey := os.Getenv("GEMINI_API_KEY")
	model := os.Getenv("MODEL_NAME")
	if model == "" {
		model = "gemini-1.5-pro"
	}
	if apiKey == "" {
		log.Println("⚠️  GEMINI_API_KEY is not set — Email Tone Agent will use fallback mode")
	} else {
		log.Printf("✅ GEMINI_API_KEY is configured (model: %s)", model)
	}

	mux := http.NewServeMux()

	// API routes
	mux.HandleFunc("GET /health", healthHandler)
	mux.HandleFunc("GET /api/transactions", transactionsHandler)
	mux.HandleFunc("POST /api/analyze", analyzeHandler)
	mux.HandleFunc("POST /api/halt/", haltHandler)

	// Serve frontend static files
	frontendDir := http.Dir("frontend")
	fileServer := http.FileServer(frontendDir)
	mux.Handle("GET /", fileServer)

	handler := corsMiddleware(mux)

	log.Printf("🛡️ SentinelAegis starting on :%s", port)
	if err := http.ListenAndServe(":"+port, handler); err != nil {
		log.Fatal(err)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
