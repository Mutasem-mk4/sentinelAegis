package agents

import (
	"sentinelaegis/data"
)

const emailSystemPrompt = `You are a cybersecurity analyst specializing in Business Email Compromise (BEC) detection.
Analyze the email for social-engineering indicators:
1. URGENCY PRESSURE: "immediately", "ASAP", "urgent", "today", "right now", "cannot wait"
2. AUTHORITY EXPLOITATION: impersonating CEO/CFO/executive, using titles to pressure
3. ISOLATION TACTICS: "do not discuss", "keep confidential", "just between us"
4. FINANCIAL MANIPULATION: IBAN/wire changes, new bank details, bypassing approval
5. ABNORMAL PATTERNS: unusual sender domain, grammar issues, tone mismatch

Return ONLY a valid JSON object. No markdown. No preamble. No explanation outside the JSON.
{
  "risk_level": "HIGH" or "MEDIUM" or "LOW",
  "confidence": <float 0.0 to 1.0>,
  "flags": ["specific quotes or patterns found"],
  "explanation": "2-3 sentence analyst summary"
}`

const emailFallbackPrompt = `Classify this email as HIGH, MEDIUM, or LOW risk for BEC fraud. Return ONLY valid JSON: {"risk_level":"HIGH","confidence":0.5,"flags":["reason"],"explanation":"one sentence"}`

// AnalyzeEmailTone calls Gemini 1.5 Pro to analyze email language for BEC indicators.
// Retries once with a simpler prompt on failure. Returns safe LOW on total failure.
func AnalyzeEmailTone(tx data.Transaction) AgentResult {
	userPrompt := "From: " + tx.EmailSender + "\nSubject: " + tx.EmailSubject + "\n\n" + tx.EmailText

	// Attempt 1: full analysis
	result, err := CallGemini(userPrompt, emailSystemPrompt)
	if err == nil {
		return AgentResult{
			AgentName:   "email_tone",
			RiskLevel:   result.RiskLevel,
			Confidence:  result.Confidence,
			Flags:       result.Flags,
			Explanation: result.Explanation,
		}
	}

	// Attempt 2: simplified prompt
	shortText := tx.EmailText
	if len(shortText) > 500 {
		shortText = shortText[:500]
	}
	result, err = CallGemini("Subject: "+tx.EmailSubject+"\n\n"+shortText, emailFallbackPrompt)
	if err == nil {
		return AgentResult{
			AgentName:   "email_tone",
			RiskLevel:   result.RiskLevel,
			Confidence:  result.Confidence,
			Flags:       result.Flags,
			Explanation: result.Explanation,
		}
	}

	// Total failure — safe fallback
	return AgentResult{
		AgentName:   "email_tone",
		RiskLevel:   "LOW",
		Confidence:  0.1,
		Flags:       []string{"AI analysis unavailable — manual review required"},
		Explanation: "Gemini API call failed after two attempts. Defaulting to LOW risk. A human analyst should review this email manually.",
	}
}
