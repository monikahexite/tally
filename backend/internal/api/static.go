package api

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
)

// spaHandler serves the built React app. Real files are served as-is
// (hashed assets cached forever); every other path falls back to index.html
// so client-side routes like /p/abc123 work on refresh.
func spaHandler(dir string) gin.HandlerFunc {
	root, _ := filepath.Abs(dir)
	index := filepath.Join(root, "index.html")
	return func(c *gin.Context) {
		if c.Request.Method != http.MethodGet && c.Request.Method != http.MethodHead {
			fail(c, http.StatusNotFound, "not_found", "Not found")
			return
		}
		clean := path.Clean("/" + c.Request.URL.Path) // strips any ../
		file := filepath.Join(root, filepath.FromSlash(clean))
		if strings.HasPrefix(file, root) {
			if info, err := os.Stat(file); err == nil && !info.IsDir() {
				if strings.HasPrefix(clean, "/assets/") {
					c.Header("Cache-Control", "public, max-age=31536000, immutable")
				}
				c.File(file)
				return
			}
		}
		c.Header("Cache-Control", "no-cache")
		c.File(index)
	}
}

func randomHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
