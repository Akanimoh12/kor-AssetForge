package middleware

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

type DistributedRateLimiter struct {
	redisClient *redis.Client
	config      *DistributedRateLimitConfig
	logger      *zap.SugaredLogger
}

type DistributedRateLimitConfig struct {
	GetRPS      float64
	GetBurst    int
	MutateRPS   float64
	MutateBurst int
	TTL         time.Duration
	logger      *zap.SugaredLogger
}

func NewDistributedRateLimiter(redisClient *redis.Client, cfg *DistributedRateLimitConfig) *DistributedRateLimiter {
	if cfg.TTL == 0 {
		cfg.TTL = 1 * time.Hour
	}
	return &DistributedRateLimiter{
		redisClient: redisClient,
		config:      cfg,
		logger:      cfg.logger,
	}
}

func (drl *DistributedRateLimiter) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		key := drl.getBucketKey(c)

		isMutating := c.Request.Method != http.MethodGet &&
			c.Request.Method != http.MethodHead &&
			c.Request.Method != http.MethodOptions

		var rps float64
		var burst int
		if isMutating {
			rps = drl.config.MutateRPS
			burst = drl.config.MutateBurst
		} else {
			rps = drl.config.GetRPS
			burst = drl.config.GetBurst
		}

		allowed, remaining := drl.checkLimit(c.Request.Context(), key, rps, burst)

		c.Header("X-RateLimit-Limit", strconv.Itoa(burst))
		c.Header("X-RateLimit-Remaining", strconv.Itoa(remaining))
		c.Header("X-RateLimit-Reset", strconv.FormatInt(time.Now().Add(drl.config.TTL).Unix(), 10))

		if !allowed {
			if drl.logger != nil {
				drl.logger.Warnw("distributed rate limit exceeded",
					"key", key,
					"method", c.Request.Method,
					"path", c.FullPath(),
				)
			}
			retryAfter := int(1.0 / rps)
			if retryAfter < 1 {
				retryAfter = 1
			}
			c.Header("Retry-After", strconv.Itoa(retryAfter))
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error":       "rate limit exceeded",
				"retry_after": retryAfter,
			})
			return
		}

		c.Next()
	}
}

func (drl *DistributedRateLimiter) getBucketKey(c *gin.Context) string {
	if uid, ok := c.Get("user_id"); ok {
		return fmt.Sprintf("rate_limit:uid:%v", uid)
	}
	if header := c.GetHeader("X-User-ID"); header != "" {
		return fmt.Sprintf("rate_limit:uid:%s", header)
	}
	return fmt.Sprintf("rate_limit:ip:%s", c.ClientIP())
}

func (drl *DistributedRateLimiter) checkLimit(ctx context.Context, key string, rps float64, burst int) (bool, int) {
	script := redis.NewScript(`
		local key = KEYS[1]
		local burst = tonumber(ARGV[1])
		local ttl = tonumber(ARGV[2])
		local now = tonumber(ARGV[3])

		local current = redis.call('GET', key)
		if current == false then
			current = 0
		else
			current = tonumber(current)
		end

		if current < burst then
			redis.call('INCR', key)
			redis.call('EXPIRE', key, ttl)
			return {1, burst - current - 1}
		else
			return {0, 0}
		end
	`)

	result, err := script.Run(ctx, drl.redisClient, []string{key},
		burst,
		int64(drl.config.TTL.Seconds()),
		time.Now().Unix(),
	).Slice()

	if err != nil {
		if drl.logger != nil {
			drl.logger.Warnw("redis rate limit check failed", "error", err)
		}
		return true, burst
	}

	if len(result) >= 2 {
		allowed := result[0].(int64) == 1
		remaining := int(result[1].(int64))
		return allowed, remaining
	}

	return true, burst
}
