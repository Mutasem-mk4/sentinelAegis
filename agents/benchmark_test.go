package agents

import (
	"testing"
)

// BenchmarkRunConsensus measures the performance of the consensus algorithm.
// This is a critical path — it runs on every analysis request.
func BenchmarkRunConsensus(b *testing.B) {
	inputs := []AgentResult{
		{AgentName: "email_tone", RiskLevel: "HIGH", Confidence: 0.92, Flags: []string{"urgency", "isolation"}, Explanation: "BEC indicators detected."},
		{AgentName: "iban_change", RiskLevel: "HIGH", Confidence: 0.91, Flags: []string{"IBAN changed 6h ago"}, Explanation: "Recent IBAN change."},
		{AgentName: "timing", RiskLevel: "MEDIUM", Confidence: 0.55, Flags: []string{"after hours"}, Explanation: "Slightly outside window."},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		RunConsensus(inputs)
	}
}

// BenchmarkRunConsensus_LowRisk benchmarks the fast-path (all LOW).
func BenchmarkRunConsensus_LowRisk(b *testing.B) {
	inputs := []AgentResult{
		{AgentName: "email_tone", RiskLevel: "LOW", Confidence: 0.10},
		{AgentName: "iban_change", RiskLevel: "LOW", Confidence: 0.15},
		{AgentName: "timing", RiskLevel: "LOW", Confidence: 0.12},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		RunConsensus(inputs)
	}
}
