package services

import (
	"context"
	"fmt"
	"log"
	"time"

	"cloud.google.com/go/bigquery"
)

// ConsensusLogEntry represents a single row in the audit trail.
type ConsensusLogEntry struct {
	Timestamp          time.Time `bigquery:"timestamp"`
	CorrelationID      string    `bigquery:"correlation_id"`
	TransactionID      string    `bigquery:"transaction_id"`
	Vendor             string    `bigquery:"vendor"`
	Amount             int64     `bigquery:"amount"`
	EmailToneRisk      string    `bigquery:"email_tone_risk"`
	IBANChangeRisk     string    `bigquery:"iban_change_risk"`
	TimingRisk         string    `bigquery:"timing_risk"`
	ConsensusDecision  string    `bigquery:"consensus_decision"`
	RiskScore          int       `bigquery:"risk_score"`
	LatencyMs          int64     `bigquery:"latency_ms"`
	AgentExplanation   string    `bigquery:"agent_explanation"`
	ErrorMessage       string    `bigquery:"error_message"`
}

type BigQueryAuditLogger struct {
	client  *bigquery.Client
	inserter *bigquery.Inserter
}

// NewBigQueryAuditLogger creates a new audit logger, verifying the table exists.
func NewBigQueryAuditLogger(ctx context.Context, projectID, datasetID, tableID string) (*BigQueryAuditLogger, error) {
	client, err := bigquery.NewClient(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("bigquery.NewClient: %v", err)
	}

	dataset := client.Dataset(datasetID)
	table := dataset.Table(tableID)

	// Ensure table exists, create if not
	md, err := table.Metadata(ctx)
	if err != nil {
		schema, err := bigquery.InferSchema(ConsensusLogEntry{})
		if err != nil {
			return nil, fmt.Errorf("InferSchema: %v", err)
		}
		if err := table.Create(ctx, &bigquery.TableMetadata{Schema: schema}); err != nil {
			return nil, fmt.Errorf("table.Create: %v", err)
		}
	} else {
		log.Printf("BigQuery table %s exists. Schema: %v", tableID, md.Schema)
	}

	return &BigQueryAuditLogger{
		client:  client,
		inserter: table.Inserter(),
	}, nil
}

// LogConsensus logs asynchronously to avoid blocking the API response.
func (b *BigQueryAuditLogger) LogConsensus(ctx context.Context, entry ConsensusLogEntry) {
	go func() {
		insertCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := b.inserter.Put(insertCtx, []*ConsensusLogEntry{&entry}); err != nil {
			log.Printf("ERROR writing to BigQuery: %v", err)
		}
	}()
}

// Close closes the underlying BigQuery client.
func (b *BigQueryAuditLogger) Close() {
	if b.client != nil {
		b.client.Close()
	}
}
