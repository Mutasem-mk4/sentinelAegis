package gmail

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"mime"
	"strings"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	gapi "google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"

	"sentinelaegis/types"
)

// GmailClient wraps the Gmail API service.
type GmailClient struct {
	service *gapi.Service
	userID  string
}

// NewGmailClient creates a Gmail client from service account JSON credentials.
// The credentialsJSON should be the raw JSON key file content.
// delegateEmail is the Gmail address to impersonate (domain-wide delegation).
func NewGmailClient(credentialsJSON []byte, delegateEmail string) (*GmailClient, error) {
	ctx := context.Background()

	config, err := google.JWTConfigFromJSON(credentialsJSON, gapi.GmailReadonlyScope)
	if err != nil {
		return nil, fmt.Errorf("failed to parse credentials: %w", err)
	}
	config.Subject = delegateEmail

	srv, err := gapi.NewService(ctx, option.WithHTTPClient(config.Client(ctx)))
	if err != nil {
		return nil, fmt.Errorf("failed to create Gmail service: %w", err)
	}

	return &GmailClient{service: srv, userID: "me"}, nil
}

// NewGmailClientOAuth creates a Gmail client from OAuth2 user credentials.
// This is simpler for hackathon demos (no domain-wide delegation needed).
func NewGmailClientOAuth(credentialsJSON []byte, tokenJSON []byte) (*GmailClient, error) {
	ctx := context.Background()

	config, err := google.ConfigFromJSON(credentialsJSON, gapi.GmailReadonlyScope)
	if err != nil {
		return nil, fmt.Errorf("failed to parse OAuth credentials: %w", err)
	}

	tok, err := tokenFromJSON(tokenJSON)
	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	client := config.Client(ctx, tok)
	srv, err := gapi.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return nil, fmt.Errorf("failed to create Gmail service: %w", err)
	}

	return &GmailClient{service: srv, userID: "me"}, nil
}

// WatchInbox sets up Gmail push notifications via Pub/Sub.
func (c *GmailClient) WatchInbox(projectID, topicName string) error {
	fullTopic := fmt.Sprintf("projects/%s/topics/%s", projectID, topicName)

	resp, err := c.service.Users.Watch(c.userID, &gapi.WatchRequest{
		LabelIds:  []string{"INBOX"},
		TopicName: fullTopic,
	}).Do()
	if err != nil {
		return fmt.Errorf("gmail watch failed: %w", err)
	}

	expiry := time.UnixMilli(resp.Expiration)
	log.Printf("📧 Gmail watch active until %s (historyId: %d)", expiry.Format(time.RFC3339), resp.HistoryId)
	return nil
}

// GetMessage fetches a full email message by ID and extracts key fields.
func (c *GmailClient) GetMessage(messageID string) (*types.EmailData, error) {
	msg, err := c.service.Users.Messages.Get(c.userID, messageID).Format("full").Do()
	if err != nil {
		return nil, fmt.Errorf("failed to get message %s: %w", messageID, err)
	}

	email := &types.EmailData{
		MessageID: msg.Id,
		ThreadID:  msg.ThreadId,
	}

	// Extract headers
	for _, h := range msg.Payload.Headers {
		switch strings.ToLower(h.Name) {
		case "from":
			email.From = decodeHeader(h.Value)
		case "subject":
			email.Subject = decodeHeader(h.Value)
		case "date":
			email.Date = h.Value
		}
	}

	// Extract body (prefer plain text, fall back to HTML)
	email.Body = extractBody(msg.Payload)

	return email, nil
}

// GetConnectedEmail returns the email address of the monitored account.
func (c *GmailClient) GetConnectedEmail() string {
	profile, err := c.service.Users.GetProfile(c.userID).Do()
	if err != nil {
		log.Printf("⚠️ Failed to get Gmail profile: %v", err)
		return "unknown@gmail.com"
	}
	return profile.EmailAddress
}

// GetHistoryID returns the current historyId for the mailbox.
func (c *GmailClient) GetHistoryID() (uint64, error) {
	profile, err := c.service.Users.GetProfile(c.userID).Do()
	if err != nil {
		return 0, err
	}
	return profile.HistoryId, nil
}

// ListNewMessages returns message IDs added since the given historyId.
func (c *GmailClient) ListNewMessages(startHistoryID uint64) ([]string, error) {
	resp, err := c.service.Users.History.List(c.userID).
		StartHistoryId(startHistoryID).
		HistoryTypes("messageAdded").
		LabelId("INBOX").
		Do()
	if err != nil {
		return nil, fmt.Errorf("history.list failed: %w", err)
	}

	var ids []string
	seen := make(map[string]bool)
	for _, h := range resp.History {
		for _, m := range h.MessagesAdded {
			if !seen[m.Message.Id] {
				ids = append(ids, m.Message.Id)
				seen[m.Message.Id] = true
			}
		}
	}
	return ids, nil
}

// ── Helpers ─────────────────────────────────────────────

func extractBody(payload *gapi.MessagePart) string {
	// Try to find plain text part first
	if body := findPart(payload, "text/plain"); body != "" {
		return body
	}
	// Fall back to HTML
	if body := findPart(payload, "text/html"); body != "" {
		return stripHTML(body)
	}
	// Direct body data
	if payload.Body != nil && payload.Body.Data != "" {
		decoded, err := base64.URLEncoding.DecodeString(payload.Body.Data)
		if err == nil {
			return string(decoded)
		}
	}
	return "(no body)"
}

func findPart(part *gapi.MessagePart, mimeType string) string {
	if part.MimeType == mimeType && part.Body != nil && part.Body.Data != "" {
		decoded, err := base64.URLEncoding.DecodeString(part.Body.Data)
		if err == nil {
			return string(decoded)
		}
	}
	for _, child := range part.Parts {
		if result := findPart(child, mimeType); result != "" {
			return result
		}
	}
	return ""
}

func stripHTML(html string) string {
	// Simple HTML tag stripper for body preview
	var result strings.Builder
	inTag := false
	for _, r := range html {
		switch {
		case r == '<':
			inTag = true
		case r == '>':
			inTag = false
		case !inTag:
			result.WriteRune(r)
		}
	}
	return strings.TrimSpace(result.String())
}

func decodeHeader(value string) string {
	dec := new(mime.WordDecoder)
	decoded, err := dec.DecodeHeader(value)
	if err != nil {
		return value
	}
	return decoded
}

func tokenFromJSON(data []byte) (*oauth2.Token, error) {
	var tok oauth2.Token
	if err := json.Unmarshal(data, &tok); err != nil {
		return nil, err
	}
	return &tok, nil
}
