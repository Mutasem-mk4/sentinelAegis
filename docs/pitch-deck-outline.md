# Pitch Deck Outline

## Slide 1: Title Slide
- **Title**: SentinelAegis
- **Subtitle**: Multi-Agent AI for Autonomous BEC Fraud Prevention
- **Visual**: Dark-themed lock/shield graphic + Google Cloud / Gemini logos.
- **Presenter**: Mutasem Kharma

## Slide 2: The $2.9 Billion Problem
- **Headline**: BEC is exploiting human trust, not systems.
- **Bullets**:
  - Attackers use context, urgency, and timing.
  - Legacy rule-based WAFs/filters fail against legitimate credentials.
  - Manual review is slow, expensive, and error-prone.

## Slide 3: The SentinelAegis Solution
- **Headline**: Context-Aware Multi-Agent Consensus
- **Diagram**: 
  - Ingress -> Orchestrator
  - Fan-out -> [Tone Agent, IBAN Agent, Timing Agent]
  - Aggregation -> Consensus Engine -> [APPROVE, REVIEW, HALT]
- **Key Tech**: Go 1.22, Google Cloud Run, Gemini 2.5 Flash.

## Slide 4: Compliance & Traceability
- **Headline**: Built for PSD3 and FinTech Auditing
- **Table/Bullets**:
  - *Requirement*: Explainable AI decisions -> *Solution*: Natural language breakdown per agent.
  - *Requirement*: Immutable Audit Trails -> *Solution*: Async BigQuery logging.
  - *Requirement*: Resiliency -> *Solution*: Rule-based fallback if API is rate-limited.

## Slide 5: The Market & Future
- **Headline**: From Hackathon to Enterprise SOC
- **Bullets**:
  - Expand integrations (Office365, Slack).
  - Train localized ML models for specific vendor behavior.
  - "Security as a Microservice" deployment model.
- **Call to Action**: Try the demo live!
