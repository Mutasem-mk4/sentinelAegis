# Trace View: Multi-Agent Consensus

This document traces the path of a high-risk transaction (`TXN-004`) through the SentinelAegis platform.

## Narrative
When `TXN-004` is submitted, the Orchestrator fans out the request to three distinct AI agents running concurrently. Each agent evaluates a different vector of the transaction.
- The **Email Tone Agent** analyzes the text but falls back to rule-based low risk due to API timeout.
- The **IBAN Change Agent** detects a modification 6 hours prior and flags HIGH risk.
- The **Timing Anomaly Agent** notes the request is 1h47m outside standard hours and flags HIGH risk.
The Orchestrator aggregates these responses and the Consensus Engine determines a final `HALT` decision because 2 out of 3 agents flagged HIGH risk.

## Sequence Diagram

```mermaid
sequenceDiagram
    participant User
    participant Frontend
    participant Orchestrator
    participant Agent_EmailTone
    participant Agent_IBAN
    participant Agent_Timing
    participant ConsensusEngine
    participant BigQuery

    User->>Frontend: Click "Run Scenario TXN-004"
    Frontend->>Orchestrator: POST /api/analyze {tx: TXN-004}
    
    par Fan-out Analysis
        Orchestrator->>Agent_EmailTone: Analyze Tone
        Orchestrator->>Agent_IBAN: Analyze IBAN History
        Orchestrator->>Agent_Timing: Analyze Timestamp
    end
    
    Agent_EmailTone-->>Orchestrator: {risk: LOW, confidence: 0.1, fallback: true}
    Agent_IBAN-->>Orchestrator: {risk: HIGH, confidence: 0.91}
    Agent_Timing-->>Orchestrator: {risk: HIGH, confidence: 0.90}
    
    Orchestrator->>ConsensusEngine: Evaluate [LOW, HIGH, HIGH]
    ConsensusEngine-->>Orchestrator: {decision: HALT, score: 86}
    
    Orchestrator->>BigQuery: LogConsensus(Asynchronous)
    Orchestrator-->>Frontend: HTTP 200 {decision: HALT, breakdown: [...]}
    Frontend-->>User: Display Red HALT UI
```
