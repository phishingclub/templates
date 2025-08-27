package handler

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// DirectoryItem represents a single file or directory in the navigation tree
type DirectoryItem struct {
	Name  string `json:"name"`
	IsDir bool   `json:"isDir"`
}

// DownloadHandler creates a zip archive of a directory and sends it to the client
func DownloadHandler(baseDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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
		err := addPhishingTemplates(zipWriter, baseDir)
		if err != nil {
			http.Error(w, fmt.Sprintf(`{"error":"Error processing templates: %s"}`, err), http.StatusInternalServerError)
			return
		}
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

// addPhishingTemplates recursively finds template folders (containing *.html files) and adds them to templates/
func addPhishingTemplates(zipWriter *zip.Writer, baseDir string) error {
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

		// Check if this directory contains any HTML files
		hasHTML, err := containsHTMLFiles(path)
		if err != nil {
			return err
		}

		// If this directory contains HTML files, it's a template directory
		if hasHTML {
			templateName := filepath.Base(path)
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
