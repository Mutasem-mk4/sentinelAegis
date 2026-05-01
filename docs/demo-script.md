# SentinelAegis — Demo Video Script (≤5 Minutes)

## Pre-Recording Checklist
- [ ] Cloud Run service is live and responsive
- [ ] Gemini API key is configured and working
- [ ] Browser with dashboard tab ready
- [ ] Screen recording software configured (1080p, 30fps minimum)
- [ ] Script printed for reference

---

## Opening (00:00 – 00:30)

**[Screen: Title slide with logo]**

"Fraud has evolved. It's no longer about stolen passwords or forged checks.

Modern wire fraud is multi-stage. An attacker compromises a vendor's email, manipulates banking details, and times the payment request to avoid oversight. Each signal looks legitimate in isolation. Together, they cost banks $2.7 billion per year.

We built SentinelAegis to see what no single system can."

---

## The Problem (00:30 – 01:15)

**[Screen: Architecture diagram showing 3 silos]**

"Here's the problem: banks rely on single-domain detection.

Email security sees phishing language — but can't see the IBAN was just changed.
Transaction monitoring sees unusual amounts — but can't read the email that triggered it.
Behavioral analytics sees after-hours activity — but doesn't know the email said 'do not discuss this with anyone.'

Each system is blind to the others. Attackers exploit this gap."

**[Screen: PSD3 timeline]**

"The EU knows this too. PSD3, expected to be adopted this year, will mandate Verification of Payee and real-time fraud monitoring. Banks that don't adapt face regulatory penalties AND increased fraud liability."

---

## The Solution (01:15 – 02:00)

**[Screen: Navigate to live dashboard URL]**

"SentinelAegis uses three independent AI agents, each powered by Google Gemini 1.5 Pro.

Agent one — the Email Tone Analyst — reads the email body for social engineering: urgency, authority exploitation, and isolation tactics.

Agent two — the IBAN Sentinel — checks when the beneficiary's banking details were last changed. A change within 48 hours is a critical BEC indicator.

Agent three — the Timing Analyst — compares when the payment was requested against the vendor's typical payment window. An $847,000 wire at 11 PM? That's suspicious.

All three agents run simultaneously — in parallel, using Go's goroutines. The total analysis time is the slowest agent, not the sum."

---

## Live Demo (02:00 – 03:30)

**[Screen: Dashboard — click "Trigger Demo BEC Scenario"]**

"Let me show you what this looks like in action.

I'm going to trigger a simulated BEC attack. The CEO has apparently emailed the finance team, asking for an urgent $847,000 wire transfer to updated banking details. The email says 'do not discuss this with anyone.'

Watch the three agents analyze simultaneously..."

**[Wait for results to appear sequentially on screen]**

"Email Tone Agent: HIGH risk — it detected urgency pressure, authority exploitation, and isolation tactics.

IBAN Change Agent: HIGH risk — the beneficiary's banking details were changed only 2 hours ago. That's within the critical 48-hour window.

Timing Agent: the request came outside normal business hours.

Now the consensus engine votes. Two out of three agents flagged HIGH risk — that crosses the threshold. The verdict: HALT. Transaction blocked. Risk score: 92 out of 100."

**[Point to the red HALT banner]**

"$847,000 saved. In under 4 seconds."

---

## Technical Depth (03:30 – 04:15)

**[Screen: Show the API response JSON or project structure]**

"Let me show you what's under the hood.

The entire backend is written in Go 1.22 using the standard library only — zero external frameworks. The Docker image is 15 megabytes.

Each agent calls the Gemini 1.5 Pro API with specialized system prompts. If Gemini is unavailable, deterministic rule-based fallbacks activate silently. The system never crashes.

The consensus engine uses a weighted scoring formula. HIGH counts for 40 points, MEDIUM for 20, plus a confidence-weighted component. Two or more HIGH flags trigger automatic HALT.

We have 16 passing tests, including table-driven consensus tests, agent-level tests, and performance benchmarks. The consensus engine runs in 871 nanoseconds."

---

## Closing (04:15 – 05:00)

**[Screen: Architecture diagram + PSD3 compliance table]**

"SentinelAegis isn't just a prototype. The architecture is designed for production scale.

Cloud Run auto-scales horizontally. Each analysis costs about 2 cents. At a million transactions per month, that's $20,000 — less than the cost of a single successful BEC attack.

And because we built this with PSD3 compliance in mind, banks adopting SentinelAegis won't just detect fraud better — they'll meet the regulatory requirements before enforcement begins.

We're not just detecting fraud. We're architecting trust for the Open Banking era.

This is SentinelAegis. Built in 10 days on Antigravity IDE, powered by Google Gemini, deployed on Cloud Run."

**[Screen: Final title slide with team names and URLs]**

---

## Key Timestamps
| Time | Content | Visual |
|---|---|---|
| 0:00 | Hook — fraud has evolved | Title slide |
| 0:30 | Problem — silo blindness | Architecture gap diagram |
| 1:15 | Solution — 3 agents, 1 consensus | Dashboard overview |
| 2:00 | Live demo — BEC attack | Dashboard in action |
| 3:30 | Technical depth — code + tests | Project structure / JSON |
| 4:15 | Scalability + PSD3 | Compliance table |
| 4:45 | Call to action | Final slide |
