# ADR-002: Gemini 1.5 Pro with Rule-Based Fallbacks

**Date:** 2026-04-24  
**Status:** Accepted  
**Authors:** Mutasem Kharma

## Context

The system depends on Gemini 1.5 Pro for AI-powered analysis, but external API availability cannot be guaranteed. During demos, a failed Gemini call could crash the entire system, destroying the presentation.

## Decision

Every agent implements a **dual-mode architecture**:

1. **Primary path:** Full Gemini 1.5 Pro analysis with structured JSON response
2. **Fallback path:** Deterministic rule-based analysis using the same data

The fallback activates silently if:
- `GEMINI_API_KEY` is not set
- Gemini returns a non-200 status
- Gemini's response fails JSON parsing
- Network timeout (20s)

### Email Tone Agent Fallback
Returns LOW risk with a flag indicating manual review is required.

### IBAN Change Agent Fallback
Applies deterministic rules:
- Changed ≤48 hours ago → HIGH (0.91 confidence)
- Changed ≤168 hours ago → MEDIUM (0.72 confidence)
- Changed >168 hours ago → LOW (0.15 confidence)

### Timing Agent Fallback
Applies deterministic rules:
- Deviation >120 minutes → HIGH (0.78 confidence)
- Deviation >30 minutes → MEDIUM (0.55 confidence)
- Within window → LOW (0.12 confidence)

## Consequences

**Positive:**
- System **never crashes** during demo — graceful degradation
- Deterministic fallbacks produce consistent, explainable results
- Development and testing work without Gemini API key
- Zero external dependencies for core functionality

**Negative:**
- Rule-based fallbacks are less nuanced than AI analysis
- Email Tone Agent's fallback is essentially a no-op (always LOW)
- Two code paths to maintain per agent

**Alternatives Considered:**
1. **Fail-fast with error:** Rejected — demo instability unacceptable
2. **Cache previous results:** Rejected — stale AI analysis is worse than no analysis
3. **Local model fallback:** Rejected — resource constraints on Cloud Run free tier
