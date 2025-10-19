// Package handler provides HTTP handlers with path traversal protection.
// TODO: Replace validatePath with os.Root when Go 1.24 is available.
package handler

import (
	"bytes"
	"fmt"
	"html"
	"html/template"

	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

// validatePath prevents directory traversal attacks through multiple encoding bypass detection.
// TODO: Replace with os.Root in Go 1.24.
func validatePath(baseDir, reqPath string) (string, error) {
	// Reject absolute paths immediately (Unix-style and Windows-style)
	if filepath.IsAbs(reqPath) {
		return "", fmt.Errorf("absolute paths not allowed")
	}

	// Check for Windows absolute paths that filepath.IsAbs might miss
	if len(reqPath) >= 2 && reqPath[1] == ':' {
		return "", fmt.Errorf("absolute paths not allowed")
	}

	// Check for UNC paths (Windows network paths)
	if strings.HasPrefix(reqPath, "\\\\") || strings.HasPrefix(reqPath, "//") {
		return "", fmt.Errorf("UNC paths not allowed")
	}

	// Decode multiple layers of encoding to prevent bypass attempts
	decodedPath, err := decodeMultipleLayers(reqPath)
	if err != nil {
		return "", fmt.Errorf("invalid encoding in path: %v", err)
	}

	// Normalize Unicode and remove dangerous characters
	normalizedPath := normalizeAndClean(decodedPath)

	// Replace all backslashes with forward slashes for consistent handling
	cleanPath := strings.ReplaceAll(normalizedPath, "\\", "/")

	// Remove any leading slashes to ensure it's treated as relative
	cleanPath = strings.TrimLeft(cleanPath, "/")

	// Check for directory traversal patterns before and after normalization
	if containsTraversalPattern(cleanPath) {
		return "", fmt.Errorf("path traversal attempt detected")
	}

	// Clean the path to resolve any "." elements and normalize separators
	cleanPath = filepath.Clean(cleanPath)

	// Final check: ensure no traversal patterns remain after cleaning
	if containsTraversalPattern(cleanPath) {
		return "", fmt.Errorf("path traversal attempt detected")
	}

	// Additional check for empty or suspicious paths
	if cleanPath == "" || cleanPath == "." {
		cleanPath = ""
	}

	// Join with base directory
	fullPath := filepath.Join(baseDir, cleanPath)

	// Get absolute paths for final verification
	absBasePath, err := filepath.Abs(baseDir)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute base path: %v", err)
	}

	absFullPath, err := filepath.Abs(fullPath)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute full path: %v", err)
	}

	// Ensure the resolved path is still within the base directory
	// Must either be exactly the base or start with base + separator
	if absFullPath != absBasePath && !strings.HasPrefix(absFullPath, absBasePath+string(filepath.Separator)) {
		return "", fmt.Errorf("path escapes base directory")
	}

	return absFullPath, nil
}

// TODO: Go 1.24 replacement with os.Root:
// func validatePathWithOSRoot(baseDir, reqPath string) (string, error) {
//     root := os.Root(baseDir)
//     _, err := root.Stat(filepath.Clean(reqPath))
//     if err != nil && !os.IsNotExist(err) {
//         return "", fmt.Errorf("invalid path: %v", err)
//     }
//     return filepath.Join(baseDir, reqPath), nil
// }

// decodeMultipleLayers performs multiple rounds of decoding to handle nested encoding
func decodeMultipleLayers(input string) (string, error) {
	current := input
	maxIterations := 5 // Prevent infinite loops

	for i := 0; i < maxIterations; i++ {
		previous := current

		// URL decode
		if decoded, err := url.QueryUnescape(current); err == nil {
			current = decoded
		}

		// HTML entity decode
		current = html.UnescapeString(current)

		// If no changes were made, we're done
		if current == previous {
			break
		}
	}

	return current, nil
}

// normalizeAndClean removes dangerous characters and normalizes Unicode
func normalizeAndClean(input string) string {
	// First, handle UTF-8 overlong sequences and malformed UTF-8
	cleanBytes := sanitizeUTF8([]byte(input))
	sanitized := string(cleanBytes)

	var result strings.Builder

	for _, r := range sanitized {
		// Remove null bytes and other control characters (except common ones)
		if r == 0 || (unicode.IsControl(r) && r != '\t' && r != '\n' && r != '\r') {
			continue
		}

		// Normalize some Unicode characters that could be used for bypass
		switch r {
		case '\u2215': // Division slash
			result.WriteRune('/')
		case '\u2044': // Fraction slash
			result.WriteRune('/')
		case '\uFF0E': // Fullwidth full stop
			result.WriteRune('.')
		case '\uFF0F': // Fullwidth solidus
			result.WriteRune('/')
		default:
			result.WriteRune(r)
		}
	}

	return result.String()
}

// sanitizeUTF8 removes invalid UTF-8 sequences and overlong encodings
func sanitizeUTF8(input []byte) []byte {
	var result []byte

	for i := 0; i < len(input); {
		// Check for overlong UTF-8 sequences that encode ASCII characters
		if i+1 < len(input) {
			// Check for overlong encoding of '.' (0x2E)
			if input[i] == 0xC0 && input[i+1] == 0xAE {
				result = append(result, '.')
				i += 2
				continue
			}
			// Check for overlong encoding of '/' (0x2F)
			if input[i] == 0xC0 && input[i+1] == 0xAF {
				result = append(result, '/')
				i += 2
				continue
			}
		}

		// For valid UTF-8, just copy the byte
		if input[i] < 0x80 || isValidUTF8Start(input[i:]) {
			result = append(result, input[i])
		}
		// Skip invalid bytes
		i++
	}

	return result
}

// isValidUTF8Start checks if the byte sequence starts a valid UTF-8 character
func isValidUTF8Start(bytes []byte) bool {
	if len(bytes) == 0 {
		return false
	}

	b := bytes[0]

	// ASCII
	if b < 0x80 {
		return true
	}

	// Invalid start bytes
	if b < 0xC2 || b > 0xF4 {
		return false
	}

	// Check continuation bytes
	if b < 0xE0 { // 2-byte sequence
		return len(bytes) >= 2 && (bytes[1]&0xC0) == 0x80
	}
	if b < 0xF0 { // 3-byte sequence
		return len(bytes) >= 3 && (bytes[1]&0xC0) == 0x80 && (bytes[2]&0xC0) == 0x80
	}
	// 4-byte sequence
	return len(bytes) >= 4 && (bytes[1]&0xC0) == 0x80 && (bytes[2]&0xC0) == 0x80 && (bytes[3]&0xC0) == 0x80
}

// containsTraversalPattern checks for various directory traversal patterns
func containsTraversalPattern(path string) bool {
	// Direct check for ".."
	if strings.Contains(path, "..") {
		return true
	}

	// Check for spaced or padded versions
	patterns := []string{
		". .",   // space between dots
		".\t.",  // tab between dots
		". ./",  // space after first dot
		".\t./", // tab after first dot
		" ../",  // leading space
		"\t../", // leading tab
	}

	for _, pattern := range patterns {
		if strings.Contains(path, pattern) {
			return true
		}
	}

	// Check for encoded versions that might have been missed
	encodedPatterns := []string{
		"%2e%2e",       // URL encoded ..
		"&#46;&#46;",   // HTML entity encoded ..
		"&#x2e;&#x2e;", // HTML hex entity encoded ..
	}

	for _, pattern := range encodedPatterns {
		if strings.Contains(strings.ToLower(path), strings.ToLower(pattern)) {
			return true
		}
	}

	return false
}

// Template data model for directory listing view
type DirListData struct {
	Path         string
	Dirs         []DirEntry
	Files        []DirEntry
	IsRoot       bool
	ParentPath   string
	NavTree      []NavTreeItem
	CurrentPath  string
	ExpandedDirs map[string]bool
}

// NavTreeItem represents an item in the navigation tree
type NavTreeItem struct {
	Name     string
	Path     string
	IsDir    bool
	IsOpen   bool
	Level    int
	Children []NavTreeItem
}

// Entry for files and directories in the listing view
type DirEntry struct {
	Name  string
	Path  string
	IsDir bool
}

// Template data model for preview view
type PreviewData struct {
	Path         string
	Filename     string
	Content      template.HTML
	ParentPath   string
	NavTree      []NavTreeItem
	CurrentPath  string
	ExpandedDirs map[string]bool
}

// Map of template variables with default values for preview
var templateVars = map[string]string{
	// Recipient fields
	"{{.rID}}":             "1234567890",
	"{{.FirstName}}":       "John",
	"{{.LastName}}":        "Doe",
	"{{.Email}}":           "john.doe@example.com",
	"{{.To}}":              "john.doe@example.com", // alias of Email
	"{{.Phone}}":           "+1-555-123-4567",
	"{{.ExtraIdentifier}}": "EMP001",
	"{{.Position}}":        "IT Manager",
	"{{.Department}}":      "Information Technology",
	"{{.City}}":            "New York",
	"{{.Country}}":         "United States",
	"{{.Misc}}":            "Additional Info",

	// Tracking fields
	"{{.Tracker}}":     `<img src="https://phishing.test/opened/unique-id" alt="" width="1" height="1" border="0" style="height:1px !important;width:1px" />`,
	"{{.TrackingURL}}": "https://phishing.test/clicked/unique-id",

	// Sender fields
	"{{.From}}": "Security Team <security@phishing.test>",

	// General fields
	"{{.BaseURL}}": "https://phishing.test",
	"{{.URL}}":     "https://phishing.test/phishing-link",
	"{{.DenyURL}}": "https://phishing.test/access-denied",

	// API sender fields
	"{{.APIKey}}":       "",
	"{{.CustomField1}}": "",
	"{{.CustomField2}}": "",
	"{{.CustomField3}}": "",
	"{{.CustomField4}}": "",

	// Legacy/additional fields for compatibility
}

// IndexHandler renders the directory listing view
func IndexHandler(baseDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Parse the base template
		tmpl, err := template.New("layout.html").Funcs(TemplateFuncs).ParseFiles("views/layout.html", "views/listing.html", "views/nav_tree.html")
		if err != nil {
			http.Error(w, "Failed to load templates: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// Get requested path
		reqPath := strings.TrimPrefix(r.URL.Path, "/")

		// No need to get from cookie anymore - just generate based on current path
		expandedDirsFromCookie := make(map[string]bool)

		// No toggle action needed - we'll just expand the current path

		// Validate and build the filesystem path
		fsPath, err := validatePath(baseDir, reqPath)
		if err != nil {
			http.Error(w, "Invalid path", http.StatusBadRequest)
			return
		}

		// Check if path exists
		info, err := os.Stat(fsPath)
		if os.IsNotExist(err) {
			http.Error(w, "Path not found", http.StatusNotFound)
			return
		}
		if err != nil {
			http.Error(w, "Error accessing path: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// If this is a file, redirect to the preview handler
		if !info.IsDir() {
			previewPath := "/preview" + reqPath
			http.Redirect(w, r, previewPath, http.StatusFound)
			return
		}

		// Get directory contents
		files, err := os.ReadDir(fsPath)
		if err != nil {
			http.Error(w, "Failed to read directory: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// Build view data
		data := DirListData{
			Path:         reqPath,
			IsRoot:       reqPath == "",
			CurrentPath:  reqPath,
			ExpandedDirs: expandedDirsFromCookie,
		}

		// Set parent path
		if data.IsRoot {
			data.ParentPath = "/"
		} else {
			parentPath := filepath.Dir(reqPath)
			if parentPath == "." {
				data.ParentPath = "/"
			} else {
				data.ParentPath = "/" + parentPath
			}
		}

		// Group entries into directories and files
		for _, file := range files {
			var entryPath string
			if reqPath == "" {
				entryPath = file.Name()
			} else {
				entryPath = filepath.ToSlash(filepath.Join(reqPath, file.Name()))
			}

			entry := DirEntry{
				Name:  file.Name(),
				Path:  entryPath,
				IsDir: file.IsDir(),
			}

			if file.IsDir() {
				data.Dirs = append(data.Dirs, entry)
			} else {
				// Include HTML files in the listing for preview
				// Other files will be included but handled differently in the template
				data.Files = append(data.Files, entry)
			}
		}

		// Sort directories and files using natural sorting
		sort.Slice(data.Dirs, func(i, j int) bool {
			return NaturalSort(data.Dirs[i].Name, data.Dirs[j].Name)
		})
		sort.Slice(data.Files, func(i, j int) bool {
			return NaturalSort(data.Files[i].Name, data.Files[j].Name)
		})

		// Build navigation tree
		navTree, expandedDirs := buildNavigationTree(baseDir, reqPath)

		// Merge cookie expanded dirs with the calculated ones
		for dir := range expandedDirsFromCookie {
			expandedDirs[dir] = true
		}

		data.NavTree = navTree
		data.ExpandedDirs = expandedDirs

		// Render template
		if err := tmpl.ExecuteTemplate(w, "layout", data); err != nil {
			http.Error(w, "Failed to render template: "+err.Error(), http.StatusInternalServerError)
			return
		}
	}
}

// buildNavigationTree creates a hierarchical tree structure for the sidebar navigation
func buildNavigationTree(baseDir, currentPath string) ([]NavTreeItem, map[string]bool) {
	// Create a map to track expanded directories
	expandedDirs := make(map[string]bool)

	// Always expand root
	expandedDirs[""] = true

	// Mark all parent directories of the current path as expanded
	pathParts := strings.Split(currentPath, "/")
	currentPathSoFar := ""
	for _, part := range pathParts {
		if part != "" {
			if currentPathSoFar != "" {
				currentPathSoFar += "/"
			}
			currentPathSoFar += part
			expandedDirs[currentPathSoFar] = true
		}
	}

	// Read the root directory
	rootEntries, err := os.ReadDir(baseDir)
	if err != nil {
		return nil, expandedDirs
	}

	// Build the root level of the tree
	rootItems := make([]NavTreeItem, 0)
	for _, entry := range rootEntries {
		if entry.IsDir() {
			// Create root level directory item
			item := NavTreeItem{
				Name:   entry.Name(),
				Path:   entry.Name(),
				IsDir:  true,
				IsOpen: expandedDirs[entry.Name()],
				Level:  0,
			}

			// If this directory should be expanded, add its children
			if expandedDirs[entry.Name()] {
				item.Children = getDirectoryChildren(filepath.Join(baseDir, entry.Name()), entry.Name(), expandedDirs, 1)
			}

			rootItems = append(rootItems, item)
		}
	}

	// Sort directories first, then by name using natural sorting
	sort.Slice(rootItems, func(i, j int) bool {
		return NaturalSort(rootItems[i].Name, rootItems[j].Name)
	})

	return rootItems, expandedDirs
}

// NaturalSort compares two strings using natural sorting (handles numbers correctly)
func NaturalSort(a, b string) bool {
	// Split strings into parts (text and numbers)
	aParts := splitNatural(a)
	bParts := splitNatural(b)

	minLen := len(aParts)
	if len(bParts) < minLen {
		minLen = len(bParts)
	}

	for i := 0; i < minLen; i++ {
		aIsNum := isNumber(aParts[i])
		bIsNum := isNumber(bParts[i])

		// If both are numbers, compare numerically
		if aIsNum && bIsNum {
			aNum, _ := strconv.Atoi(aParts[i])
			bNum, _ := strconv.Atoi(bParts[i])
			if aNum != bNum {
				return aNum < bNum
			}
		} else if aIsNum != bIsNum {
			// Numbers come before text
			return aIsNum
		} else {
			// Both are text, compare lexicographically
			if aParts[i] != bParts[i] {
				return aParts[i] < bParts[i]
			}
		}
	}

	// If all compared parts are equal, shorter string comes first
	return len(aParts) < len(bParts)
}

// splitNatural splits a string into alternating text and number parts
func splitNatural(s string) []string {
	var parts []string
	var current strings.Builder
	var lastWasDigit bool

	for i, r := range s {
		isDigit := r >= '0' && r <= '9'

		if i > 0 && isDigit != lastWasDigit {
			parts = append(parts, current.String())
			current.Reset()
		}

		current.WriteRune(r)
		lastWasDigit = isDigit
	}

	if current.Len() > 0 {
		parts = append(parts, current.String())
	}

	return parts
}

// isNumber checks if a string represents a number
func isNumber(s string) bool {
	_, err := strconv.Atoi(s)
	return err == nil
}

// getDirectoryChildren reads a directory and returns its children as NavTreeItems
func getDirectoryChildren(dirPath, relativePath string, expandedDirs map[string]bool, level int) []NavTreeItem {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil
	}

	children := make([]NavTreeItem, 0)
	for _, entry := range entries {
		childRelPath := filepath.Join(relativePath, entry.Name())
		childRelPath = filepath.ToSlash(childRelPath) // Ensure consistent path format

		item := NavTreeItem{
			Name:   entry.Name(),
			Path:   childRelPath,
			IsDir:  entry.IsDir(),
			IsOpen: expandedDirs[childRelPath],
			Level:  level,
		}

		// If this is a directory and it's expanded, add its children
		if entry.IsDir() && expandedDirs[childRelPath] {
			item.Children = getDirectoryChildren(
				filepath.Join(dirPath, entry.Name()),
				childRelPath,
				expandedDirs,
				level+1,
			)
		}

		children = append(children, item)
	}

	// Sort children by directories first, then by name using natural sorting
	sort.Slice(children, func(i, j int) bool {
		if children[i].IsDir != children[j].IsDir {
			return children[i].IsDir
		}
		return NaturalSort(children[i].Name, children[j].Name)
	})

	return children
}

// OriginalContentHandler serves the raw template content without any processing
func OriginalContentHandler(baseDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Get requested path
		reqPath := strings.TrimPrefix(r.URL.Path, "/original/")
		if reqPath == "" {
			http.Error(w, "No template specified", http.StatusBadRequest)
			return
		}

		// Validate and build the filesystem path
		fsPath, err := validatePath(baseDir, reqPath)
		if err != nil {
			http.Error(w, "Invalid path", http.StatusBadRequest)
			return
		}

		// Check if file exists
		info, err := os.Stat(fsPath)
		if os.IsNotExist(err) {
			http.Error(w, "Template not found", http.StatusNotFound)
			return
		}
		if err != nil {
			http.Error(w, "Error accessing template: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// If this is a directory, return error
		if info.IsDir() {
			http.Error(w, "Cannot view directory content", http.StatusBadRequest)
			return
		}

		// Read file content without any processing
		content, err := os.ReadFile(fsPath)
		if err != nil {
			http.Error(w, "Failed to read template: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// Set content type based on file extension
		ext := strings.ToLower(filepath.Ext(fsPath))
		switch ext {
		case ".html":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
		case ".txt":
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		default:
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		}

		// Serve the unprocessed content directly
		w.Write(content)
	}
}

// RawViewHandler serves the template content directly without wrapping it in the UI
func RawViewHandler(baseDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Get requested path
		reqPath := strings.TrimPrefix(r.URL.Path, "/raw/")
		if reqPath == "" {
			http.Error(w, "No template specified", http.StatusBadRequest)
			return
		}

		// Validate and build the filesystem path
		fsPath, err := validatePath(baseDir, reqPath)
		if err != nil {
			http.Error(w, "Invalid path", http.StatusBadRequest)
			return
		}

		// Check if file exists
		info, err := os.Stat(fsPath)
		if os.IsNotExist(err) {
			http.Error(w, "Template not found", http.StatusNotFound)
			return
		}
		if err != nil {
			http.Error(w, "Error accessing template: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// If this is a directory, redirect to the index handler
		if info.IsDir() {
			http.Redirect(w, r, "/"+reqPath, http.StatusFound)
			return
		}

		// For HTML files, process the template content before serving
		if filepath.Ext(fsPath) == ".html" || filepath.Ext(fsPath) == ".yaml" {
			// Read file content
			content, err := os.ReadFile(fsPath)
			if err != nil {
				http.Error(w, "Failed to read template: "+err.Error(), http.StatusInternalServerError)
				return
			}

			// Process the template content (replacing variables)
			processedContent := processTemplateContent(string(content), reqPath, baseDir)

			// Set content type and serve
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Write([]byte(processedContent))
			return
		}

		// For non-HTML files, serve directly
		http.ServeFile(w, r, fsPath)
	}
}

// PreviewHandler renders the template preview
func PreviewHandler(baseDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Parse the base template
		tmpl, err := template.New("layout.html").Funcs(TemplateFuncs).ParseFiles("views/layout.html", "views/preview.html", "views/nav_tree.html")
		if err != nil {
			http.Error(w, "Failed to load templates: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// Get requested path
		reqPath := strings.TrimPrefix(r.URL.Path, "/preview/")
		if reqPath == "" {
			http.Error(w, "No template specified", http.StatusBadRequest)
			return
		}

		// No toggle functionality in preview handler

		// Validate and build the filesystem path
		fsPath, err := validatePath(baseDir, reqPath)
		if err != nil {
			http.Error(w, "Invalid path", http.StatusBadRequest)
			return
		}

		// Check if file exists
		info, err := os.Stat(fsPath)
		if os.IsNotExist(err) {
			http.Error(w, "Template not found", http.StatusNotFound)
			return
		}
		if err != nil {
			http.Error(w, "Error accessing template: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// If this is a directory, redirect to the index handler
		if info.IsDir() {
			http.Redirect(w, r, fmt.Sprintf("/%s", reqPath), http.StatusFound)
			return
		}

		// For non-HTML files, serve them directly
		if filepath.Ext(fsPath) != ".html" {
			// Set appropriate content type based on file extension
			ext := filepath.Ext(fsPath)
			var contentType string
			switch strings.ToLower(ext) {
			case ".jpg", ".jpeg":
				contentType = "image/jpeg"
			case ".png":
				contentType = "image/png"
			case ".gif":
				contentType = "image/gif"
			case ".svg":
				contentType = "image/svg+xml"
			case ".css":
				contentType = "text/css"
			case ".js":
				contentType = "application/javascript"
			case ".pdf":
				contentType = "application/pdf"
			case ".txt":
				contentType = "text/plain"
			case ".yaml":
				contentType = "text/plain"
			default:
				contentType = "application/octet-stream"
			}

			// Read and serve the file
			fileData, err := os.ReadFile(fsPath)
			if err != nil {
				http.Error(w, "Failed to read file: "+err.Error(), http.StatusInternalServerError)
				return
			}

			w.Header().Set("Content-Type", contentType)
			w.Write(fileData)
			return
		}

		// Read file content
		content, err := os.ReadFile(fsPath)
		if err != nil {
			http.Error(w, "Failed to read template: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// Process the template content
		processedContent := processTemplateContent(string(content), reqPath, baseDir)

		// Build view data
		parentDir := filepath.Dir(reqPath)
		var parentPath string
		if parentDir == "." {
			parentPath = "/"
		} else {
			parentPath = "/" + parentDir
		}

		// No need to get expanded dirs from cookie in preview handler
		expandedDirsFromCookie := make(map[string]bool)

		// Build navigation tree for the sidebar
		navTree, expandedDirs := buildNavigationTree(baseDir, parentDir)

		// Merge cookie expanded dirs with the calculated ones
		for dir := range expandedDirsFromCookie {
			expandedDirs[dir] = true
		}

		data := PreviewData{
			Path:         reqPath,
			Filename:     filepath.Base(reqPath),
			Content:      template.HTML(processedContent),
			ParentPath:   parentPath,
			NavTree:      navTree,
			CurrentPath:  parentDir,
			ExpandedDirs: expandedDirs,
		}

		// Render template
		if err := tmpl.ExecuteTemplate(w, "layout", data); err != nil {
			http.Error(w, "Failed to render template: "+err.Error(), http.StatusInternalServerError)
			return
		}
	}
}

// Process template content by replacing GoPhish template variables
func processTemplateContent(content, reqPath, baseDir string) string {
	// Process BaseURL specially to make it relative to the current template
	dirPath := filepath.ToSlash(filepath.Dir(reqPath))

	// Handle special case for root directory
	if dirPath == "." {
		dirPath = ""
	}

	baseURL := fmt.Sprintf("/templates/%s", dirPath)

	// Clean the URL to remove any duplicate slashes or unnecessary path elements
	baseURL = strings.TrimRight(baseURL, "/")

	// Create template data with all variables and BaseURL
	templateData := make(map[string]any)

	// Add all template variables to data (removing the {{. }} wrapper)
	for placeholder, value := range templateVars {
		// Extract variable name from {{.VarName}} format
		if strings.HasPrefix(placeholder, "{{.") && strings.HasSuffix(placeholder, "}}") {
			varName := strings.TrimPrefix(strings.TrimSuffix(placeholder, "}}"), "{{.")
			// Special handling for Tracker - it should be rendered as unescaped HTML
			if varName == "Tracker" {
				templateData[varName] = template.HTML(value)
			} else {
				templateData[varName] = value
			}
		}
	}

	// Add BaseURL to template data (override the static value with computed path)
	templateData["BaseURL"] = baseURL

	// Always process templates
	tmpl, err := template.New("content").Funcs(TemplateFuncs).Parse(content)
	if err != nil {
		// If template parsing fails, fall back to string replacement
		content = strings.Replace(content, "{{.BaseURL}}", baseURL, -1)
		for placeholder, value := range templateVars {
			content = strings.Replace(content, placeholder, value, -1)
		}
		return processAssetPaths(content, reqPath, baseDir)
	}

	var buf bytes.Buffer
	err = tmpl.Execute(&buf, templateData)
	if err != nil {
		// If template execution fails, fall back to string replacement
		content = strings.Replace(content, "{{.BaseURL}}", baseURL, -1)
		for placeholder, value := range templateVars {
			content = strings.Replace(content, placeholder, value, -1)
		}
		return processAssetPaths(content, reqPath, baseDir)
	}

	content = buf.String()

	return processAssetPaths(content, reqPath, baseDir)
}

// processAssetPaths handles asset path processing for template content
func processAssetPaths(content, reqPath, baseDir string) string {
	// Fix any double slashes in paths (except for http:// or https://)
	content = strings.Replace(content, "src=\"//", "src=\"/", -1)
	content = strings.Replace(content, "href=\"//", "href=\"/", -1)

	// Fix any occurrences of double slashes in URLs within HTML attributes only
	// This avoids corrupting JavaScript comments like "// comment"
	// Use a function to safely replace double slashes while preserving protocols
	content = regexp.MustCompile(`((?:src|href|action)=["']([^"']*?)["'])`).ReplaceAllStringFunc(content, func(match string) string {
		// Don't modify URLs that start with http:// or https://
		if strings.Contains(match, "http://") || strings.Contains(match, "https://") {
			return match
		}
		// Replace multiple slashes with single slash for non-protocol URLs
		return regexp.MustCompile(`//+`).ReplaceAllString(match, "/")
	})

	// Find and replace any img/script/link/a tags that reference relative paths
	templateDir := filepath.Dir(reqPath)

	// Process src attributes with asset fallback logic
	srcRegex := regexp.MustCompile(`(src|href)=["']([^"']+)["']`)
	content = srcRegex.ReplaceAllStringFunc(content, func(match string) string {
		parts := srcRegex.FindStringSubmatch(match)
		attr := parts[1]
		path := parts[2]

		// Skip absolute URLs and data URLs
		if strings.HasPrefix(path, "http://") ||
			strings.HasPrefix(path, "https://") ||
			strings.HasPrefix(path, "//") ||
			strings.HasPrefix(path, "data:") ||
			strings.HasPrefix(path, "#") {
			return match
		}

		// Handle paths that start with /templates/ (already processed BaseURL paths)
		if strings.HasPrefix(path, "/templates/") {
			// Check if the file actually exists at this path
			relativePath := strings.TrimPrefix(path, "/templates/")
			fullPath := filepath.Join(baseDir, relativePath)

			if _, err := os.Stat(fullPath); err == nil {
				// File exists at the current path, keep it
				return match
			}

			// File doesn't exist, try to extract the asset part and check in global assets
			templatePrefix := templateDir + "/"
			if strings.HasPrefix(relativePath, templatePrefix) {
				assetPath := strings.TrimPrefix(relativePath, templatePrefix)

				// Check if the assetPath starts with "assets/" - if so, remove it
				// This handles templates that use {{.BaseURL}}/assets/... pattern
				if strings.HasPrefix(assetPath, "assets/") {
					assetPath = strings.TrimPrefix(assetPath, "assets/")
				}

				// Try global assets directory
				globalAssetsPath := filepath.Join(baseDir, "assets", assetPath)
				if _, err := os.Stat(globalAssetsPath); err == nil {
					// File exists in global assets directory
					newPath := filepath.ToSlash(filepath.Join("/templates/assets", assetPath))
					newPath = filepath.Clean(newPath)
					return fmt.Sprintf(`%s="%s"`, attr, newPath)
				}
			}

			// Return original if no fallback found
			return match
		}

		// Handle relative paths (don't start with /)
		// Try local template directory first
		localPath := filepath.Join(baseDir, templateDir, path)
		if _, err := os.Stat(localPath); err == nil {
			// File exists in template directory, use it
			newPath := filepath.ToSlash(filepath.Join("/templates", templateDir, path))
			newPath = filepath.Clean(newPath)
			if !strings.HasPrefix(newPath, "/templates") {
				newPath = "/templates/" + strings.TrimPrefix(newPath, "/")
			}
			return fmt.Sprintf(`%s="%s"`, attr, newPath)
		}

		// Try global assets directory as fallback
		globalAssetsPath := filepath.Join(baseDir, "assets", path)
		if _, err := os.Stat(globalAssetsPath); err == nil {
			// File exists in global assets directory
			newPath := filepath.ToSlash(filepath.Join("/templates/assets", path))
			newPath = filepath.Clean(newPath)
			return fmt.Sprintf(`%s="%s"`, attr, newPath)
		}

		// File doesn't exist in either location, use template directory path for compatibility
		newPath := filepath.ToSlash(filepath.Join("/templates", templateDir, path))
		newPath = filepath.Clean(newPath)
		if !strings.HasPrefix(newPath, "/templates") {
			newPath = "/templates/" + strings.TrimPrefix(newPath, "/")
		}
		return fmt.Sprintf(`%s="%s"`, attr, newPath)
	})

	return content
}
