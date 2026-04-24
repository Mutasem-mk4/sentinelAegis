package agents

import (
	"fmt"
	"strings"
)

// AgentResult is the unified result type returned by all agents.
type AgentResult struct {
	AgentName      string   `json:"agent_name"`
	RiskLevel      string   `json:"risk_level"` // HIGH, MEDIUM, LOW
	Confidence     float64  `json:"confidence"`
	Flags          []string `json:"flags,omitempty"`
	Explanation    string   `json:"explanation"`
	DeviationHours float64  `json:"deviation_hours,omitempty"`
	TypicalWindow  string   `json:"typical_window,omitempty"`
}

// ConsensusResult is the output of the 2-of-3 voting engine.
type ConsensusResult struct {
	Decision       string        `json:"decision"` // HALT, REVIEW, APPROVE
	RiskScore      int           `json:"risk_score"`
	Explanation    string        `json:"explanation"`
	AgentBreakdown []AgentResult `json:"agent_breakdown"`
}

// RunConsensus applies 2-of-3 voting across agent results.
//
// Rules:
//   - 2+ HIGH → HALT  (risk_score 85-100)
//   - 1 HIGH  → REVIEW (risk_score 45-75)
//   - 0 HIGH  → APPROVE (risk_score 0-30)
//
// Score formula: (HIGH_count × 40) + (MEDIUM_count × 20) + avg(confidence) × 10
func RunConsensus(results []AgentResult) ConsensusResult {
	highCount := 0
	medCount := 0
	totalConf := 0.0

	var highAgents []string
	var medAgents []string

	nameMap := map[string]string{
		"email_tone":  "Email Tone Analysis",
		"iban_change": "IBAN Change Detection",
		"timing":      "Timing Anomaly Detection",
	}

	for _, r := range results {
		totalConf += r.Confidence
		switch r.RiskLevel {
		case "HIGH":
			highCount++
			highAgents = append(highAgents, nameMap[r.AgentName])
		case "MEDIUM":
			medCount++
			medAgents = append(medAgents, nameMap[r.AgentName])
		}
	}

	avgConf := totalConf / float64(max(len(results), 1))
	rawScore := float64(highCount*40) + float64(medCount*20) + avgConf*10
	riskScore := int(rawScore)
	if riskScore > 100 {
		riskScore = 100
	}

	var decision, explanation string

	switch {
	case highCount >= 2:
		decision = "HALT"
		if riskScore < 85 {
			riskScore = 85
		}
		flagged := strings.Join(highAgents, ", ")
		explanation = fmt.Sprintf(
			"TRANSACTION HALTED. %d of %d agents flagged HIGH risk (risk score: %d/100). "+
				"Flagged by: %s. ",
			highCount, len(results), riskScore, flagged,
		)
		if len(results) > 0 {
			explanation += results[0].Explanation[:min(len(results[0].Explanation), 120)]
			if !strings.HasSuffix(explanation, ".") {
				explanation += "."
			}
		}
		explanation += " This transaction requires immediate manual review and should not be processed until verified through an out-of-band channel."

	case highCount == 1:
		decision = "REVIEW"
		if riskScore < 45 {
			riskScore = 45
		}
		if riskScore > 75 {
			riskScore = 75
		}
		highName := "Unknown Agent"
		if len(highAgents) > 0 {
			highName = highAgents[0]
		}
		var supporting string
		if len(medAgents) > 0 {
			supporting = fmt.Sprintf(", with supporting concerns from %s", strings.Join(medAgents, ", "))
		}
		explanation = fmt.Sprintf(
			"MANUAL REVIEW REQUIRED. %s flagged HIGH risk%s (risk score: %d/100). "+
				"Recommend secondary verification before processing.",
			highName, supporting, riskScore,
		)

	default:
		decision = "APPROVE"
		if riskScore > 30 {
			riskScore = 30
		}
		explanation = fmt.Sprintf(
			"TRANSACTION APPROVED. All agents reported acceptable risk levels "+
				"(risk score: %d/100). No significant BEC indicators detected. "+
				"Standard processing may proceed.",
			riskScore,
		)
	}

	return ConsensusResult{
		Decision:       decision,
		RiskScore:      riskScore,
		Explanation:    explanation,
		AgentBreakdown: results,
	}
}
