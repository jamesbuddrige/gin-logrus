package gin_logrus

import (
	"net/http"
	"runtime"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// UserClaimsKey is the key for user claims in context
const UserClaimsKey = "userClaims"

// GinLogrus is a middleware function that uses Logrus logger instead of the default Gin logger.
func GinLogrus(logger *logrus.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Start timer
		start := time.Now()

		// Process request
		c.Next()

		// Generate log fields
		fields := generateLogFields(c, start)

		// Create log entry
		entry := logger.WithFields(fields)

		// If user exists in context, add user ID to log entry.
		if user, ok := c.Get(UserClaimsKey); ok {
			if userMap, ok := user.(map[string]interface{}); ok {
				if userID, exists := userMap["UserID"].(string); exists {
					entry = entry.WithContext(c.Request.Context()).WithField("user.id", userID)
				}
			}
		}

		if len(c.Errors) > 0 {
			// Append error field if this is an erroneous request.
			entry.Errorf("Request failed: %v", c.Errors.String())
		} else {
			entry.Info("Request processed successfully")
		}
	}
}

func RecoveryWithLoggerMiddleware(logger *logrus.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				start := time.Now()
				// Capture the stack trace
				stack := make([]byte, 2048)
				stack = stack[:runtime.Stack(stack, false)]

				// Generate log fields
				fields := generateLogFields(c, start)

				// Create log entry
				entry := logger.WithFields(fields)

				// Log as a single entry
				entry.WithFields(logrus.Fields{
					"error.message":     err,
					"error.stack_trace": string(stack),
				}).Error("A panic occurred")

				span := trace.SpanFromContext(c.Request.Context())

				if span.SpanContext().IsValid() {
					// Set outcome
					span.SetStatus(codes.Error, "panic occurred")
				}

				// Optionally, you can write a response to the client
				c.AbortWithStatus(http.StatusInternalServerError)
			}
		}()

		// Process the next handler
		c.Next()
	}
}

func generateLogFields(c *gin.Context, start time.Time) logrus.Fields {
	// Calculate latency
	latency := time.Since(start)

	fields := logrus.Fields{
		"url.domain":                c.Request.Host,
		"url.fragment":              c.Request.URL.Fragment,
		"url.full":                  c.Request.URL.String(),
		"url.original":              c.Request.URL.String(),
		"url.path":                  c.Request.URL.Path,
		"url.port":                  c.Request.URL.Port(),
		"url.query":                 c.Request.URL.RawQuery,
		"url.registered_domain":     c.Request.URL.Hostname(),
		"url.scheme":                c.Request.URL.Scheme,
		"http.request.bytes":        c.Request.ContentLength,
		"http.request.method":       c.Request.Method,
		"http.request.mime_type":    c.ContentType(),
		"http.request.referrer":     c.Request.Referer(),
		"http.response.body.bytes":  c.Writer.Size(),
		"http.response.status_code": c.Writer.Status(),
		"http.version":              c.Request.Proto,
		"client.address":            c.ClientIP(),
		"client.ip":                 c.ClientIP(),
		"server.address":            c.Request.Host,
		"server.ip":                 c.Request.Host,
		"user_agent.original":       c.Request.UserAgent(),
		"event.duration":            latency.Seconds(),
		"event.start":               start.Format("2006-01-02T15:04:05.000Z"),
		"event.end":                 time.Now().Format("2006-01-02T15:04:05.000Z"),
	}

	// If user exists in context, add user ID to fields.
	if user, ok := c.Get(UserClaimsKey); ok {
		// Check if it's a map with string keys and interface{} values
		if userMap, ok := user.(map[string]interface{}); ok {
			if userID, exists := userMap["UserID"].(string); exists {
				fields["user.id"] = userID
			}
		}
	}

	return fields
}
