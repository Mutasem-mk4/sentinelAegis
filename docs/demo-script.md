# Demo Script: SentinelAegis (4.5 Minutes)

**Presenter**: Mutasem Kharma  
**Goal**: Demonstrate real-time BEC threat detection, multi-agent AI consensus, and immediate actionability for the Antigravity IDE Project Challenge.

---

### [0:00 - 0:45] The Problem (Slide 1-2)
**Action**: Keep dashboard hidden, show slide deck.
**Script**:
"Hello everyone. In 2024, Business Email Compromise (BEC) cost organizations over $2.9 billion globally. Attackers don't hack systems; they hack human trust. They learn communication patterns, wait for the perfect moment, and intercept payments by swapping bank details. Traditional rule-based systems are blind to this context. Today, I'm introducing SentinelAegis—an autonomous SOC platform that uses Google Gemini's multi-agent AI to read the context between the lines and stop BEC before the money leaves the bank."

### [0:45 - 1:15] The Architecture (Slide 3)
**Action**: Switch to Architecture diagram slide.
**Script**:
"We built this completely natively in Go, deployed on Cloud Run for zero-latency scaling. When an email or payment request arrives, it doesn't go to one AI model—it fans out to three concurrent specialized agents: an Email Tone agent, an IBAN Change agent, and a Timing Anomaly agent. They evaluate the risk independently, and a Consensus Engine makes a final decision. Let me show you this live."

### [1:15 - 2:30] Live Demo: Automated Scenarios
**Action**: Switch to browser showing the SentinelAegis Dashboard. Click "Run All 5 Demo Scenarios".
**Script**:
"This is our SOC dashboard. I'm going to run a simulation of 5 live transactions hitting our payment gateway right now.
*(Wait for TXN-001 and TXN-002)*
"Notice how routine transactions are instantly APPROVED or marked for REVIEW.
*(TXN-004 halts, screen flashes red)*
"But look at this one—TXN-004 just triggered a HALT. Let's expand the trace."
**Action**: Click TXN-004 to expand.
**Script**:
"Our system explains exactly *why*. The IBAN was changed just 6 hours ago, and the request came in at 6:47 PM, completely outside this vendor's normal business hours. The Consensus Engine aggregated these HIGH risk signals from the specialized agents and blocked an $847,000 fraudulent transfer, logging the entire audit trail securely."

### [2:30 - 3:30] Live Demo: Custom Scenario
**Action**: Click "Trigger Custom BEC Scenario".
**Script**:
"Let's trigger a custom attack live. I'll put in a spoofed email body: 'URGENT: Please process this invoice to our new banking details immediately to avoid contract termination.' I'll set the amount to $500,000."
**Action**: Submit form, wait for HALT. Expand row.
**Script**:
"Within milliseconds, the AI intercepts the social engineering tactics—urgency and secrecy—and cross-references the recent IBAN change. Another half-million dollars saved, with zero human intervention required."

### [3:30 - 4:15] Security & Compliance (Slide 4)
**Action**: Switch back to slides (PSD3 compliance).
**Script**:
"In FinTech, explainability is not optional; it's the law. SentinelAegis maps directly to upcoming PSD3 compliance requirements. Every single AI decision is fully traceable, securely logged, and generates human-readable explanations. There are no 'black box' rejections here."

### [4:15 - 4:30] Closing
**Action**: Final slide.
**Script**:
"SentinelAegis transforms passive security monitoring into active, intelligent defense. Thank you to Google and GDGoC for this challenge. I'm ready for your questions."

---
*Pro-tip for demo: Ensure Cloud Run container is warm before starting to avoid cold-start latency on TXN-001.*
