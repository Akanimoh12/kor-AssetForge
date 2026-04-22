package apperrors

import (
	"net/http"
	"sync"
	"sync/atomic"

	"github.com/gin-gonic/gin"
)

var errorCounters sync.Map

func IncrementErrorCounter(code ErrorCode) {
	if code == "" {
		code = CodeInternalServerError
	}

	counter, _ := errorCounters.LoadOrStore(code, new(uint64))
	atomic.AddUint64(counter.(*uint64), 1)
}

func GetErrorMetrics() map[string]uint64 {
	metrics := make(map[string]uint64)
	errorCounters.Range(func(key, value interface{}) bool {
		if code, ok := key.(ErrorCode); ok {
			metrics[string(code)] = atomic.LoadUint64(value.(*uint64))
		}
		return true
	})
	return metrics
}

func MetricsHandler(c *gin.Context) {
	c.JSON(http.StatusOK, GetErrorMetrics())
}
