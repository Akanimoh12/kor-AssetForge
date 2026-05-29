package middleware

import (
	"context"
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type AuditLog struct {
	ID        uint      `gorm:"primaryKey"`
	CreatedAt time.Time
	UserID    string    `gorm:"index"`
	Action    string    `gorm:"index"`
	Resource  string
	ResourceID string
	Method    string
	Path      string
	Status    int
	IPAddress string
	Details   string
	Error     string
}

type AuditService struct {
	db *gorm.DB
}

func NewAuditService(db *gorm.DB) *AuditService {
	return &AuditService{db: db}
}

func (as *AuditService) LogAction(ctx context.Context, userID string, action string, resource string, resourceID string, method string, path string, status int, ip string, details string) error {
	log := AuditLog{
		UserID:     userID,
		Action:     action,
		Resource:   resource,
		ResourceID: resourceID,
		Method:     method,
		Path:       path,
		Status:     status,
		IPAddress:  ip,
		Details:    details,
		CreatedAt:  time.Now(),
	}
	return as.db.WithContext(ctx).Create(&log).Error
}

func (as *AuditService) LogError(ctx context.Context, userID string, action string, resource string, method string, path string, ip string, err error) error {
	log := AuditLog{
		UserID:    userID,
		Action:    action,
		Resource:  resource,
		Method:    method,
		Path:      path,
		IPAddress: ip,
		Error:     err.Error(),
		CreatedAt: time.Now(),
		Status:    500,
	}
	return as.db.WithContext(ctx).Create(&log).Error
}

func (as *AuditService) GetAuditLogs(ctx context.Context, userID string, limit int, offset int) ([]AuditLog, error) {
	var logs []AuditLog
	err := as.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&logs).Error
	return logs, err
}

func (as *AuditService) SearchAuditLogs(ctx context.Context, query string, action string, resource string, startTime time.Time, endTime time.Time) ([]AuditLog, error) {
	var logs []AuditLog
	db := as.db.WithContext(ctx)

	if query != "" {
		db = db.Where("user_id LIKE ? OR path LIKE ?", "%"+query+"%", "%"+query+"%")
	}
	if action != "" {
		db = db.Where("action = ?", action)
	}
	if resource != "" {
		db = db.Where("resource = ?", resource)
	}
	if !startTime.IsZero() {
		db = db.Where("created_at >= ?", startTime)
	}
	if !endTime.IsZero() {
		db = db.Where("created_at <= ?", endTime)
	}

	err := db.Order("created_at DESC").Find(&logs).Error
	return logs, err
}

func AuditMiddleware(auditService *AuditService) gin.HandlerFunc {
	return func(c *gin.Context) {
		startTime := time.Now()

		c.Next()

		userID := ""
		if uid, ok := c.Get("user_id"); ok {
			userID = fmt.Sprintf("%v", uid)
		}

		isSensitiveAction := isSensitiveOperation(c.Request.Method, c.Request.URL.Path)
		if isSensitiveAction {
			err := auditService.LogAction(
				context.Background(),
				userID,
				getActionType(c.Request.Method),
				extractResource(c.Request.URL.Path),
				extractResourceID(c.Request.URL.Path),
				c.Request.Method,
				c.Request.URL.Path,
				c.Writer.Status(),
				c.ClientIP(),
				fmt.Sprintf("duration: %dms", time.Since(startTime).Milliseconds()),
			)
			if err != nil {
				c.Error(fmt.Errorf("audit log failed: %w", err))
			}
		}
	}
}

func isSensitiveOperation(method string, path string) bool {
	if method == "GET" || method == "HEAD" || method == "OPTIONS" {
		return false
	}
	if method == "POST" && (contains(path, "login") || contains(path, "register")) {
		return true
	}
	if contains(path, "admin") || contains(path, "kyc") || contains(path, "compliance") {
		return true
	}
	if contains(path, "transfer") || contains(path, "transaction") {
		return true
	}
	return true
}

func getActionType(method string) string {
	switch method {
	case "POST":
		return "CREATE"
	case "PUT", "PATCH":
		return "UPDATE"
	case "DELETE":
		return "DELETE"
	default:
		return "READ"
	}
}

func extractResource(path string) string {
	parts := splitPath(path)
	if len(parts) > 0 {
		return parts[0]
	}
	return "unknown"
}

func extractResourceID(path string) string {
	parts := splitPath(path)
	if len(parts) > 1 {
		return parts[1]
	}
	return ""
}

func splitPath(path string) []string {
	var parts []string
	var current string
	for _, c := range path {
		if c == '/' {
			if current != "" {
				parts = append(parts, current)
				current = ""
			}
		} else {
			current += string(c)
		}
	}
	if current != "" {
		parts = append(parts, current)
	}
	return parts
}

func contains(s string, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
