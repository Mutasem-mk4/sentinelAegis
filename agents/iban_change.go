package agents

import (
	"fmt"
	"log"

	"sentinelaegis/data"
)

const ibanSystemPrompt = `You are a fraud analyst specializing in payment security and IBAN verification.
Analyze the following banking detail change in the context of a wire transfer request.

Consider these factors:
1. How recently was the IBAN changed? Changes within 48 hours are critical BEC indicators.
2. Did the IBAN change cross country borders (e.g., DE→GB, US→CH)? Cross-border changes are higher risk.
3. Is the change consistent with the stated business reason?
4. Does the pattern match known IBAN-swap BEC attack techniques?

Return ONLY a valid JSON object. No markdown. No preamble.
{
  "risk_level": "HIGH" or "MEDIUM" or "LOW",
  "confidence": <float 0.0 to 1.0>,
  "flags": ["specific observations about the IBAN change"],
  "explanation": "2-3 sentence analyst summary of the IBAN risk"
}`

// ibanRecord holds mock IBAN change history for a transaction.
type ibanRecord struct {
	PreviousIBAN    string
	ChangedHoursAgo int
}

// Mock IBAN history keyed by transaction ID.
var ibanHistory = map[string]ibanRecord{
	"TXN-001": {PreviousIBAN: "", ChangedHoursAgo: 0},
	"TXN-002": {PreviousIBAN: "", ChangedHoursAgo: 0},
	"TXN-003": {PreviousIBAN: "US33BOFA026009593524", ChangedHoursAgo: 120},
	"TXN-004": {PreviousIBAN: "DE89370400440532013000", ChangedHoursAgo: 6},
	"TXN-005": {PreviousIBAN: "FR7630006000011234567890189", ChangedHoursAgo: 3},
}

// CheckIBANChange uses Gemini AI to analyze IBAN change risk,
// with deterministic rule-based logic as a reliable fallback.
func CheckIBANChange(tx data.Transaction) AgentResult {
	record, exists := ibanHistory[tx.ID]

	// No IBAN change on record — low risk via rules, skip Gemini
	if !exists || record.PreviousIBAN == "" {
		return AgentResult{
			AgentName:  "iban_change",
			RiskLevel:  "LOW",
			Confidence: 0.15,
			Flags: []string{
				fmt.Sprintf("Current IBAN: %s", tx.IBAN),
				"No IBAN changes on record",
				"Beneficiary banking details are stable",
			},
			Explanation: "No recent IBAN changes detected. The beneficiary's banking details have been consistent, posing minimal risk.",
		}
	}

	// IBAN was changed — call Gemini for AI-enhanced analysis
	hours := record.ChangedHoursAgo
	geminiPrompt := fmt.Sprintf(
		"Wire transfer to: %s\nAmount: $%.2f %s\nCurrent IBAN: %s\nPrevious IBAN: %s\nIBAN changed: %d hours ago\nPayment requested at: %s\n\nContext: The beneficiary's banking details were updated %d hours before this payment request was submitted.",
		tx.Vendor, tx.Amount, tx.Currency, tx.IBAN, record.PreviousIBAN, hours, tx.RequestedAt, hours,
	)

	aiResult, err := CallGemini(geminiPrompt, ibanSystemPrompt)
	if err == nil {
		log.Printf("[iban_change] Gemini analysis complete: %s (%.0f%%)", aiResult.RiskLevel, aiResult.Confidence*100)
		// Merge AI flags with our deterministic facts
		mergedFlags := []string{
			fmt.Sprintf("IBAN changed %d hours ago", hours),
			fmt.Sprintf("Previous: %s", record.PreviousIBAN),
			fmt.Sprintf("Current:  %s", tx.IBAN),
		}
		mergedFlags = append(mergedFlags, aiResult.Flags...)
		return AgentResult{
			AgentName:   "iban_change",
			RiskLevel:   aiResult.RiskLevel,
			Confidence:  aiResult.Confidence,
			Flags:       mergedFlags,
			Explanation: aiResult.Explanation,
		}
	}

	// Gemini failed — fall back to deterministic rules
	log.Printf("[iban_change] Gemini unavailable (%v), using rule-based fallback", err)
	return ibanRuleFallback(tx, record)
}

// ibanRuleFallback provides deterministic IBAN change analysis.
func ibanRuleFallback(tx data.Transaction, record ibanRecord) AgentResult {
	hours := record.ChangedHoursAgo
	flags := []string{
		fmt.Sprintf("IBAN changed %d hours ago", hours),
		fmt.Sprintf("Previous: %s", record.PreviousIBAN),
		fmt.Sprintf("Current:  %s", tx.IBAN),
	}

	if hours <= 48 {
		flags = append(flags, "Change within critical 48-hour window", "Matches BEC IBAN-swap attack pattern")
		return AgentResult{
			AgentName:  "iban_change",
			RiskLevel:  "HIGH",
			Confidence: 0.91,
			Flags:      flags,
			Explanation: fmt.Sprintf(
				"The beneficiary IBAN was changed only %d hours before this payment request. "+
					"This is a critical indicator of a BEC IBAN-swap attack.", hours),
		}
	}

	if hours <= 168 {
		days := float64(hours) / 24.0
		flags = append(flags, fmt.Sprintf("Changed %.1f days ago — within 7-day window", days))
		return AgentResult{
			AgentName:  "iban_change",
			RiskLevel:  "MEDIUM",
			Confidence: 0.72,
			Flags:      flags,
			Explanation: fmt.Sprintf(
				"The beneficiary IBAN was changed %.1f days ago. Recent changes warrant secondary verification.", days),
		}
	}

	days := float64(hours) / 24.0
	flags = append(flags, fmt.Sprintf("Changed %.0f days ago — outside risk window", days))
	return AgentResult{
		AgentName:  "iban_change",
		RiskLevel:  "LOW",
		Confidence: 0.15,
		Flags:      flags,
		Explanation: fmt.Sprintf("IBAN was changed %.0f days ago, well outside the risk window.", days),
	}
}
