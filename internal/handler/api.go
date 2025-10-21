package handler

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// DirectoryItem represents a single file or directory in the navigation tree
type DirectoryItem struct {
	Name  string `json:"name"`
	IsDir bool   `json:"isDir"`
}

// CampaignInfo represents campaign metadata
type CampaignInfo struct {
	Name string `yaml:"name"`
	Path string
	Dir  string
}

// DuplicateError represents a duplicate campaign error
type DuplicateError struct {
	Type      string   // "name" or "folder"
	Value     string   // the duplicate name or folder
	Campaigns []string // paths of conflicting campaigns
}

func (e DuplicateError) Error() string {
	if e.Type == "name" {
		return fmt.Sprintf("Duplicate campaign name '%s' found in: %s", e.Value, strings.Join(e.Campaigns, ", "))
	}
	return fmt.Sprintf("Multiple campaigns found in folder '%s': %s", e.Value, strings.Join(e.Campaigns, ", "))
}

// DownloadHandler creates a zip archive of a directory and sends it to the client
func DownloadHandler(baseDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// First validate campaigns for duplicates
		err := validateCampaigns(baseDir)
		if err != nil {
			http.Error(w, fmt.Sprintf(`{"error":"Campaign validation failed: %s"}`, err), http.StatusConflict)
			return
		}

		// Get requested path from query parameter
		reqPath := r.URL.Query().Get("path")
		if reqPath == "" {
			http.Error(w, `{"error":"No path specified"}`, http.StatusBadRequest)
			return
		}

		// Remove any starting slash for consistency and validate path
		reqPath = strings.TrimPrefix(reqPath, "/")

		// Clean the path to prevent directory traversal
		cleanPath := filepath.Clean(reqPath)
		if strings.Contains(cleanPath, "..") {
			http.Error(w, `{"error":"Invalid path"}`, http.StatusBadRequest)
			return
		}

		// Build the filesystem path
		fsPath := filepath.Join(baseDir, cleanPath)

		// Ensure the path is within baseDir
		absBaseDir, _ := filepath.Abs(baseDir)
		absPath, _ := filepath.Abs(fsPath)
		if !strings.HasPrefix(absPath, absBaseDir) {
			http.Error(w, `{"error":"Invalid path"}`, http.StatusBadRequest)
			return
		}

		// Check if path exists and is a directory
		info, err := os.Stat(fsPath)
		if os.IsNotExist(err) {
			http.Error(w, `{"error":"Path not found"}`, http.StatusNotFound)
			return
		}
		if err != nil {
			http.Error(w, `{"error":"Error accessing path"}`, http.StatusInternalServerError)
			return
		}
		if !info.IsDir() {
			http.Error(w, `{"error":"Not a directory"}`, http.StatusBadRequest)
			return
		}

		// Create a timestamp for the zip filename
		timestamp := time.Now().Format("20060102-150405")

		// Get the directory name for the zip file name
		dirName := filepath.Base(reqPath)
		if dirName == "." || dirName == "" {
			dirName = "templates"
		}

		// Set filename with directory name and timestamp
		zipFilename := fmt.Sprintf("%s-%s.zip", dirName, timestamp)

		// Set headers for file download
		w.Header().Set("Content-Type", "application/zip")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", zipFilename))

		// Create a new zip archive writing directly to the response
		zipWriter := zip.NewWriter(w)
		defer zipWriter.Close()

		// Walk the directory and add files to the zip
		err = filepath.Walk(fsPath, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			// Create a zip header based on the file info
			header, err := zip.FileInfoHeader(info)
			if err != nil {
				return err
			}

			// Set the name based on the relative path from the base directory
			relPath, err := filepath.Rel(fsPath, path)
			if err != nil {
				return err
			}

			// Skip the current directory
			if relPath == "." {
				return nil
			}

			// Ensure forward slashes for compatibility
			header.Name = filepath.ToSlash(relPath)

			// Set appropriate method for directories or files
			if info.IsDir() {
				header.Name += "/"
				header.Method = zip.Store
			} else {
				header.Method = zip.Deflate
			}

			// Create writer for the file
			writer, err := zipWriter.CreateHeader(header)
			if err != nil {
				return err
			}

			// If it's a directory, we're done
			if info.IsDir() {
				return nil
			}

			// Open the file for reading
			file, err := os.Open(path)
			if err != nil {
				return err
			}
			defer file.Close()

			// Copy file contents to the zip writer
			_, err = io.Copy(writer, file)
			return err
		})

		if err != nil {
			http.Error(w, fmt.Sprintf(`{"error":"Error creating zip file: %s"}`, err), http.StatusInternalServerError)
			return
		}
	}
}

// ExportHandler creates a structured zip export with assets and templates
func ExportHandler(baseDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// First validate campaigns for duplicates
		err := validateCampaigns(baseDir)
		if err != nil {
			http.Error(w, fmt.Sprintf(`{"error":"Campaign validation failed: %s"}`, err), http.StatusConflict)
			return
		}

		// Create a timestamp for the zip filename
		timestamp := time.Now().Format("20060102-150405")
		zipFilename := fmt.Sprintf("export-%s.zip", timestamp)

		// Set headers for file download
		w.Header().Set("Content-Type", "application/zip")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", zipFilename))

		// Create a new zip archive writing directly to the response
		zipWriter := zip.NewWriter(w)
		defer zipWriter.Close()

		// Process assets (check both "assets" and "Assets" directories)
		assetsPath := filepath.Join(baseDir, "assets")
		if _, err := os.Stat(assetsPath); err == nil {
			err = addAssets(zipWriter, assetsPath)
			if err != nil {
				http.Error(w, fmt.Sprintf(`{"error":"Error processing assets: %s"}`, err), http.StatusInternalServerError)
				return
			}
		}

		// Also check for "Assets" directory (legacy support)
		assetsPathCap := filepath.Join(baseDir, "Assets")
		if _, err := os.Stat(assetsPathCap); err == nil {
			err = addAssets(zipWriter, assetsPathCap)
			if err != nil {
				http.Error(w, fmt.Sprintf(`{"error":"Error processing Assets: %s"}`, err), http.StatusInternalServerError)
				return
			}
		}

		// Process phishing templates
		err = addPhishingTemplates(zipWriter, baseDir)
		if err != nil {
			http.Error(w, fmt.Sprintf(`{"error":"Error processing templates: %s"}`, err), http.StatusInternalServerError)
			return
		}
	}
}

// ValidateCampaignsHandler provides an endpoint to validate campaigns for duplicates
func ValidateCampaignsHandler(baseDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		err := validateCampaigns(baseDir)
		if err != nil {
			// Return conflict status with detailed error information
			response := map[string]interface{}{
				"valid": false,
				"error": err.Error(),
			}

			if dupErr, ok := err.(DuplicateError); ok {
				response["type"] = dupErr.Type
				response["value"] = dupErr.Value
				response["campaigns"] = dupErr.Campaigns
			}

			w.WriteHeader(http.StatusConflict)
			json.NewEncoder(w).Encode(response)
			return
		}

		// No conflicts found
		response := map[string]interface{}{
			"valid":   true,
			"message": "No campaign conflicts detected",
		}
		json.NewEncoder(w).Encode(response)
	}
}

// addAssets adds all folders from assets/ in the zip
func addAssets(zipWriter *zip.Writer, assetsPath string) error {
	return filepath.Walk(assetsPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Get relative path from assets directory
		relPath, err := filepath.Rel(assetsPath, path)
		if err != nil {
			return err
		}

		// Skip the root assets directory
		if relPath == "." {
			return nil
		}

		// Create the path in assets folder
		zipPath := filepath.Join("assets", relPath)
		zipPath = filepath.ToSlash(zipPath) // Ensure forward slashes

		// Create zip header
		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}

		header.Name = zipPath
		if info.IsDir() {
			header.Name += "/"
			header.Method = zip.Store
		} else {
			header.Method = zip.Deflate
		}

		// Create writer for the file
		writer, err := zipWriter.CreateHeader(header)
		if err != nil {
			return err
		}

		// If it's a directory, we're done
		if info.IsDir() {
			return nil
		}

		// Open and copy file contents
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()

		_, err = io.Copy(writer, file)
		return err
	})
}

// validateCampaigns checks for duplicate campaign names and folder conflicts
func validateCampaigns(baseDir string) error {
	campaigns := make([]CampaignInfo, 0)
	nameMap := make(map[string][]string)
	folderMap := make(map[string][]string)

	// Collect all campaigns
	err := filepath.Walk(baseDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip if not a directory
		if !info.IsDir() {
			return nil
		}

		// Skip the assets directories
		if strings.Contains(path, "Assets") || strings.Contains(path, "assets") {
			return filepath.SkipDir
		}

		// Skip private directories (client-specific content that should not be validated)
		relPath, err := filepath.Rel(baseDir, path)
		if err == nil {
			pathComponents := strings.Split(filepath.ToSlash(relPath), "/")
			if len(pathComponents) > 0 && strings.ToLower(pathComponents[0]) == "private" {
				return filepath.SkipDir
			}
		}

		// Check if this directory contains any HTML files
		hasHTML, err := containsHTMLFiles(path)
		if err != nil {
			return err
		}

		// If this directory contains HTML files, it's a campaign directory
		if hasHTML {
			relPath, err := filepath.Rel(baseDir, path)
			if err != nil {
				return err
			}

			campaign := CampaignInfo{
				Path: relPath,
				Dir:  filepath.Base(path),
			}

			// Try to read campaign name from data.yaml (top-level name field)
			dataYamlPath := filepath.Join(path, "data.yaml")
			if _, err := os.Stat(dataYamlPath); err == nil {
				data, err := os.ReadFile(dataYamlPath)
				if err == nil {
					var yamlData struct {
						Name string `yaml:"name"`
						// Note: emails and landing_pages sections are ignored for campaign-level validation
						// as they can have the same names within a single campaign
					}
					if yaml.Unmarshal(data, &yamlData) == nil && yamlData.Name != "" {
						campaign.Name = yamlData.Name
					}
				}
			}

			// If no name in data.yaml, use directory name as campaign name
			if campaign.Name == "" {
				campaign.Name = campaign.Dir
			}

			campaigns = append(campaigns, campaign)

			// Track by campaign name (not individual email/page names)
			nameMap[campaign.Name] = append(nameMap[campaign.Name], campaign.Path)

			// Track by folder (directory name) - prevent same folder conflicts
			folderMap[campaign.Dir] = append(folderMap[campaign.Dir], campaign.Path)
		}

		return nil
	})

	if err != nil {
		return err
	}

	// Check for duplicate campaign names first
	for name, paths := range nameMap {
		if len(paths) > 1 {
			// Check if this is a legitimate email/landing page organization
			isEmailLandingOrg := true
			orgTypes := make(map[string]bool)

			for _, path := range paths {
				pathParts := strings.Split(filepath.ToSlash(path), "/")
				if len(pathParts) >= 2 {
					// Check if parent directory indicates content type
					parentDir := strings.ToLower(pathParts[len(pathParts)-2])
					if parentDir == "emails" || parentDir == "landing pages" || parentDir == "pages" {
						orgTypes[parentDir] = true
					} else {
						isEmailLandingOrg = false
						break
					}
				} else {
					isEmailLandingOrg = false
					break
				}
			}

			// If this appears to be email/landing page organization with different types, allow it
			if isEmailLandingOrg && len(orgTypes) > 1 {
				continue // Allow this duplicate name
			}

			// Otherwise, it's a real duplicate campaign name conflict
			return DuplicateError{
				Type:      "name",
				Value:     name,
				Campaigns: paths,
			}
		}
	}

	// Note: Folder conflicts are now automatically resolved during export by adding number suffixes
	// No need to block export for folder name conflicts since the data.yaml determines import behavior

	return nil
}

// addPhishingTemplates recursively finds template folders (containing *.html files) and adds them to templates/
func addPhishingTemplates(zipWriter *zip.Writer, baseDir string) error {
	usedNames := make(map[string]bool)

	return filepath.Walk(baseDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip if not a directory
		if !info.IsDir() {
			return nil
		}

		// Skip the assets directories as they're handled separately
		if strings.Contains(path, "Assets") || strings.Contains(path, "assets") {
			return filepath.SkipDir
		}

		// Skip private directories (client-specific content that should not be exported)
		relPath, err := filepath.Rel(baseDir, path)
		if err == nil {
			pathComponents := strings.Split(filepath.ToSlash(relPath), "/")
			if len(pathComponents) > 0 && strings.ToLower(pathComponents[0]) == "private" {
				return filepath.SkipDir
			}
		}

		// Check if this directory contains any HTML files
		hasHTML, err := containsHTMLFiles(path)
		if err != nil {
			return err
		}

		// If this directory contains HTML files, it's a template directory
		if hasHTML {
			templateName := filepath.Base(path)

			// Handle name conflicts by adding a hash suffix
			if usedNames[templateName] {
				// Create a unique hash based on the full path and current time
				hashInput := fmt.Sprintf("%s-%d", path, time.Now().UnixNano())
				hasher := sha256.New()
				hasher.Write([]byte(hashInput))
				hash := hex.EncodeToString(hasher.Sum(nil))[:8] // Use first 8 chars
				templateName = fmt.Sprintf("%s-%s", templateName, hash)
			}
			usedNames[templateName] = true

			return addTemplateToZip(zipWriter, path, templateName)
		}

		return nil
	})
}

// containsHTMLFiles checks if a directory contains any *.html files
func containsHTMLFiles(dirPath string) (bool, error) {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return false, err
	}

	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(strings.ToLower(entry.Name()), ".html") {
			return true, nil
		}
	}
	return false, nil
}

// addTemplateToZip adds an entire template directory to the templates/ folder in the zip
func addTemplateToZip(zipWriter *zip.Writer, templatePath, templateName string) error {
	return filepath.Walk(templatePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Get relative path from template directory
		relPath, err := filepath.Rel(templatePath, path)
		if err != nil {
			return err
		}

		// Skip the root template directory
		if relPath == "." {
			return nil
		}

		// Create the path in templates folder
		zipPath := filepath.Join("templates", templateName, relPath)
		zipPath = filepath.ToSlash(zipPath) // Ensure forward slashes

		// Create zip header
		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}

		header.Name = zipPath
		if info.IsDir() {
			header.Name += "/"
			header.Method = zip.Store
		} else {
			header.Method = zip.Deflate
		}

		// Create writer for the file
		writer, err := zipWriter.CreateHeader(header)
		if err != nil {
			return err
		}

		// If it's a directory, we're done
		if info.IsDir() {
			return nil
		}

		// Open and copy file contents
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()

		_, err = io.Copy(writer, file)
		return err
	})
}

// StructureHandler handles API requests for directory structure
// TODO this looks dangerous! consider using go 1.24's os.Root
func StructureHandler(baseDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Set content type
		w.Header().Set("Content-Type", "application/json")

		// Get requested path from query parameter
		reqPath := r.URL.Query().Get("path")

		// Clean the path to prevent directory traversal
		cleanPath := filepath.Clean(reqPath)
		if strings.Contains(cleanPath, "..") {
			http.Error(w, `{"error":"Invalid path"}`, http.StatusBadRequest)
			return
		}

		// Build the filesystem path
		fsPath := filepath.Join(baseDir, cleanPath)

		// Ensure the path is within baseDir
		absBaseDir, _ := filepath.Abs(baseDir)
		absPath, _ := filepath.Abs(fsPath)
		if !strings.HasPrefix(absPath, absBaseDir) {
			http.Error(w, `{"error":"Invalid path"}`, http.StatusBadRequest)
			return
		}

		// Check if path exists
		_, err := os.Stat(fsPath)
		if os.IsNotExist(err) {
			http.Error(w, `{"error":"Path not found"}`, http.StatusNotFound)
			return
		}
		if err != nil {
			http.Error(w, `{"error":"Error accessing path"}`, http.StatusInternalServerError)
			return
		}

		// Read directory contents
		entries, err := os.ReadDir(fsPath)
		if err != nil {
			http.Error(w, `{"error":"Failed to read directory"}`, http.StatusInternalServerError)
			return
		}

		// Convert to DirectoryItem format
		items := make([]DirectoryItem, 0, len(entries))
		for _, entry := range entries {
			item := DirectoryItem{
				Name:  entry.Name(),
				IsDir: entry.IsDir(),
			}
			items = append(items, item)
		}

		// Return as JSON
		if err := json.NewEncoder(w).Encode(items); err != nil {
			http.Error(w, `{"error":"Failed to encode response"}`, http.StatusInternalServerError)
			return
		}
	}
}
