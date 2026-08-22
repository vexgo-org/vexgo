package middleware

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// RequestLogger is a Gin middleware that logs HTTP requests and responses
func RequestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		// Get request details
		path := c.Request.URL.Path
		raw := c.Request.URL.RawQuery
		if raw != "" {
			path += "?" + raw
		}

		// Get user ID from context if available
		userID, exists := c.Get(CtxUserIDKey)
		if !exists {
			userID = "anonymous"
		}

		// Log request start
		logrus.WithFields(logrus.Fields{
			"method": c.Request.Method,
			"path":   path,
			"userID": userID,
			"ip":     c.ClientIP(),
		}).Debug("Request started")

		// Process request
		c.Next()

		// Calculate duration
		duration := time.Since(start)

		// Log response
		status := c.Writer.Status()
		fields := logrus.Fields{
			"method":    c.Request.Method,
			"path":      path,
			"status":    status,
			"duration":  duration.String(),
			"ip":        c.ClientIP(),
			"userAgent": c.Request.UserAgent(),
			"referer":   c.Request.Referer(),
		}

		// Add user ID if available
		if userID != "anonymous" {
			fields["userID"] = userID
		}

		// Add error info if status >= 400
		if status >= http.StatusBadRequest {
			errorMessages := c.Errors.Errors()
			if len(errorMessages) > 0 {
				fields["errors"] = errorMessages
			}
		}

		// Log based on status code
		if status >= http.StatusInternalServerError {
			logrus.WithFields(fields).Error("Request completed with server error")
		} else if status >= http.StatusBadRequest {
			logrus.WithFields(fields).Warn("Request completed with client error")
		} else if status >= http.StatusMultipleChoices {
			logrus.WithFields(fields).Info("Request completed with redirection")
		} else {
			logrus.WithFields(fields).Info("Request completed successfully")
		}
	}
}
