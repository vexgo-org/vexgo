package middleware

import (
	"log/slog"
	"net/http"
	"net/url"
	"time"

	"github.com/gin-gonic/gin"
)

// redactedQueryParams are query parameters whose values are credentials and
// must not end up in access logs.
var redactedQueryParams = map[string]struct{}{
	"token": {},
}

// sanitizeQuery strips credential-bearing query parameters (e.g. the emailed
// verification / reset token in ?token=...) before the URL is logged.
func sanitizeQuery(rawQuery string) string {
	if rawQuery == "" {
		return ""
	}
	values, err := url.ParseQuery(rawQuery)
	if err != nil {
		// Unparseable query: drop it rather than risk logging a token.
		return ""
	}
	redacted := false
	for key := range values {
		if _, sensitive := redactedQueryParams[key]; sensitive {
			values[key] = []string{"[REDACTED]"}
			redacted = true
		}
	}
	if !redacted {
		return rawQuery
	}
	return values.Encode()
}

// RequestLogger is a Gin middleware that logs HTTP requests and responses
func RequestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		// Get request details
		path := c.Request.URL.Path
		if raw := sanitizeQuery(c.Request.URL.RawQuery); raw != "" {
			path += "?" + raw
		}

		// Get user ID from context if available
		userID, exists := c.Get(CtxUserIDKey)
		if !exists {
			userID = "anonymous"
		}

		// Log request start
		slog.Debug(
			"request started",
			"method", c.Request.Method,
			"path", path,
			"userID", userID,
			"ip", c.ClientIP(),
		)

		// Process request
		c.Next()

		// Calculate duration
		duration := time.Since(start)

		// Log response
		status := c.Writer.Status()
		fields := []slog.Attr{
			slog.String("method", c.Request.Method),
			slog.String("path", path),
			slog.Int("status", status),
			slog.String("duration", duration.String()),
			slog.String("ip", c.ClientIP()),
			slog.String("userAgent", c.Request.UserAgent()),
			slog.String("referer", c.Request.Referer()),
		}

		// Add user ID if available
		if userID != "anonymous" {
			fields = append(fields, slog.Any("userID", userID))
		}

		// Add error info if status >= 400
		if status >= http.StatusBadRequest {
			errorMessages := c.Errors.Errors()
			if len(errorMessages) > 0 {
				fields = append(fields, slog.Any("errors", errorMessages))
			}
		}

		// Log based on status code
		switch {
		case status >= http.StatusInternalServerError:
			slog.LogAttrs(c.Request.Context(), slog.LevelError, "request completed with server error", fields...)
		case status >= http.StatusBadRequest:
			slog.LogAttrs(c.Request.Context(), slog.LevelWarn, "request completed with client error", fields...)
		case status >= http.StatusMultipleChoices:
			slog.LogAttrs(c.Request.Context(), slog.LevelInfo, "request completed with redirection", fields...)
		default:
			slog.LogAttrs(c.Request.Context(), slog.LevelInfo, "request completed successfully", fields...)
		}
	}
}
