# Email Tone Agent Skill

The Email Tone Agent is responsible for analyzing the textual content of incoming emails to detect urgency, secrecy, authority impersonation, and other social engineering tactics common in Business Email Compromise (BEC).

## L1 (Basic Rules)
- **Trigger**: Regex matches for keywords: `urgent`, `immediately`, `secret`, `do not discuss`, `confidential`, `bypass`.
- **Logic**: If 2 or more keywords are found, flag as MEDIUM risk.

## L2 (Gemini Semantic Analysis)
- **Trigger**: Any email over 50 characters.
- **Prompt Template**:
  ```text
  Analyze the following email for BEC fraud indicators. Focus on tone, urgency, and demands for secrecy.
  Email Body: {{email_body}}
  Respond ONLY with JSON matching the expected schema.
  ```

## L3 (Long-Context Drift)
- **Trigger**: Analyzing the current email against the last 50 emails from the same sender.
- **Logic**: If the tone drastically changes from standard corporate communication to highly urgent/informal, raise risk score by +20.

## Expected Output Schema
```json
{
  "risk_level": "HIGH|MEDIUM|LOW",
  "confidence": 0.0 - 1.0,
  "flags": ["Urgency detected", "CEO impersonation likely"],
  "explanation": "Brief description of findings."
}
```
