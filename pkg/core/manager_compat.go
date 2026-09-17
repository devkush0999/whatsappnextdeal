// Modified for standalone operation: external activation and telemetry removed.
package core

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// RegisterManagerCompatibilityRoutes supports the bundled manager until its
// source is available to remove the legacy activation UI. "active" describes
// local runtime availability; these endpoints neither validate nor grant a
// software license. API authorization remains in the authentication middleware.
func RegisterManagerCompatibilityRoutes(eng *gin.Engine) {
	status := func(c *gin.Context) {
		c.Header("Cache-Control", "no-store")
		c.JSON(http.StatusOK, gin.H{
			"status":           "active",
			"mode":             "standalone",
			"license_required": false,
			"message":          "Standalone mode; external activation is not required.",
		})
	}

	// Old bookmarks/callbacks also resolve locally, without exchanging codes or
	// accepting credentials for a third-party service.
	eng.GET("/license/status", status)
	eng.GET("/license/register", status)
	eng.GET("/license/activate", status)
}
