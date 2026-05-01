# Timing Anomaly Agent Skill

The Timing Anomaly Agent detects suspicious invoice submission times outside the vendor's standard operating hours.

## L1 (Basic Rules)
- **Trigger**: Invoice received.
- **Logic**: Extract timestamp. If hour is < 08:00 or > 18:00 vendor local time, flag MEDIUM risk.

## L2 (Gemini Statistical Analysis)
- **Trigger**: Timestamp deviates significantly from historical average.
- **Prompt Template**:
  ```text
  Analyze this transaction timing.
  Vendor: {{vendor_name}}
  Time of request: {{request_time}}
  Historical average window: {{historical_window}}
  Is this deviation typical or suspicious?
  Respond ONLY with JSON matching the schema.
  ```

## L3 (Global Timezone Correlation)
- **Trigger**: IP address timezone differs from sender timezone.
- **Logic**: If the email originates from an IP timezone opposite to the vendor's stated working hours, escalate confidence to 0.95.

## Expected Output Schema
```json
{
  "risk_level": "HIGH|MEDIUM|LOW",
  "confidence": 0.0 - 1.0,
  "flags": ["Outside business hours", "Timezone anomaly"],
  "explanation": "Brief description of findings."
}
```
