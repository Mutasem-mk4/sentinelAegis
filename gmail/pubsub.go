package gmail

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"sync"
	"sync/atomic"

	"sentinelaegis/types"
)

// PubSubMessage matches the Google Pub/Sub push message format.
type PubSubMessage struct {
	Message struct {
		Data        string `json:"data"`
		MessageID   string `json:"messageId"`
		PublishTime string `json:"publishTime"`
	} `json:"message"`
	Subscription string `json:"subscription"`
}

// GmailNotification is the decoded Data field from Gmail push.
type GmailNotification struct {
	EmailAddress string `json:"emailAddress"`
	HistoryID    uint64 `json:"historyId"`
}

// PubSubHandler processes Gmail push notifications.
type PubSubHandler struct {
	gmailClient   *GmailClient
	analyzer      func(types.EmailData) types.AnalysisEvent
	eventStream   chan types.AnalysisEvent
	lastHistoryID atomic.Uint64
	processedMu   sync.Mutex
	processed     map[string]bool
}

// NewPubSubHandler creates a handler for Gmail Pub/Sub push notifications.
func NewPubSubHandler(client *GmailClient, analyzer func(types.EmailData) types.AnalysisEvent) *PubSubHandler {
	h := &PubSubHandler{
		gmailClient: client,
		analyzer:    analyzer,
		eventStream: make(chan types.AnalysisEvent, 100),
		processed:   make(map[string]bool),
	}

	// Initialize with current history ID
	histID, err := client.GetHistoryID()
	if err != nil {
		log.Printf("⚠️ Could not get initial historyId: %v", err)
	} else {
		h.lastHistoryID.Store(histID)
		log.Printf("📧 Initial historyId: %d", histID)
	}

	return h
}

// HandlePush is the HTTP handler for the Pub/Sub push endpoint.
// POST /pubsub/push
func (h *PubSubHandler) HandlePush(w http.ResponseWriter, r *http.Request) {
	// Always return 200 to Pub/Sub to prevent retries
	defer func() { w.WriteHeader(http.StatusOK) }()

	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("[pubsub] Failed to read body: %v", err)
		return
	}

	var pushMsg PubSubMessage
	if err := json.Unmarshal(body, &pushMsg); err != nil {
		log.Printf("[pubsub] Failed to decode push message: %v", err)
		return
	}

	// Decode the base64-encoded notification data
	decoded, err := base64.StdEncoding.DecodeString(pushMsg.Message.Data)
	if err != nil {
		log.Printf("[pubsub] Failed to decode data: %v", err)
		return
	}

	var notification GmailNotification
	if err := json.Unmarshal(decoded, &notification); err != nil {
		log.Printf("[pubsub] Failed to decode notification: %v", err)
		return
	}

	log.Printf("[pubsub] Notification from %s, historyId: %d", notification.EmailAddress, notification.HistoryID)

	// Get new messages since our last known history ID
	lastID := h.lastHistoryID.Load()
	if lastID == 0 {
		lastID = notification.HistoryID - 1
	}

	messageIDs, err := h.gmailClient.ListNewMessages(lastID)
	if err != nil {
		log.Printf("[pubsub] Failed to list new messages: %v", err)
		return
	}

	// Update our history ID
	h.lastHistoryID.Store(notification.HistoryID)

	// Process each new message
	for _, msgID := range messageIDs {
		// Dedup: skip already processed messages
		h.processedMu.Lock()
		if h.processed[msgID] {
			h.processedMu.Unlock()
			continue
		}
		h.processed[msgID] = true
		h.processedMu.Unlock()

		go h.processMessage(msgID)
	}
}

func (h *PubSubHandler) processMessage(messageID string) {
	email, err := h.gmailClient.GetMessage(messageID)
	if err != nil {
		log.Printf("[pubsub] Failed to get message %s: %v", messageID, err)
		return
	}

	log.Printf("[pubsub] Processing email: %s — %s", email.From, email.Subject)

	// Broadcast "email_received" event first
	h.eventStream <- types.AnalysisEvent{
		EventType: "email_received",
		Timestamp: email.Date,
		Email:     *email,
	}

	// Run analysis
	result := h.analyzer(*email)
	result.Email = *email

	// Broadcast "consensus" event with full results
	h.eventStream <- result

	log.Printf("[pubsub] Analysis complete: %s → %s (risk: %s)",
		email.Subject, result.Consensus.Decision, result.RiskLevel)
}

// GetEventStream returns the channel of analysis events.
func (h *PubSubHandler) GetEventStream() chan types.AnalysisEvent {
	return h.eventStream
}
