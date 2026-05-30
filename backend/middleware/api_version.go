package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

const (
	CurrentAPIVersion    = "v2"
	DeprecatedAPIVersion = "v1"

	headerAPIVersion   = "X-API-Version"
	headerDeprecation  = "Deprecation"
	headerSunset       = "Sunset"
	headerLink         = "Link"

	// SunsetDate is the planned end-of-life date for v1
	SunsetDate = "Sat, 31 Dec 2026 23:59:59 GMT"
)

// VersionFromPath extracts the version segment (e.g. "v1", "v2") from the
// URL path and stores it in the gin context under the key "api_version".
func VersionFromPath() gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Request.URL.Path
		version := ""
		parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
		for _, p := range parts {
			if strings.HasPrefix(p, "v") && len(p) > 1 {
				version = p
				break
			}
		}
		if version == "" {
			version = CurrentAPIVersion
		}
		c.Set("api_version", version)
		c.Next()
	}
}

// DeprecationWarning injects Deprecation / Sunset headers on all v1 responses
// so clients are notified to migrate.
func DeprecationWarning() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		version, _ := c.Get("api_version")
		if version == DeprecatedAPIVersion {
			c.Header(headerAPIVersion, DeprecatedAPIVersion)
			c.Header(headerDeprecation, "true")
			c.Header(headerSunset, SunsetDate)
			c.Header(headerLink, `</api/v2>; rel="successor-version"`)
		} else {
			c.Header(headerAPIVersion, CurrentAPIVersion)
		}
	}
}

// RequireMinVersion aborts with 410 Gone if the request targets a version
// older than the minimum supported version.
func RequireMinVersion(minVersion string) gin.HandlerFunc {
	return func(c *gin.Context) {
		version, exists := c.Get("api_version")
		if exists && versionOlderThan(version.(string), minVersion) {
			c.AbortWithStatusJSON(http.StatusGone, gin.H{
				"error":   "API version no longer supported",
				"version": version,
				"migrate": "/api/" + minVersion,
			})
			return
		}
		c.Next()
	}
}

// versionOlderThan compares simple "vN" version strings.
func versionOlderThan(v, min string) bool {
	return strings.TrimPrefix(v, "v") < strings.TrimPrefix(min, "v")
}
