package agents

import (
	"fmt"

	"sentinelaegis/data"
)

// ibanRecord holds mock IBAN change history for a transaction.
type ibanRecord struct {
	PreviousIBAN    string
	ChangedHoursAgo int
}

// Mock IBAN history keyed by transaction ID.
// This simulates a banking backend lookup.
var ibanHistory = map[string]ibanRecord{
	"TXN-001": {PreviousIBAN: "", ChangedHoursAgo: 0},                        // No change on record
	"TXN-002": {PreviousIBAN: "", ChangedHoursAgo: 0},                        // No change on record
	"TXN-003": {PreviousIBAN: "US33BOFA026009593524", ChangedHoursAgo: 120},   // Changed 5 days ago
	"TXN-004": {PreviousIBAN: "DE89370400440532013000", ChangedHoursAgo: 6},   // Changed 6 hours ago!
	"TXN-005": {PreviousIBAN: "FR7630006000011234567890189", ChangedHoursAgo: 3}, // Changed 3 hours ago!
}

// CheckIBANChange assesses IBAN change risk using mock history data.
func CheckIBANChange(tx data.Transaction) AgentResult {
	record, exists := ibanHistory[tx.ID]

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

	hours := record.ChangedHoursAgo
	flags := []string{
		fmt.Sprintf("IBAN changed %d hours ago", hours),
		fmt.Sprintf("Previous: %s", record.PreviousIBAN),
		fmt.Sprintf("Current:  %s", tx.IBAN),
	}

	// Critical window: changed within 48 hours
	if hours <= 48 {
		flags = append(flags,
			"Change within critical 48-hour window",
			"Matches BEC IBAN-swap attack pattern",
		)
		return AgentResult{
			AgentName:  "iban_change",
			RiskLevel:  "HIGH",
			Confidence: 0.91,
			Flags:      flags,
			Explanation: fmt.Sprintf(
				"The beneficiary IBAN was changed only %d hours before this payment request. "+
					"This is a critical indicator of a BEC IBAN-swap attack, where fraudsters redirect "+
					"wire transfers to accounts they control.", hours),
		}
	}

	// Elevated window: changed within 7 days (168 hours)
	if hours <= 168 {
		days := float64(hours) / 24.0
		flags = append(flags,
			fmt.Sprintf("Changed %.1f days ago — outside 48h but within 7 days", days),
			"Recommend secondary verification with beneficiary",
		)
		return AgentResult{
			AgentName:  "iban_change",
			RiskLevel:  "MEDIUM",
			Confidence: 0.72,
			Flags:      flags,
			Explanation: fmt.Sprintf(
				"The beneficiary IBAN was changed %.1f days ago. While outside the critical 48-hour "+
					"window, recent changes still warrant secondary verification through an established "+
					"communication channel.", days),
		}
	}

	// Old change — low risk
	days := float64(hours) / 24.0
	flags = append(flags, fmt.Sprintf("Changed %.0f days ago — well outside risk window", days))
	return AgentResult{
		AgentName:  "iban_change",
		RiskLevel:  "LOW",
		Confidence: 0.15,
		Flags:      flags,
		Explanation: fmt.Sprintf(
			"IBAN was changed %.0f days ago, well outside the risk window. "+
				"Normal banking detail updates occur periodically.", days),
	}
}
