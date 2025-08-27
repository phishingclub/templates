package handler

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// AssetHandler creates a handler for serving template assets with fallback support
func AssetHandler(baseDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Get the requested path (already stripped of /templates/ prefix)
		reqPath := r.URL.Path
		
		// Clean the path to prevent directory traversal
		cleanPath := filepath.Clean(reqPath)
		if strings.Contains(cleanPath, "..") {
			http.Error(w, "Invalid path", http.StatusBadRequest)
			return
		}
		
		// Build the primary filesystem path
		primaryPath := filepath.Join(baseDir, cleanPath)
		
		// Try to serve the file from the primary location
		if info, err := os.Stat(primaryPath); err == nil && !info.IsDir() {
			http.ServeFile(w, r, primaryPath)
			return
		}
		
		// Primary location failed, try asset fallback
		// Check if this looks like a template asset path that should fall back to global assets
		if strings.Contains(reqPath, "/") {
			pathParts := strings.Split(strings.TrimPrefix(reqPath, "/"), "/")
			
			// For template asset requests like: Microsoft/Emails/Template/microsoft/microsoft-logo.png
			// We want to extract the asset part (microsoft/microsoft-logo.png) and check in global assets
			if len(pathParts) >= 2 {
				// Get the last two parts (directory/filename) as potential asset path
				assetPath := strings.Join(pathParts[len(pathParts)-2:], "/")
				fallbackPath := filepath.Join(baseDir, "assets", assetPath)
				
				if info, err := os.Stat(fallbackPath); err == nil && !info.IsDir() {
					http.ServeFile(w, r, fallbackPath)
					return
				}
				
				// If that doesn't work, try just the filename
				filename := pathParts[len(pathParts)-1]
				fallbackPath = filepath.Join(baseDir, "assets", filename)
				
				if info, err := os.Stat(fallbackPath); err == nil && !info.IsDir() {
					http.ServeFile(w, r, fallbackPath)
					return
				}
			}
		}
		
		// No fallback worked, return 404
		http.NotFound(w, r)
	}
}