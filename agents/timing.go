package agents

import (
	"fmt"
	"log"
	"math"
	"strconv"
	"strings"

	"sentinelaegis/data"
)

const timingSystemPrompt = `You are a fraud analyst specializing in behavioral pattern analysis for wire transfer timing.
Analyze the payment request timing against the organization's typical payment processing window.

Consider these factors:
1. Is the payment request outside normal business hours? Off-hours requests are a top BEC tactic.
2. How far outside the window is it? Minor deviations vs. significant after-hours requests.
3. Is the timing consistent with how BEC attackers operate (late evening, weekends, holidays)?
4. Could the timing be explained by legitimate reasons (time zones, urgent but real deadlines)?

Return ONLY a valid JSON object. No markdown. No preamble.
{
  "risk_level": "HIGH" or "MEDIUM" or "LOW",
  "confidence": <float 0.0 to 1.0>,
  "flags": ["specific timing observations"],
  "explanation": "2-3 sentence analyst summary of the timing risk"
}`

// vendorWindow defines a vendor's typical payment processing hours.
type vendorWindow struct {
	Start string
	End   string
}

// Mock typical payment windows keyed by transaction ID.
var vendorWindows = map[string]vendorWindow{
	"TXN-001": {Start: "09:00", End: "17:00"},
	"TXN-002": {Start: "08:00", End: "16:00"},
	"TXN-003": {Start: "09:00", End: "17:30"},
	"TXN-004": {Start: "09:00", End: "17:00"},
	"TXN-005": {Start: "09:00", End: "17:00"},
}

// parseHHMM converts "HH:MM" to minutes since midnight.
func parseHHMM(s string) int {
	parts := strings.Split(strings.TrimSpace(s), ":")
	if len(parts) != 2 {
		return 720
	}
	h, _ := strconv.Atoi(parts[0])
	m, _ := strconv.Atoi(parts[1])
	return h*60 + m
}

// CheckTimingAnomaly uses Gemini AI to analyze payment timing anomalies,
// with deterministic rule-based logic as a reliable fallback.
func CheckTimingAnomaly(tx data.Transaction) AgentResult {
	window, exists := vendorWindows[tx.ID]
	if !exists {
		window = vendorWindow{Start: "09:00", End: "17:00"}
	}

	reqMins := parseHHMM(tx.RequestedAt)
	startMins := parseHHMM(window.Start)
	endMins := parseHHMM(window.End)

	var deviationMins int
	var direction string
	if reqMins < startMins {
		deviationMins = startMins - reqMins
		direction = "before"
	} else if reqMins > endMins {
		deviationMins = reqMins - endMins
		direction = "after"
	} else {
		deviationMins = 0
		direction = "within"
	}

	deviationHours := float64(deviationMins) / 60.0
	typicalWindow := fmt.Sprintf("%s–%s", window.Start, window.End)

	// Call Gemini for AI-enhanced analysis
	geminiPrompt := fmt.Sprintf(
		"Wire transfer request details:\nVendor: %s\nAmount: $%.2f %s\nPayment requested at: %s\nTypical payment window for this vendor: %s\nDeviation: %d minutes %s the window\n\nContext: This organization typically processes payments for this vendor between %s. The current request was submitted at %s.",
		tx.Vendor, tx.Amount, tx.Currency, tx.RequestedAt, typicalWindow, deviationMins, direction, typicalWindow, tx.RequestedAt,
	)

	aiResult, err := CallGemini(geminiPrompt, timingSystemPrompt)
	if err == nil {
		log.Printf("[timing] Gemini analysis complete: %s (%.0f%%)", aiResult.RiskLevel, aiResult.Confidence*100)
		// Merge AI flags with deterministic timing facts
		mergedFlags := []string{
			fmt.Sprintf("Payment requested at %s", tx.RequestedAt),
			fmt.Sprintf("Typical window: %s", typicalWindow),
		}
		if deviationMins > 0 {
			devH := deviationMins / 60
			devM := deviationMins % 60
			if devH > 0 {
				mergedFlags = append(mergedFlags, fmt.Sprintf("Deviation: +%dh%02dm %s window", devH, devM, direction))
			} else {
				mergedFlags = append(mergedFlags, fmt.Sprintf("Deviation: +%dm %s window", deviationMins, direction))
			}
		}
		mergedFlags = append(mergedFlags, aiResult.Flags...)
		return AgentResult{
			AgentName:      "timing",
			RiskLevel:      aiResult.RiskLevel,
			Confidence:     aiResult.Confidence,
			Flags:          mergedFlags,
			Explanation:    aiResult.Explanation,
			DeviationHours: math.Round(deviationHours*10) / 10,
			TypicalWindow:  typicalWindow,
		}
	}

	// Gemini failed — fall back to deterministic rules
	log.Printf("[timing] Gemini unavailable (%v), using rule-based fallback", err)
	return timingRuleFallback(tx, deviationMins, direction, deviationHours, typicalWindow)
}

// timingRuleFallback provides deterministic timing analysis.
func timingRuleFallback(tx data.Transaction, devMins int, direction string, devHours float64, window string) AgentResult {
	flags := []string{
		fmt.Sprintf("Payment requested at %s", tx.RequestedAt),
		fmt.Sprintf("Typical window: %s", window),
	}

	if devMins > 120 {
		devH := devMins / 60
		devM := devMins % 60
		flags = append(flags,
			fmt.Sprintf("Deviation: +%dh%02dm %s window", devH, devM, direction),
			"Significant deviation — common BEC tactic to avoid oversight",
		)
		return AgentResult{
			AgentName:  "timing", RiskLevel: "HIGH", Confidence: 0.78,
			Flags: flags,
			Explanation: fmt.Sprintf("Payment requested at %s, significantly outside %s window. Off-hours requests are a documented BEC tactic.", tx.RequestedAt, window),
			DeviationHours: math.Round(devHours*10) / 10, TypicalWindow: window,
		}
	}

	if devMins > 30 {
		flags = append(flags, fmt.Sprintf("Deviation: +%dm %s window", devMins, direction))
		return AgentResult{
			AgentName:  "timing", RiskLevel: "MEDIUM", Confidence: 0.55,
			Flags: flags,
			Explanation: fmt.Sprintf("Payment requested at %s, slightly outside %s window. Adds context with other risk signals.", tx.RequestedAt, window),
			DeviationHours: math.Round(devHours*10) / 10, TypicalWindow: window,
		}
	}

	flags = append(flags, "Request falls within normal payment hours")
	return AgentResult{
		AgentName:  "timing", RiskLevel: "LOW", Confidence: 0.12,
		Flags: flags,
		Explanation: fmt.Sprintf("Payment requested at %s, within normal %s window. No anomaly detected.", tx.RequestedAt, window),
		DeviationHours: 0, TypicalWindow: window,
	}
}
