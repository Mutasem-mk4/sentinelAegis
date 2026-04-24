package agents

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"sentinelaegis/data"
)

const systemPrompt = `You are a cybersecurity analyst specializing in Business Email Compromise (BEC) detection.
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

// geminiRequest/response structs for the REST API.
type geminiRequest struct {
	SystemInstruction *geminiContent     `json:"systemInstruction,omitempty"`
	Contents          []geminiContent    `json:"contents"`
	GenerationConfig  *geminiGenConfig   `json:"generationConfig,omitempty"`
}
type geminiContent struct {
	Parts []geminiPart `json:"parts"`
}
type geminiPart struct {
	Text string `json:"text"`
}
type geminiGenConfig struct {
	Temperature     float64 `json:"temperature"`
	MaxOutputTokens int     `json:"maxOutputTokens"`
}
type geminiResponse struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

type geminiResult struct {
	RiskLevel   string   `json:"risk_level"`
	Confidence  float64  `json:"confidence"`
	Flags       []string `json:"flags"`
	Explanation string   `json:"explanation"`
}

// AnalyzeEmailTone calls Gemini 1.5 Pro via REST API.
// Retries once with a simpler prompt on failure.
// Never panics — returns safe LOW on total failure.
func AnalyzeEmailTone(tx data.Transaction) AgentResult {
	result, err := callGemini(tx.EmailText, systemPrompt)
	if err == nil {
		return AgentResult{
			AgentName:   "email_tone",
			RiskLevel:   result.RiskLevel,
			Confidence:  result.Confidence,
			Flags:       result.Flags,
			Explanation: result.Explanation,
		}
	}

	// Retry with simpler prompt
	simpleSystem := `Classify this email as HIGH, MEDIUM, or LOW risk for BEC fraud. Return ONLY valid JSON: {"risk_level":"HIGH","confidence":0.5,"flags":["reason"],"explanation":"one sentence"}`
	shortText := tx.EmailText
	if len(shortText) > 500 {
		shortText = shortText[:500]
	}
	result, err = callGemini("Subject: "+tx.EmailSubject+"\n\n"+shortText, simpleSystem)
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
		Flags:       []string{"Analysis unavailable — manual review required"},
		Explanation: "Gemini API call failed after two attempts. Defaulting to LOW risk. A human analyst should review this email manually.",
	}
}

func callGemini(userText, system string) (*geminiResult, error) {
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("GEMINI_API_KEY not set")
	}

	model := os.Getenv("MODEL_NAME")
	if model == "" {
		model = "gemini-1.5-pro"
	}

	url := fmt.Sprintf(
		"https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s",
		model, apiKey,
	)

	reqBody := geminiRequest{
		SystemInstruction: &geminiContent{
			Parts: []geminiPart{{Text: system}},
		},
		Contents: []geminiContent{
			{Parts: []geminiPart{{Text: userText}}},
		},
		GenerationConfig: &geminiGenConfig{
			Temperature:     0.2,
			MaxOutputTokens: 500,
		},
	}

	bodyBytes, _ := json.Marshal(reqBody)

	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Post(url, "application/json", bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	respBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("Gemini API returned %d: %s", resp.StatusCode, string(respBytes[:min(len(respBytes), 200)]))
	}

	var gemResp geminiResponse
	if err := json.Unmarshal(respBytes, &gemResp); err != nil {
		return nil, fmt.Errorf("failed to parse Gemini response: %w", err)
	}
	if gemResp.Error != nil {
		return nil, fmt.Errorf("Gemini error: %s", gemResp.Error.Message)
	}
	if len(gemResp.Candidates) == 0 || len(gemResp.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("empty Gemini response")
	}

	raw := strings.TrimSpace(gemResp.Candidates[0].Content.Parts[0].Text)
	// Strip markdown fences
	if strings.HasPrefix(raw, "```") {
		if idx := strings.Index(raw, "\n"); idx != -1 {
			raw = raw[idx+1:]
		}
		if idx := strings.LastIndex(raw, "```"); idx != -1 {
			raw = raw[:idx]
		}
		raw = strings.TrimSpace(raw)
	}

	var result geminiResult
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		return nil, fmt.Errorf("failed to parse JSON from Gemini: %w", err)
	}

	// Normalize risk level
	result.RiskLevel = strings.ToUpper(strings.TrimSpace(result.RiskLevel))
	if result.RiskLevel != "HIGH" && result.RiskLevel != "MEDIUM" && result.RiskLevel != "LOW" {
		result.RiskLevel = "MEDIUM"
	}
	if result.Confidence < 0 {
		result.Confidence = 0
	}
	if result.Confidence > 1 {
		result.Confidence = 1
	}

	return &result, nil
}
