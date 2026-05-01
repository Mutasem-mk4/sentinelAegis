package middleware

import (
	"fmt"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
)

var (
	AnalysisTotalHalt    uint64
	AnalysisTotalReview  uint64
	AnalysisTotalApprove uint64

	GeminiApiErrorsTotal uint64
	RuleFallbackTotal    uint64

	// Buckets: 50, 100, 250, 500, 1000, 2000, 5000 ms
	LatencyBuckets = []int64{50, 100, 250, 500, 1000, 2000, 5000}
	LatencyCounts  = make([]uint64, len(LatencyBuckets)+1)
	LatencySum     uint64
	LatencyCount   uint64
	mu             sync.Mutex
)

// RecordAnalysis updates the decision counters and latency histogram.
func RecordAnalysis(decision string, latencyMs int64) {
	d := strings.ToUpper(decision)
	if d == "HALT" {
		atomic.AddUint64(&AnalysisTotalHalt, 1)
	} else if d == "REVIEW" {
		atomic.AddUint64(&AnalysisTotalReview, 1)
	} else if d == "APPROVE" {
		atomic.AddUint64(&AnalysisTotalApprove, 1)
	}

	mu.Lock()
	defer mu.Unlock()
	LatencySum += uint64(latencyMs)
	LatencyCount++
	
	placed := false
	for i, bucket := range LatencyBuckets {
		if latencyMs <= bucket {
			LatencyCounts[i]++
			placed = true
			break
		}
	}
	if !placed {
		LatencyCounts[len(LatencyBuckets)]++ // +Inf bucket
	}
}

// RecordGeminiError increments the API error counter.
func RecordGeminiError() {
	atomic.AddUint64(&GeminiApiErrorsTotal, 1)
}

// RecordRuleFallback increments the rule-based fallback counter.
func RecordRuleFallback() {
	atomic.AddUint64(&RuleFallbackTotal, 1)
}

// MetricsHandler serves metrics in Prometheus text format.
func MetricsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4")
	
	fmt.Fprintf(w, "# HELP sentinelaegis_analysis_total Total analyses performed by decision\n")
	fmt.Fprintf(w, "# TYPE sentinelaegis_analysis_total counter\n")
	fmt.Fprintf(w, "sentinelaegis_analysis_total{decision=\"HALT\"} %d\n", atomic.LoadUint64(&AnalysisTotalHalt))
	fmt.Fprintf(w, "sentinelaegis_analysis_total{decision=\"REVIEW\"} %d\n", atomic.LoadUint64(&AnalysisTotalReview))
	fmt.Fprintf(w, "sentinelaegis_analysis_total{decision=\"APPROVE\"} %d\n", atomic.LoadUint64(&AnalysisTotalApprove))

	fmt.Fprintf(w, "# HELP sentinelaegis_gemini_api_errors_total Total Gemini API errors\n")
	fmt.Fprintf(w, "# TYPE sentinelaegis_gemini_api_errors_total counter\n")
	fmt.Fprintf(w, "sentinelaegis_gemini_api_errors_total %d\n", atomic.LoadUint64(&GeminiApiErrorsTotal))

	fmt.Fprintf(w, "# HELP sentinelaegis_rule_fallback_total Total times system fell back to rules\n")
	fmt.Fprintf(w, "# TYPE sentinelaegis_rule_fallback_total counter\n")
	fmt.Fprintf(w, "sentinelaegis_rule_fallback_total %d\n", atomic.LoadUint64(&RuleFallbackTotal))

	mu.Lock()
	defer mu.Unlock()
	
	fmt.Fprintf(w, "# HELP sentinelaegis_analysis_latency_ms Histogram of analysis latency in milliseconds\n")
	fmt.Fprintf(w, "# TYPE sentinelaegis_analysis_latency_ms histogram\n")
	var cumulative uint64
	for i, bucket := range LatencyBuckets {
		cumulative += LatencyCounts[i]
		fmt.Fprintf(w, "sentinelaegis_analysis_latency_ms_bucket{le=\"%d\"} %d\n", bucket, cumulative)
	}
	cumulative += LatencyCounts[len(LatencyBuckets)]
	fmt.Fprintf(w, "sentinelaegis_analysis_latency_ms_bucket{le=\"+Inf\"} %d\n", cumulative)
	fmt.Fprintf(w, "sentinelaegis_analysis_latency_ms_sum %d\n", LatencySum)
	fmt.Fprintf(w, "sentinelaegis_analysis_latency_ms_count %d\n", LatencyCount)
}
