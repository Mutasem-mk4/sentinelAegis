package agents

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"sentinelaegis/data"
)

// vendorWindow defines a vendor's typical payment processing hours.
type vendorWindow struct {
	Start string // "HH:MM"
	End   string // "HH:MM"
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
		return 720 // default noon
	}
	h, _ := strconv.Atoi(parts[0])
	m, _ := strconv.Atoi(parts[1])
	return h*60 + m
}

// CheckTimingAnomaly assesses whether the payment time is suspicious.
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

	flags := []string{
		fmt.Sprintf("Payment requested at %s", tx.RequestedAt),
		fmt.Sprintf("Typical window: %s", typicalWindow),
	}

	// Significant deviation: > 2 hours outside window
	if deviationMins > 120 {
		devH := deviationMins / 60
		devM := deviationMins % 60
		flags = append(flags,
			fmt.Sprintf("Deviation: +%dh%02dm %s window", devH, devM, direction),
			"Significant deviation — common BEC tactic to avoid oversight",
		)
		return AgentResult{
			AgentName:      "timing",
			RiskLevel:      "HIGH",
			Confidence:     0.78,
			Flags:          flags,
			Explanation: fmt.Sprintf(
				"Payment was requested at %s, significantly outside the normal %s window. "+
					"Off-hours requests are a well-documented BEC tactic used to avoid scrutiny from colleagues.",
				tx.RequestedAt, typicalWindow),
			DeviationHours: math.Round(deviationHours*10) / 10,
			TypicalWindow:  typicalWindow,
		}
	}

	// Mild deviation: 30 min – 2 hours
	if deviationMins > 30 {
		flags = append(flags,
			fmt.Sprintf("Deviation: +%dm %s window", deviationMins, direction),
			"Slightly outside normal hours — warrants attention",
		)
		return AgentResult{
			AgentName:      "timing",
			RiskLevel:      "MEDIUM",
			Confidence:     0.55,
			Flags:          flags,
			Explanation: fmt.Sprintf(
				"Payment was requested at %s, slightly outside the typical %s window. "+
					"While not conclusive alone, this adds context when combined with other risk signals.",
				tx.RequestedAt, typicalWindow),
			DeviationHours: math.Round(deviationHours*10) / 10,
			TypicalWindow:  typicalWindow,
		}
	}

	// Within normal window
	flags = append(flags, "Request falls within normal payment hours")
	return AgentResult{
		AgentName:      "timing",
		RiskLevel:      "LOW",
		Confidence:     0.12,
		Flags:          flags,
		Explanation: fmt.Sprintf(
			"Payment was requested at %s, within the normal %s processing window. No timing anomaly detected.",
			tx.RequestedAt, typicalWindow),
		DeviationHours: 0,
		TypicalWindow:  typicalWindow,
	}
}
