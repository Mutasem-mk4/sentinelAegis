# IBAN Change Agent Skill

The IBAN Change Agent detects when a vendor's payment details are modified shortly before an invoice is submitted.

## L1 (Basic Rules)
- **Trigger**: Regex extraction of IBAN patterns.
- **Logic**: Compare extracted IBAN with the known vendor IBAN in the database. If different, flag as HIGH risk.

## L2 (Gemini Contextual Analysis)
- **Trigger**: IBAN change detected within 48 hours of an invoice.
- **Prompt Template**:
  ```text
  A vendor requested payment to a new IBAN. 
  Vendor: {{vendor_name}}
  Current IBAN: {{current_iban}}
  Hours since IBAN change: {{hours_ago}}
  Assess the risk of this being an IBAN-swap BEC attack.
  Respond ONLY with JSON matching the schema.
  ```

## L3 (Network Graph Analysis)
- **Trigger**: The new IBAN belongs to a different country code than the vendor's registered address.
- **Logic**: Cross-reference IBAN country code (e.g., `GB`) against vendor HQ (e.g., `DE`). If mismatch, escalate to HALT.

## Expected Output Schema
```json
{
  "risk_level": "HIGH|MEDIUM|LOW",
  "confidence": 0.0 - 1.0,
  "flags": ["IBAN changed recently", "Country mismatch"],
  "explanation": "Brief description of findings."
}
```
