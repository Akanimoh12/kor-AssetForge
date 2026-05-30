package services

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/yourusername/kor-assetforge/models"
	"gorm.io/gorm"
)

const (
	maxRetries     = 5
	initialBackoff = 30 * time.Second
)

// WebhookDeliveryService dispatches outgoing webhook events and handles retries
type WebhookDeliveryService struct {
	db         *gorm.DB
	httpClient *http.Client
}

// NewWebhookDeliveryService creates a new delivery service
func NewWebhookDeliveryService(db *gorm.DB) *WebhookDeliveryService {
	return &WebhookDeliveryService{
		db: db,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Dispatch finds all active subscriptions for the given event type, creates
// delivery log entries, and attempts delivery immediately.
func (s *WebhookDeliveryService) Dispatch(eventType models.WebhookEventType, payload interface{}) {
	raw, err := json.Marshal(payload)
	if err != nil {
		log.Printf("[webhook] failed to marshal payload: %v", err)
		return
	}

	var subs []models.WebhookSubscription
	s.db.Where("active = ?", true).Find(&subs)

	for _, sub := range subs {
		if !s.subscribedTo(sub, eventType) {
			continue
		}

		entry := models.WebhookDeliveryLog{
			SubscriptionID: sub.ID,
			EventType:      eventType,
			Payload:        string(raw),
			Status:         models.WebhookDeliveryPending,
		}
		if err := s.db.Create(&entry).Error; err != nil {
			log.Printf("[webhook] failed to create delivery log: %v", err)
			continue
		}

		go s.deliver(sub, &entry)
	}
}

// RetryPending retries delivery for all failed or pending log entries whose
// next_retry_at is in the past. Call this from a background job or cron.
func (s *WebhookDeliveryService) RetryPending() {
	var entries []models.WebhookDeliveryLog
	now := time.Now()
	s.db.Where(
		"status IN ? AND (next_retry_at IS NULL OR next_retry_at <= ?) AND attempt_count < ?",
		[]string{
			string(models.WebhookDeliveryPending),
			string(models.WebhookDeliveryRetrying),
			string(models.WebhookDeliveryFailed),
		},
		now,
		maxRetries,
	).Find(&entries)

	for i := range entries {
		var sub models.WebhookSubscription
		if err := s.db.First(&sub, entries[i].SubscriptionID).Error; err != nil {
			continue
		}
		go s.deliver(sub, &entries[i])
	}
}

func (s *WebhookDeliveryService) deliver(sub models.WebhookSubscription, entry *models.WebhookDeliveryLog) {
	entry.AttemptCount++
	entry.Status = models.WebhookDeliveryRetrying

	sig := s.sign([]byte(entry.Payload), sub.Secret)
	req, err := http.NewRequest(http.MethodPost, sub.URL, bytes.NewBufferString(entry.Payload))
	if err != nil {
		s.markFailed(entry, 0, fmt.Sprintf("build request: %v", err))
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-AssetForge-Signature", sig)
	req.Header.Set("X-AssetForge-Event", string(entry.EventType))
	req.Header.Set("X-AssetForge-Delivery", fmt.Sprintf("%d", entry.ID))

	resp, err := s.httpClient.Do(req)
	if err != nil {
		s.scheduleRetry(entry, 0, fmt.Sprintf("http error: %v", err))
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	entry.HTTPStatus = resp.StatusCode
	entry.ResponseBody = string(body)

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		now := time.Now()
		entry.Status = models.WebhookDeliverySuccess
		entry.DeliveredAt = &now
		s.db.Save(entry)
		return
	}

	if entry.AttemptCount >= maxRetries {
		entry.Status = models.WebhookDeliveryAbandoned
		s.db.Save(entry)
		return
	}

	s.scheduleRetry(entry, resp.StatusCode, string(body))
}

func (s *WebhookDeliveryService) scheduleRetry(entry *models.WebhookDeliveryLog, httpStatus int, responseBody string) {
	backoff := initialBackoff * time.Duration(1<<uint(entry.AttemptCount-1))
	next := time.Now().Add(backoff)
	entry.Status = models.WebhookDeliveryFailed
	entry.HTTPStatus = httpStatus
	entry.ResponseBody = responseBody
	entry.NextRetryAt = &next
	s.db.Save(entry)
}

func (s *WebhookDeliveryService) markFailed(entry *models.WebhookDeliveryLog, httpStatus int, reason string) {
	entry.Status = models.WebhookDeliveryAbandoned
	entry.HTTPStatus = httpStatus
	entry.ResponseBody = reason
	s.db.Save(entry)
}

func (s *WebhookDeliveryService) subscribedTo(sub models.WebhookSubscription, event models.WebhookEventType) bool {
	for _, e := range strings.Split(sub.Events, ",") {
		if strings.TrimSpace(e) == string(event) || strings.TrimSpace(e) == "*" {
			return true
		}
	}
	return false
}

func (s *WebhookDeliveryService) sign(payload []byte, secret string) string {
	h := hmac.New(sha256.New, []byte(secret))
	h.Write(payload)
	return "sha256=" + hex.EncodeToString(h.Sum(nil))
}
