# PSD3/PSR Compliance Mapping — SentinelAegis

## Regulatory Context

The **Payment Services Directive 3 (PSD3)** and **Payment Services Regulation (PSR)** are expected to be adopted by the EU in 2026, with enforcement beginning 2027–2028. Key provisions directly relevant to SentinelAegis:

## Compliance Feature Mapping

| PSD3/PSR Requirement | Article/Section | SentinelAegis Implementation | Status |
|---|---|---|---|
| **Verification of Payee (VoP)** | PSR Art. 58 | IBAN Change Agent validates beneficiary details against historical records. Detects IBAN-swap attacks. | ✅ Implemented |
| **Strong Customer Authentication (SCA)** | PSD3 Art. 97 | Consensus engine triggers step-up verification on HIGH risk transactions. Multi-factor agent agreement mimics SCA's multi-factor approach. | ✅ Implemented |
| **Fraud Detection Obligation** | PSR Art. 83 | Three-agent real-time analysis of every wire transfer request. No human bottleneck for screening. | ✅ Implemented |
| **Transaction Monitoring** | PSD3 Art. 94 | Real-time SSE dashboard with audit trail. Every agent decision logged with full context. | ✅ Implemented |
| **Liability Shift for Non-VoP** | PSR Art. 59 | Automated VoP check via IBAN Change Agent. Documentation in audit trail proves compliance. | ✅ Implemented |
| **Cross-Domain Data Sharing** | PSD3 Recital 45 | Multi-domain analysis (email + ERP + payment) in single consensus decision. Breaks down silo-based fraud detection. | ✅ Implemented |
| **Consumer Protection** | PSD3 Art. 100 | HALT decision prevents fraudulent transfers before execution. REVIEW escalates ambiguous cases. | ✅ Implemented |
| **Audit Trail Requirements** | PSR Art. 85 | Structured JSON logging with correlation IDs. Full agent reasoning chain preserved. | ✅ Implemented |
| **Incident Response Time** | PSD3 Art. 96 | Sub-5-second analysis latency. Real-time alerting via SSE. | ✅ Implemented |

## How SentinelAegis Enables PSD3 Compliance

### 1. Verification of Payee (VoP) — PSR Article 58
PSD3 mandates that Payment Service Providers (PSPs) verify that the payee's name matches the IBAN before executing a transfer. SentinelAegis's **IBAN Change Agent** provides the detection layer:
- Monitors for recent IBAN changes (critical 48-hour window)
- Cross-references current vs. historical banking details
- Flags cross-border IBAN changes as higher risk
- Generates evidence chain for regulatory reporting

### 2. Strong Customer Authentication (SCA) — PSD3 Article 97
SentinelAegis's consensus mechanism acts as an AI-driven supplement to SCA:
- 2-of-3 agent agreement mirrors multi-factor authentication principles
- HIGH consensus triggers mandatory human verification (step-up auth)
- Low-confidence results escalate to REVIEW, preventing over-automation

### 3. Real-Time Fraud Monitoring — PSR Article 83
PSR requires PSPs to implement **real-time transaction monitoring mechanisms**. SentinelAegis provides:
- Sub-second email ingestion via Gmail Pub/Sub
- Concurrent 3-agent analysis in <5 seconds
- SSE-powered real-time dashboard for SOC operators
- Automated HALT/REVIEW/APPROVE decisions with full audit trail

## Regulatory Timeline

```
2024 ─── EU Commission proposal published
2025 ─── Legislative negotiations (trilogue)
2026 ─── Expected adoption ← WE ARE HERE
2027 ─── Implementation period begins
2028 ─── Enforcement begins (estimated)
```

**SentinelAegis is positioned to be compliance-ready before enforcement begins**, giving early adopters a competitive advantage in the EU banking market.

## Key Differentiator

Traditional fraud detection systems are **single-domain** (analyze only transactions OR only emails OR only timing). PSD3's VoP and SCA requirements demand **cross-domain correlation** — exactly what SentinelAegis's multi-agent consensus provides.

> *"By the time PSD3 enforcement begins in 2028, banks that haven't implemented cross-domain fraud detection will face both regulatory penalties and increased fraud liability."*
