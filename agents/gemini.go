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
)

// ── Gemini REST API types ───────────────────────────────

type geminiRequest struct {
	SystemInstruction *geminiContent   `json:"systemInstruction,omitempty"`
	Contents          []geminiContent  `json:"contents"`
	GenerationConfig  *geminiGenConfig `json:"generationConfig,omitempty"`
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
	ResponseMimeType string  `json:"responseMimeType,omitempty"`
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

// GeminiResult is the standard JSON structure all agents request from Gemini.
type GeminiResult struct {
	RiskLevel   string   `json:"risk_level"`
	Confidence  float64  `json:"confidence"`
	Flags       []string `json:"flags"`
	Explanation string   `json:"explanation"`
}

// CallGemini sends a prompt to the Gemini REST API and returns parsed JSON.
// This is the shared infrastructure used by all 3 agents.
func CallGemini(userText, systemPrompt string) (*GeminiResult, error) {
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
			Parts: []geminiPart{{Text: systemPrompt}},
		},
		Contents: []geminiContent{
			{Parts: []geminiPart{{Text: userText}}},
		},
		GenerationConfig: &geminiGenConfig{
			Temperature:      0.2,
			MaxOutputTokens:  500,
			ResponseMimeType: "application/json",
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
		limit := len(respBytes)
		if limit > 200 {
			limit = 200
		}
		return nil, fmt.Errorf("Gemini API returned %d: %s", resp.StatusCode, string(respBytes[:limit]))
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

	// Strip markdown fences if present
	if strings.HasPrefix(raw, "```") {
		if idx := strings.Index(raw, "\n"); idx != -1 {
			raw = raw[idx+1:]
		}
		if idx := strings.LastIndex(raw, "```"); idx != -1 {
			raw = raw[:idx]
		}
		raw = strings.TrimSpace(raw)
	}

	var result GeminiResult
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		fmt.Printf("Gemini parsing error. Raw string was: %s\n", raw)
		return nil, fmt.Errorf("failed to parse JSON from Gemini: %w", err)
	}

	// Normalize
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
