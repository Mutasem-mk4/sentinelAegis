package gmail

import (
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"sentinelaegis/agents"
	"sentinelaegis/data"
	"sentinelaegis/types"
)

var (
	amountRegex = regexp.MustCompile(`\$[\d,]+(?:\.\d{2})?|\d[\d,]*(?:\.\d{2})?\s*(?:USD|EUR|GBP)`)
	ibanRegex   = regexp.MustCompile(`[A-Z]{2}\d{2}[A-Z0-9]{4,30}`)
	numberRegex = regexp.MustCompile(`[\d,.]+`)
)

// AnalyzeEmail builds a Transaction from an email and runs 3-agent consensus.
func AnalyzeEmail(email types.EmailData) types.AnalysisEvent {
	start := time.Now()

	amount := extractAmount(email.Body)
	iban := extractIBAN(email.Body)
	vendor := extractVendorName(email.From)
	now := time.Now().Format("15:04")

	ibanHours := 0
	prevIBAN := ""
	if iban != "" {
		ibanHours = 6 // Assume recent change if IBAN found in body
		prevIBAN = "UNKNOWN_PREVIOUS"
	}

	customID := fmt.Sprintf("GMAIL-%d", time.Now().UnixMilli())
	tx := data.Transaction{
		ID:           customID,
		Vendor:       vendor,
		Amount:       amount,
		Currency:     "USD",
		IBAN:         iban,
		EmailSubject: email.Subject,
		EmailSender:  email.From,
		EmailText:    email.Body,
		RequestedAt:  now,
		Status:       "pending",
	}

	// Inject custom IBAN/timing data for agents
	agents.SetCustomIBAN(customID, prevIBAN, ibanHours)
	agents.SetCustomWindow(customID, "09:00", "17:00")
	defer agents.CleanupCustom(customID)

	// Run all 3 agents concurrently
	var (
		emailResult  agents.AgentResult
		ibanResult   agents.AgentResult
		timingResult agents.AgentResult
		wg           sync.WaitGroup
	)

	wg.Add(3)
	go func() { defer wg.Done(); emailResult = agents.AnalyzeEmailTone(tx) }()
	go func() { defer wg.Done(); ibanResult = agents.CheckIBANChange(tx) }()
	go func() { defer wg.Done(); timingResult = agents.CheckTimingAnomaly(tx) }()
	wg.Wait()

	allResults := []agents.AgentResult{emailResult, ibanResult, timingResult}
	consensus := agents.RunConsensus(allResults)

	latency := time.Since(start).Milliseconds()

	log.Printf("[GMAIL] %s → %s (score: %d, latency: %dms)", email.Subject, consensus.Decision, consensus.RiskScore, latency)

	return types.AnalysisEvent{
		EventType:     "consensus",
		Timestamp:     time.Now().Format(time.RFC3339),
		Email:         email,
		TransactionID: customID,
		Vendor:        vendor,
		Amount:        amount,
		Consensus:     &consensus,
		RiskLevel:     consensus.Decision,
		LatencyMs:     latency,
	}
}

func extractAmount(body string) float64 {
	matches := amountRegex.FindAllString(body, -1)
	var maxAmount float64
	for _, m := range matches {
		nums := numberRegex.FindString(m)
		nums = strings.ReplaceAll(nums, ",", "")
		if val, err := strconv.ParseFloat(nums, 64); err == nil && val > maxAmount {
			maxAmount = val
		}
	}
	if maxAmount == 0 {
		return 50000 // Default amount for demo
	}
	return maxAmount
}

func extractIBAN(body string) string {
	match := ibanRegex.FindString(body)
	if match != "" && len(match) >= 15 {
		return match
	}
	return ""
}

func extractVendorName(from string) string {
	// "John Smith <john@company.com>" → "John Smith"
	if idx := strings.Index(from, "<"); idx > 0 {
		return strings.TrimSpace(from[:idx])
	}
	// "john@company.com" → "company.com"
	if idx := strings.Index(from, "@"); idx >= 0 {
		domain := from[idx+1:]
		domain = strings.TrimSuffix(domain, ">")
		return domain
	}
	return from
}
