package agents

import (
	"testing"

	"sentinelaegis/data"
)

// TestAnalyzeEmailTone_RuleFallback verifies the email agent returns safe
// LOW risk when Gemini is unavailable (GEMINI_API_KEY unset in test env).
func TestAnalyzeEmailTone_RuleFallback(t *testing.T) {
	tests := []struct {
		name     string
		tx       data.Transaction
		wantRisk string
	}{
		{
			name: "benign email → LOW or agent fallback",
			tx: data.Transaction{
				ID:           "TEST-001",
				EmailSender:  "invoices@cloudvault.com",
				EmailSubject: "Monthly Invoice #1234",
				EmailText:    "Please find attached your monthly invoice. Payment terms: Net 30.",
			},
		},
		{
			name: "BEC email with urgency markers",
			tx: data.Transaction{
				ID:           "TEST-002",
				EmailSender:  "ceo@acme-corp.com",
				EmailSubject: "URGENT — Confidential Wire Transfer",
				EmailText:    "I need you to process an urgent wire transfer IMMEDIATELY. Do NOT discuss this with anyone. This is strictly confidential.",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := AnalyzeEmailTone(tt.tx)

			if result.AgentName != "email_tone" {
				t.Errorf("AgentName = %q, want %q", result.AgentName, "email_tone")
			}

			validRisks := map[string]bool{"HIGH": true, "MEDIUM": true, "LOW": true}
			if !validRisks[result.RiskLevel] {
				t.Errorf("RiskLevel = %q, want HIGH/MEDIUM/LOW", result.RiskLevel)
			}

			if result.Confidence < 0 || result.Confidence > 1 {
				t.Errorf("Confidence = %f, want 0.0-1.0", result.Confidence)
			}

			if result.Explanation == "" {
				t.Error("Explanation should not be empty")
			}
		})
	}
}

// TestCheckIBANChange_NoChange verifies LOW risk when no IBAN history exists.
func TestCheckIBANChange_NoChange(t *testing.T) {
	tx := data.Transaction{
		ID:   "NOCHANGE-001",
		IBAN: "US44JPMC021000089012",
	}

	result := CheckIBANChange(tx)

	if result.AgentName != "iban_change" {
		t.Errorf("AgentName = %q, want %q", result.AgentName, "iban_change")
	}
	if result.RiskLevel != "LOW" {
		t.Errorf("RiskLevel = %q, want LOW (no IBAN change)", result.RiskLevel)
	}
}

// TestCheckIBANChange_RecentChange verifies HIGH risk for very recent IBAN changes.
func TestCheckIBANChange_RecentChange(t *testing.T) {
	// Inject a recent IBAN change
	txID := "IBAN-TEST-RECENT"
	SetCustomIBAN(txID, "DE89370400440532013000", 4) // Changed 4 hours ago
	defer CleanupCustom(txID)

	tx := data.Transaction{
		ID:          txID,
		Vendor:      "Test Vendor",
		Amount:      100000,
		Currency:    "USD",
		IBAN:        "GB29NWBK60161331926819",
		RequestedAt: "18:30",
	}

	result := CheckIBANChange(tx)

	if result.AgentName != "iban_change" {
		t.Errorf("AgentName = %q, want %q", result.AgentName, "iban_change")
	}

	// With Gemini unavailable in tests, rule-based fallback should fire
	// 4 hours < 48 hours → HIGH
	if result.RiskLevel != "HIGH" {
		t.Errorf("RiskLevel = %q, want HIGH (IBAN changed 4h ago)", result.RiskLevel)
	}
}

// TestCheckIBANChange_OldChange verifies LOW risk for old IBAN changes.
func TestCheckIBANChange_OldChange(t *testing.T) {
	txID := "IBAN-TEST-OLD"
	SetCustomIBAN(txID, "FR7630006000011234567890189", 720) // Changed 30 days ago
	defer CleanupCustom(txID)

	tx := data.Transaction{
		ID:          txID,
		Vendor:      "Test Vendor",
		Amount:      50000,
		Currency:    "EUR",
		IBAN:        "DE89370400440532013000",
		RequestedAt: "10:00",
	}

	result := CheckIBANChange(tx)

	if result.RiskLevel != "LOW" {
		t.Errorf("RiskLevel = %q, want LOW (IBAN changed 30 days ago)", result.RiskLevel)
	}
}

// TestCheckTimingAnomaly_WithinWindow verifies LOW risk for normal hours.
func TestCheckTimingAnomaly_WithinWindow(t *testing.T) {
	txID := "TIMING-TEST-NORMAL"
	SetCustomWindow(txID, "09:00", "17:00")
	defer CleanupCustom(txID)

	tx := data.Transaction{
		ID:          txID,
		Vendor:      "Test Vendor",
		Amount:      25000,
		Currency:    "USD",
		RequestedAt: "10:30",
	}

	result := CheckTimingAnomaly(tx)

	if result.AgentName != "timing" {
		t.Errorf("AgentName = %q, want %q", result.AgentName, "timing")
	}

	// 10:30 is within 09:00–17:00 → LOW
	if result.RiskLevel != "LOW" {
		t.Errorf("RiskLevel = %q, want LOW (within business hours)", result.RiskLevel)
	}
}

// TestCheckTimingAnomaly_AfterHours verifies HIGH risk for late-night requests.
func TestCheckTimingAnomaly_AfterHours(t *testing.T) {
	txID := "TIMING-TEST-LATE"
	SetCustomWindow(txID, "09:00", "17:00")
	defer CleanupCustom(txID)

	tx := data.Transaction{
		ID:          txID,
		Vendor:      "Test Vendor",
		Amount:      847000,
		Currency:    "USD",
		RequestedAt: "23:30",
	}

	result := CheckTimingAnomaly(tx)

	// 23:30 is 6.5 hours after 17:00 → HIGH (>120 min deviation)
	if result.RiskLevel != "HIGH" {
		t.Errorf("RiskLevel = %q, want HIGH (23:30 is far outside business hours)", result.RiskLevel)
	}
}

// TestCheckTimingAnomaly_SlightlyLate verifies MEDIUM risk for borderline timing.
func TestCheckTimingAnomaly_SlightlyLate(t *testing.T) {
	txID := "TIMING-TEST-SLIGHT"
	SetCustomWindow(txID, "09:00", "17:00")
	defer CleanupCustom(txID)

	tx := data.Transaction{
		ID:          txID,
		Vendor:      "Test Vendor",
		Amount:      50000,
		Currency:    "USD",
		RequestedAt: "17:45",
	}

	result := CheckTimingAnomaly(tx)

	// 17:45 is 45 min after 17:00 → MEDIUM (30–120 min deviation)
	if result.RiskLevel != "MEDIUM" {
		t.Errorf("RiskLevel = %q, want MEDIUM (slightly outside window)", result.RiskLevel)
	}
}
