package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// RedirectMiddleware returns a Gin middleware that handles URL redirections.
// It provides backward compatibility by redirecting old '/xui' paths to new '/panel' paths,
// including API endpoints. The middleware performs permanent redirects (301) for SEO purposes.
func RedirectMiddleware(basePath string) gin.HandlerFunc {
	// Use a slice to guarantee longest-prefix-first matching order.
	// A map would have nondeterministic iteration, causing "/xui/API" to
	// sometimes match the shorter "/xui" rule instead.
	redirects := []struct{ from, to string }{
		{"panel/API", "panel/api"},
		{"xui/API", "panel/api"},
		{"xui", "panel"},
	}

	return func(c *gin.Context) {
		path := c.Request.URL.Path
		for _, r := range redirects {
			from := basePath + r.from
			to := basePath + r.to

			if strings.HasPrefix(path, from) {
				newPath := to + path[len(from):]

				c.Redirect(http.StatusMovedPermanently, newPath)
				c.Abort()
				return
			}
		}

		c.Next()
	}
}
