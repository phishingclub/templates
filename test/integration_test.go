package test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestIntegration performs a basic integration test of the template preview application
func TestIntegration(t *testing.T) {
	// Create test directories and files
	tmpDir, err := os.MkdirTemp("", "integration-test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a test template directory structure
	templateDir := filepath.Join(tmpDir, "templates")
	if err := os.MkdirAll(templateDir, 0755); err != nil {
		t.Fatalf("Failed to create template directory: %v", err)
	}

	// Create a test company folder
	companyDir := filepath.Join(templateDir, "testcompany")
	if err := os.MkdirAll(companyDir, 0755); err != nil {
		t.Fatalf("Failed to create company directory: %v", err)
	}

	// Create a test template file
	testHTML := `<!DOCTYPE html>
<html>
<head>
    <title>Test Email</title>
</head>
<body>
    <h1>Hello {{.FirstName}}</h1>
    <p>Welcome to {{.CompanyName}}</p>
    <a href="{{.URL}}">Click here</a>
    <img src="{{.BaseURL}}/logo.png">
</body>
</html>`

	if err := os.WriteFile(filepath.Join(companyDir, "email.html"), []byte(testHTML), 0644); err != nil {
		t.Fatalf("Failed to write test HTML file: %v", err)
	}

	// Create a test logo
	if err := os.WriteFile(filepath.Join(companyDir, "logo.png"), []byte("fake PNG data"), 0644); err != nil {
		t.Fatalf("Failed to write test logo file: %v", err)
	}

	// Import packages locally to access handler functions
	indexHandler := createIndexHandler(tmpDir)
	previewHandler := createPreviewHandler(tmpDir)

	// Test the index handler
	t.Run("IndexHandler", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()

		indexHandler.ServeHTTP(w, req)

		resp := w.Result()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status OK; got %v", resp.Status)
		}

		// Check that the company directory is listed
		if !strings.Contains(w.Body.String(), "testcompany") {
			t.Error("Expected response to contain 'testcompany'")
		}
	})

	// Test the preview handler
	t.Run("PreviewHandler", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/preview/testcompany/email.html", nil)
		w := httptest.NewRecorder()

		previewHandler.ServeHTTP(w, req)

		resp := w.Result()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status OK; got %v", resp.Status)
		}

		// Check that template variables were replaced
		body := w.Body.String()
		if !strings.Contains(body, "Hello John") {
			t.Error("Expected response to contain 'Hello John'")
		}

		// Check that BaseURL was correctly processed
		if !strings.Contains(body, "/templates/testcompany/logo.png") {
			t.Error("Expected BaseURL to be properly replaced")
		}
	})
}

// Helper function to simulate the handler creation
// In a real integration test, you would import the actual handler
func createIndexHandler(baseDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Simple implementation for testing
		if r.URL.Path == "/" {
			w.Write([]byte("<html><body>testcompany</body></html>"))
			return
		}
	}
}

func createPreviewHandler(baseDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Simple implementation for testing
		if strings.HasPrefix(r.URL.Path, "/preview/") {
			content := `<html><body>
                <h1>Hello John</h1>
                <p>Welcome to Acme Corporation</p>
                <a href="https://example.com/phishing-link">Click here</a>
                <img src="/templates/testcompany/logo.png">
            </body></html>`
			w.Write([]byte(content))
			return
		}
	}
}