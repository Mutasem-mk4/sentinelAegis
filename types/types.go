package types

import "sentinelaegis/agents"

// EmailData represents an email fetched from Gmail.
type EmailData struct {
	MessageID string `json:"message_id"`
	From      string `json:"from"`
	Subject   string `json:"subject"`
	Body      string `json:"body"`
	Date      string `json:"date"`
	ThreadID  string `json:"thread_id"`
}

// AnalysisEvent is broadcast via SSE to connected dashboard clients.
type AnalysisEvent struct {
	EventType   string                 `json:"event_type"` // email_received, analyzing, agent_result, consensus
	Timestamp   string                 `json:"timestamp"`
	Email       EmailData              `json:"email"`
	AgentResult *agents.AgentResult    `json:"agent_result,omitempty"`
	Consensus   *agents.ConsensusResult `json:"consensus,omitempty"`
	AgentIndex  int                    `json:"agent_index,omitempty"`
	RiskLevel   string                 `json:"risk_level,omitempty"`
	LatencyMs   int64                  `json:"latency_ms,omitempty"`
}
