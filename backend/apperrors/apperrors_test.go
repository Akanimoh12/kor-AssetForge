package apperrors

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestFormatErrorResponseIncludesDebugWhenEnabled(t *testing.T) {
	appErr := Wrap(nil, CodeBadRequest, "Request payload invalid", http.StatusBadRequest)
	appErr.Err = errDummy
	resp := FormatErrorResponse(appErr, "req-123", true)

	if resp.Debug != errDummy.Error() {
		t.Fatalf("expected debug field to include internal error, got %q", resp.Debug)
	}
}

func TestFormatErrorResponseHidesDebugWhenDisabled(t *testing.T) {
	appErr := Wrap(nil, CodeBadRequest, "Request payload invalid", http.StatusBadRequest)
	appErr.Err = errDummy
	resp := FormatErrorResponse(appErr, "req-123", false)

	if resp.Debug != "" {
		t.Fatalf("expected debug field to be empty when debug disabled, got %q", resp.Debug)
	}
}

func TestErrorHandlerFormatsAppError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(ErrorHandler(false))
	router.GET("/test", func(c *gin.Context) {
		AbortWithError(c, New(CodeBadRequest, "Invalid request", http.StatusBadRequest))
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d got %d", http.StatusBadRequest, w.Code)
	}

	var body ErrorResponse
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to parse response body: %v", err)
	}

	if body.Code != CodeBadRequest {
		t.Fatalf("expected code %q got %q", CodeBadRequest, body.Code)
	}
}

var errDummy = New(CodeBadRequest, "dummy", http.StatusBadRequest)
