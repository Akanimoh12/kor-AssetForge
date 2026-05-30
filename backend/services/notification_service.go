package services

import (
	"log"

	"github.com/yourusername/kor-assetforge/models"
	"gorm.io/gorm"
)

// NotificationService creates and dispatches notifications
type NotificationService struct {
	db           *gorm.DB
	emailService EmailService
}

// NewNotificationService creates a NotificationService
func NewNotificationService(db *gorm.DB, emailService EmailService) *NotificationService {
	return &NotificationService{db: db, emailService: emailService}
}

// Notify creates an in-app notification and optionally sends email/push based
// on the user's preferences.
func (s *NotificationService) Notify(userID uint, notifType models.NotificationType, title, body string, resourceID *uint, resourceType string) {
	// Check preferences
	pref := s.getPreferences(userID, notifType)

	if pref.InApp {
		n := models.Notification{
			UserID:       userID,
			Type:         notifType,
			Title:        title,
			Body:         body,
			ResourceID:   resourceID,
			ResourceType: resourceType,
		}
		if err := s.db.Create(&n).Error; err != nil {
			log.Printf("[notification] failed to persist in-app: %v", err)
		}
	}

	if pref.Email && s.emailService != nil {
		var user models.User
		if err := s.db.First(&user, userID).Error; err == nil && user.Email != "" {
			go func() {
				// Reuse KYC status email as a generic notification channel until
				// a dedicated SendNotification method is added to EmailService.
				if err := s.emailService.SendKYCStatusUpdate(user.Email, user.Username, title, body); err != nil {
					log.Printf("[notification] email failed for user %d: %v", userID, err)
				}
			}()
		}
	}

	if pref.Push {
		// Push notification delivery would integrate with a provider (FCM/APNs).
		// Placeholder for future integration.
		log.Printf("[notification] push placeholder for user %d type %s", userID, notifType)
	}
}

// getPreferences returns stored preferences or sensible defaults
func (s *NotificationService) getPreferences(userID uint, notifType models.NotificationType) models.NotificationPreference {
	var pref models.NotificationPreference
	if err := s.db.Where("user_id = ? AND notification_type = ?", userID, notifType).
		First(&pref).Error; err != nil {
		// Default: in_app + email on, push off
		return models.NotificationPreference{
			UserID:           userID,
			NotificationType: notifType,
			InApp:            true,
			Email:            true,
			Push:             false,
		}
	}
	return pref
}
