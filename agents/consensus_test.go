package agents

import (
	"testing"
)

func TestRunConsensus(t *testing.T) {
	tests := []struct {
		name             string
		agents           []AgentResult
		expectedDecision string
		minScore         int
		maxScore         int
	}{
		{
			name: "3 HIGH → HALT",
			agents: []AgentResult{
				{AgentName: "email_tone", RiskLevel: "HIGH", Confidence: 0.92},
				{AgentName: "iban_change", RiskLevel: "HIGH", Confidence: 0.91},
				{AgentName: "timing", RiskLevel: "HIGH", Confidence: 0.78},
			},
			expectedDecision: "HALT",
			minScore:         85,
			maxScore:         100,
		},
		{
			name: "2 HIGH + 1 LOW → HALT",
			agents: []AgentResult{
				{AgentName: "email_tone", RiskLevel: "HIGH", Confidence: 0.90},
				{AgentName: "iban_change", RiskLevel: "HIGH", Confidence: 0.85},
				{AgentName: "timing", RiskLevel: "LOW", Confidence: 0.12},
			},
			expectedDecision: "HALT",
			minScore:         85,
			maxScore:         100,
		},
		{
			name: "1 HIGH + 1 MEDIUM + 1 LOW → REVIEW",
			agents: []AgentResult{
				{AgentName: "email_tone", RiskLevel: "HIGH", Confidence: 0.80},
				{AgentName: "iban_change", RiskLevel: "MEDIUM", Confidence: 0.72},
				{AgentName: "timing", RiskLevel: "LOW", Confidence: 0.12},
			},
			expectedDecision: "REVIEW",
			minScore:         45,
			maxScore:         75,
		},
		{
			name: "1 HIGH + 2 LOW → REVIEW",
			agents: []AgentResult{
				{AgentName: "email_tone", RiskLevel: "HIGH", Confidence: 0.75},
				{AgentName: "iban_change", RiskLevel: "LOW", Confidence: 0.15},
				{AgentName: "timing", RiskLevel: "LOW", Confidence: 0.12},
			},
			expectedDecision: "REVIEW",
			minScore:         45,
			maxScore:         75,
		},
		{
			name: "3 LOW → APPROVE",
			agents: []AgentResult{
				{AgentName: "email_tone", RiskLevel: "LOW", Confidence: 0.10},
				{AgentName: "iban_change", RiskLevel: "LOW", Confidence: 0.15},
				{AgentName: "timing", RiskLevel: "LOW", Confidence: 0.12},
			},
			expectedDecision: "APPROVE",
			minScore:         0,
			maxScore:         30,
		},
		{
			name: "3 MEDIUM → APPROVE",
			agents: []AgentResult{
				{AgentName: "email_tone", RiskLevel: "MEDIUM", Confidence: 0.55},
				{AgentName: "iban_change", RiskLevel: "MEDIUM", Confidence: 0.60},
				{AgentName: "timing", RiskLevel: "MEDIUM", Confidence: 0.50},
			},
			expectedDecision: "APPROVE",
			minScore:         0,
			maxScore:         30,
		},
		{
			name: "2 HIGH + 1 MEDIUM → HALT (maximum signals)",
			agents: []AgentResult{
				{AgentName: "email_tone", RiskLevel: "HIGH", Confidence: 0.95},
				{AgentName: "iban_change", RiskLevel: "HIGH", Confidence: 0.91},
				{AgentName: "timing", RiskLevel: "MEDIUM", Confidence: 0.55},
			},
			expectedDecision: "HALT",
			minScore:         85,
			maxScore:         100,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RunConsensus(tt.agents)

			if result.Decision != tt.expectedDecision {
				t.Errorf("Decision = %q, want %q", result.Decision, tt.expectedDecision)
			}

			if result.RiskScore < tt.minScore || result.RiskScore > tt.maxScore {
				t.Errorf("RiskScore = %d, want between %d and %d", result.RiskScore, tt.minScore, tt.maxScore)
			}

			if result.Explanation == "" {
				t.Error("Explanation should not be empty")
			}

			if len(result.AgentBreakdown) != len(tt.agents) {
				t.Errorf("AgentBreakdown length = %d, want %d", len(result.AgentBreakdown), len(tt.agents))
			}
		})
	}
}

func TestRunConsensus_EmptyInput(t *testing.T) {
	result := RunConsensus([]AgentResult{})
	if result.Decision != "APPROVE" {
		t.Errorf("Empty input should APPROVE, got %q", result.Decision)
	}
}
