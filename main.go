package main

import (
	"archive/zip"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/phishingclub/templates/internal/handler"
)

func main() {
	port := flag.Int("port", 8080, "Port to run the server on")
	templatesDir := flag.String("templates", "./phishing-templates", "Directory containing templates")
	export := flag.Bool("export", false, "Export templates and assets to zip file and exit")
	flag.Parse()

	// Ensure templates directory exists
	if _, err := os.Stat(*templatesDir); os.IsNotExist(err) {
		log.Printf("Templates directory %s does not exist. Creating...", *templatesDir)
		if err := os.MkdirAll(*templatesDir, 0755); err != nil {
			log.Fatalf("Failed to create templates directory: %v", err)
		}
	}

	// Create absolute path for templates directory
	absPath, err := filepath.Abs(*templatesDir)
	if err != nil {
		log.Fatalf("Failed to get absolute path: %v", err)
	}

	// Handle export mode
	if *export {
		err := performExport(absPath)
		if err != nil {
			log.Fatalf("Export failed: %v", err)
		}
		log.Println("Export completed successfully")
		os.Exit(0)
	}

	// Setup router
	mux := http.NewServeMux()

	// Set up static file server for UI assets
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	// Set up custom asset handler for template assets with fallback support
	mux.Handle(
		"/templates/",
		http.StripPrefix("/templates/", handler.AssetHandler(absPath)),
	)

	// Handle template preview
	mux.HandleFunc("/preview/", handler.PreviewHandler(absPath))

	// Handle directory listings
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if !strings.HasPrefix(path, "/") {
			path = "/" + path
		}
		path = strings.TrimLeft(path, "/")
		r.URL.Path = "/" + path

		// Check for toggle parameter in query to expand/collapse folders
		toggleDir := r.URL.Query().Get("toggle")
		if toggleDir != "" {
			// Toggle state in session or redirect to handle via handler
			handler.IndexHandler(absPath)(w, r)
			return
		}

		handler.IndexHandler(absPath)(w, r)
	})

	// API endpoints
	mux.HandleFunc("/api/structure", handler.StructureHandler(absPath))
	mux.HandleFunc("/api/download", handler.DownloadHandler(absPath))
	mux.HandleFunc("/api/export", handler.ExportHandler(absPath))

	// Raw template view handler
	mux.HandleFunc("/raw/", handler.RawViewHandler(absPath))

	// Original unprocessed template content handler
	mux.HandleFunc("/original/", handler.OriginalContentHandler(absPath))

	// Start the server with our custom handler
	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", *port),
		Handler: mux,
	}

	log.Printf("Starting server on http://localhost%s", server.Addr)
	log.Printf("Serving templates from: %s", absPath)
	log.Fatal(server.ListenAndServe())
}

// performExport handles command line export functionality
func performExport(templatesDir string) error {
	timestamp := time.Now().Format("20060102-150405")
	zipFilename := fmt.Sprintf("export-%s.zip", timestamp)

	// Create output file
	outputFile, err := os.Create(zipFilename)
	if err != nil {
		return fmt.Errorf("failed to create output file: %v", err)
	}
	defer outputFile.Close()

	// Create zip writer
	zipWriter := zip.NewWriter(outputFile)
	defer zipWriter.Close()

	// Process branding assets (check both "assets" and "Branding" directories)
	assetsPath := filepath.Join(templatesDir, "assets")
	if _, err := os.Stat(assetsPath); err == nil {
		err = addBrandingAssets(zipWriter, assetsPath)
		if err != nil {
			return fmt.Errorf("error processing assets: %v", err)
		}
	}

	// Also check for legacy "Branding" directory
	brandingPath := filepath.Join(templatesDir, "Branding")
	if _, err := os.Stat(brandingPath); err == nil {
		err = addBrandingAssets(zipWriter, brandingPath)
		if err != nil {
			return fmt.Errorf("error processing branding assets: %v", err)
		}
	}

	// Process phishing templates
	err = addPhishingTemplates(zipWriter, templatesDir)
	if err != nil {
		return fmt.Errorf("error processing templates: %v", err)
	}

	log.Printf("Export saved as: %s", zipFilename)
	return nil
}

// addBrandingAssets adds all folders from Branding to assets/ in the zip
func addBrandingAssets(zipWriter *zip.Writer, brandingPath string) error {
	return filepath.Walk(brandingPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Get relative path from branding directory
		relPath, err := filepath.Rel(brandingPath, path)
		if err != nil {
			return err
		}

		// Skip the root branding directory
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
		if strings.Contains(path, "assets") {
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
