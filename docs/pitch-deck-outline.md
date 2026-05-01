# SentinelAegis — Pitch Deck Outline

## Slide Structure (12 Slides, 5-Minute Presentation)

---

### Slide 1: Title
**SentinelAegis: Autonomous Multi-Agent Consensus for Cross-Domain Financial Integrity**
- Team: Mutasem Kharma (Lead Dev), Moayad Darwish (Research), Ibrahim Sabbah (Security), Fathe Al-allak (Frontend)
- Al-Zaytoonah University · FinTech & Secure Banking Track · Build with AI 2026
- Visual: Logo + shield animation

---

### Slide 2: The Problem
**"Fraud has evolved. Your defenses haven't."**
- **$2.7 billion** lost to BEC fraud in 2025 (FBI IC3)
- **70%** of successful wire fraud is multi-stage, cross-system
- Banks rely on single-domain detection: email OR transactions OR behavior — never together
- Visual: Diagram showing 3 blind silos (email, ERP, payments) with fraud moving between them

---

### Slide 3: The Opportunity — PSD3 Is Coming
**The EU is about to mandate what we've already built**
- PSD3/PSR adoption expected 2026
- **Verification of Payee (VoP)** becomes mandatory
- Banks face **liability shift** if they don't verify beneficiary details
- €10B+ market for compliance solutions
- Visual: PSD3 timeline graphic

---

### Slide 4: Current Solutions & Their Gaps
| Solution Type | What It Sees | What It Misses |
|---|---|---|
| Email security | Phishing language | IBAN manipulation, timing |
| Transaction monitoring | Amount anomalies | Social engineering context |
| Behavioral analytics | User patterns | Cross-vendor coordination |
| **SentinelAegis** | **All three domains simultaneously** | — |

---

### Slide 5: Our Solution — SentinelAegis
**Three AI agents. One consensus. Zero fraud.**
- 3 independent Gemini 1.5 Pro agents analyze every wire transfer:
  - 📧 **Email Tone Agent** — detects social engineering
  - 🏦 **IBAN Change Agent** — detects beneficiary manipulation
  - ⏰ **Timing Agent** — detects behavioral anomalies
- **2-of-3 consensus** → HALT, REVIEW, or APPROVE
- Visual: Architecture diagram with agent icons

---

### Slide 6: Technical Architecture
- Go 1.22 + standard library only → 15MB Docker image
- Google Cloud Run with auto-scaling
- Fan-out concurrency: 3 parallel goroutines
- SSE real-time dashboard
- Gmail Pub/Sub integration for autonomous email monitoring
- Visual: Mermaid architecture diagram

---

### Slide 7: The Three Agents (Deep Dive)
**Email Tone Agent** — "The Linguist"
- Analyzes urgency pressure, authority exploitation, isolation tactics
- Uses Gemini 1.5 Pro structured output

**IBAN Change Agent** — "The Sentinel"
- Detects recent IBAN changes (critical 48-hour window)
- Cross-border IBAN swap detection

**Timing Agent** — "The Analyst"
- Compares request time vs. vendor's typical payment window
- Off-hours requests → BEC indicator

---

### Slide 8: Consensus Protocol
**The Innovation: Multi-Agent Voting**
```
Score = (HIGH_count × 40) + (MEDIUM_count × 20) + avg(confidence) × 10
```
- 2+ agents HIGH → **HALT** (auto-block, score ≥85)
- 1 agent HIGH → **REVIEW** (human escalation, score 45-75)
- 0 agents HIGH → **APPROVE** (score ≤30)
- Rule-based fallbacks if Gemini unavailable → system NEVER crashes
- Visual: Scoring diagram with color-coded thresholds

---

### Slide 9: Live Demo
**Watch SentinelAegis catch a $847,000 BEC attack in real-time**
1. CEO impersonation email arrives
2. Email Tone Agent: **HIGH** (urgency + isolation + authority)
3. IBAN Change Agent: **HIGH** (changed 6 hours ago)
4. Timing Agent: **HIGH** (18:47 — after business hours)
5. Consensus: **🚨 HALT** (risk score: 92/100)
6. Transaction blocked. $847K saved.
- Visual: Dashboard screenshot with red HALT banner

---

### Slide 10: Scalability & Production Roadmap
| Phase | Timeline | Features |
|---|---|---|
| **MVP** (Now) | April 2026 | 3 agents, Gemini, Cloud Run, demo scenarios |
| **Phase 2** | Q3 2026 | Core banking API integration, additional agents (sanctions, geo) |
| **Phase 3** | Q1 2027 | PSD3 compliance certification, multi-tenant SaaS |
| **Phase 4** | Q3 2027 | Enterprise deployment, SOC integration, custom agent framework |

**Cost model:** $0.02 per analysis → $20K/month for 1M transactions → cheaper than one fraud loss

---

### Slide 11: Built With
- 🧠 **Google Gemini 1.5 Pro** — AI analysis backbone (2M token context)
- 🛠️ **Google Agent Development Kit (ADK)** — multi-agent orchestration framework
- ☁️ **Google Cloud Run** — serverless deployment
- 📧 **Gmail API + Pub/Sub** — real-time email ingestion
- 🔐 **Secret Manager** — credentials management
- 💻 **Antigravity IDE** — development, testing, deployment
- 🏗️ **Go 1.22** — zero-framework backend

---

### Slide 12: Thank You / Q&A
**"We're not just detecting fraud. We're architecting trust for the Open Banking era."**
- Live Demo: [sentinelaegis.run.app]
- GitHub: [github.com/Mutasem-mk4/sentinelAegis]
- Contact: [team email]

---

## Anticipated Judge Questions

1. **"Why 3 agents instead of 1?"** → False positive reduction; mirrors real SOC teams; each domain is a distinct ML problem
2. **"Is the Gemini API call too slow for real-time?"** → Fan-out pattern: latency = max(agent), not sum. Avg <4s.
3. **"What happens if Gemini is down?"** → Rule-based fallbacks activate silently. System never crashes.
4. **"How do you handle the cost at scale?"** → $0.02/analysis. At 1M txns/month = $20K. Average BEC loss = $120K.
5. **"Is this a real product or a prototype?"** → Prototype with production-grade architecture. Adding core banking APIs makes it production-ready.
6. **"How is this different from existing fraud detection?"** → Cross-domain correlation. No existing tool combines email, banking, and behavioral analysis in a single consensus.
