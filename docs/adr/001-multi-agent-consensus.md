# ADR-001: Multi-Agent Consensus Architecture

**Date:** 2026-04-24  
**Status:** Accepted  
**Authors:** Mutasem Kharma

## Context

Business Email Compromise (BEC) fraud is multi-dimensional — it exploits email communication, banking detail changes, and timing patterns simultaneously. A single detection model cannot reliably distinguish sophisticated BEC from legitimate activity because each signal in isolation has high false-positive rates.

## Decision

We implement a **3-agent consensus architecture** where three independent Gemini 1.5 Pro agents analyze the same transaction from different perspectives, and a deterministic 2-of-3 voting engine produces a final decision.

### Agent Roles
| Agent | Signal Domain | Primary Detection Target |
|---|---|---|
| Email Tone Agent | Natural language | Social engineering, urgency, authority exploitation |
| IBAN Change Agent | Banking data | Beneficiary manipulation, IBAN-swap attacks |
| Timing Agent | Behavioral patterns | Off-hours requests, deadline pressure |

### Consensus Rules
- **2+ agents flag HIGH** → `HALT` (auto-block, score ≥85)
- **1 agent flags HIGH** → `REVIEW` (escalate to human, score 45–75)
- **0 agents flag HIGH** → `APPROVE` (score ≤30)

### Score Formula
```
risk_score = (HIGH_count × 40) + (MEDIUM_count × 20) + avg(confidence) × 10
```

## Consequences

**Positive:**
- Dramatically reduces false positives (requires 2+ agents to agree)
- Each agent can be independently improved without affecting others
- Adding new agents (sanctions, geolocation) requires zero changes to existing code
- Mirrors real-world SOC team decision-making processes

**Negative:**
- Requires 3 concurrent Gemini API calls per analysis ($0.02/analysis)
- Total latency equals max(agent_latency), not sum, due to fan-out pattern
- Single-domain attacks may be missed if only 1 agent detects it (mitigated by REVIEW)

**Alternatives Considered:**
1. **Single unified model:** Rejected — context window management becomes complex, single point of failure
2. **Sequential pipeline:** Rejected — increases latency (serial calls), creates agent ordering dependency
3. **Majority voting without confidence:** Rejected — loses nuance; HIGH/MEDIUM distinction is critical
