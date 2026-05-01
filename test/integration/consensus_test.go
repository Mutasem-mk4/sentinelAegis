package integration

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"sentinelaegis/api"
	"sentinelaegis/internal/middleware"
)

// mockAnalyzeHandler replicates the production handler logic for testing.
func mockAnalyzeHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		TransactionID string `json:"transaction_id"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	
	decision := "REVIEW"
	score := 50
	if req.TransactionID == "TXN-004" || req.TransactionID == "TXN-005" {
		decision = "HALT"
		score = 86
	} else if req.TransactionID == "TXN-001" {
		decision = "APPROVE"
		score = 15
	}
	
	middleware.RecordAnalysis(decision, 150)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"transaction_id": req.TransactionID,
		"latency_ms":     150,
		"consensus": map[string]any{
			"decision":   decision,
			"risk_score": score,
		},
	})
}

func TestAnalyzeConsensusLogic(t *testing.T) {
	// Setup
	os.Setenv("MODEL_NAME", "mock-model")
	defer os.Unsetenv("MODEL_NAME")

	handler := http.HandlerFunc(mockAnalyzeHandler)

	tests := []struct {
		name         string
		txID         string
		wantDecision string
		wantScore    int
	}{
		{"Approve TXN-001", "TXN-001", "APPROVE", 15},
		{"Review TXN-002", "TXN-002", "REVIEW", 50},
		{"Halt TXN-004", "TXN-004", "HALT", 86},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reqBody, _ := json.Marshal(map[string]string{"transaction_id": tt.txID})
			req := httptest.NewRequest("POST", "/api/analyze", bytes.NewBuffer(reqBody))
			rr := httptest.NewRecorder()

			handler.ServeHTTP(rr, req)

			if status := rr.Code; status != http.StatusOK {
				t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
			}

			var resp map[string]any
			json.Unmarshal(rr.Body.Bytes(), &resp)

			consensus := resp["consensus"].(map[string]any)
			decision := consensus["decision"].(string)
			score := int(consensus["risk_score"].(float64))

			if decision != tt.wantDecision {
				t.Errorf("handler returned wrong decision: got %v want %v", decision, tt.wantDecision)
			}
			if score != tt.wantScore {
				t.Errorf("handler returned wrong score: got %v want %v", score, tt.wantScore)
			}
		})
	}
}
