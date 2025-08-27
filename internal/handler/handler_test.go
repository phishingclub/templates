package handler

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIndexHandler(t *testing.T) {
	// Create temp directory for test templates
	tmpDir := createTestTemplateDir(t)
	defer os.RemoveAll(tmpDir)

	// Create views directory for templates
	viewsDir := filepath.Join(tmpDir, "views")
	if err := os.MkdirAll(viewsDir, 0755); err != nil {
		t.Fatalf("Failed to create views dir: %v", err)
	}

	// Create mock layout template
	layoutHTML := `{{define "layout"}}{{template "content" .}}{{end}}`
	if err := os.WriteFile(filepath.Join(viewsDir, "layout.html"), []byte(layoutHTML), 0644); err != nil {
		t.Fatalf("Failed to create layout template: %v", err)
	}

	// Create mock listing template
	listingHTML := `{{define "content"}}<ul>{{range .Dirs}}<li>{{.Name}}</li>{{end}}{{range .Files}}<li>{{.Name}}</li>{{end}}</ul>{{end}}`
	if err := os.WriteFile(filepath.Join(viewsDir, "listing.html"), []byte(listingHTML), 0644); err != nil {
		t.Fatalf("Failed to create listing template: %v", err)
	}

	// Create mock nav_tree template
	navTreeHTML := `{{define "nav_tree"}}{{end}}`
	if err := os.WriteFile(filepath.Join(viewsDir, "nav_tree.html"), []byte(navTreeHTML), 0644); err != nil {
		t.Fatalf("Failed to create nav_tree template: %v", err)
	}

	// Create test handler with local working directory
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}
	defer os.Chdir(oldWd)

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Failed to change working directory: %v", err)
	}

	handler := IndexHandler(tmpDir)

	tests := []struct {
		name             string
		path             string
		expectedStatus   int
		expectedContains string
	}{
		{
			name:             "Root directory",
			path:             "/",
			expectedStatus:   http.StatusOK,
			expectedContains: "test-dir",
		},
		{
			name:             "Subdirectory",
			path:             "/test-dir",
			expectedStatus:   http.StatusOK,
			expectedContains: "test.html",
		},
		{
			name:           "Non-existent path",
			path:           "/not-exist",
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "HTML file should redirect to preview",
			path:           "/test-dir/test.html",
			expectedStatus: http.StatusFound, // 302 redirect
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := http.NewRequest("GET", tt.path, nil)
			if err != nil {
				t.Fatal(err)
			}

			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			if status := rr.Code; status != tt.expectedStatus {
				t.Errorf("handler returned wrong status code: got %v want %v",
					status, tt.expectedStatus)
			}

			// Check contents if expected
			if tt.expectedStatus == http.StatusOK && tt.expectedContains != "" {
				if !strings.Contains(rr.Body.String(), tt.expectedContains) {
					t.Errorf("handler response doesn't contain %q", tt.expectedContains)
				}
			}

			// Check redirect location for files
			if tt.expectedStatus == http.StatusFound {
				location := rr.Header().Get("Location")
				if !strings.HasPrefix(location, "/preview") {
					t.Errorf("redirect location incorrect: %q", location)
				}
			}
		})
	}
}

func TestPreviewHandler(t *testing.T) {
	// Create temp directory for test templates
	tmpDir := createTestTemplateDir(t)
	defer os.RemoveAll(tmpDir)

	// Create views directory for templates
	viewsDir := filepath.Join(tmpDir, "views")
	if err := os.MkdirAll(viewsDir, 0755); err != nil {
		t.Fatalf("Failed to create views dir: %v", err)
	}

	// Create mock layout template
	layoutHTML := `{{define "layout"}}{{template "content" .}}{{end}}`
	if err := os.WriteFile(filepath.Join(viewsDir, "layout.html"), []byte(layoutHTML), 0644); err != nil {
		t.Fatalf("Failed to create layout template: %v", err)
	}

	// Create mock preview template
	previewHTML := `{{define "content"}}{{.Content}}{{end}}`
	if err := os.WriteFile(filepath.Join(viewsDir, "preview.html"), []byte(previewHTML), 0644); err != nil {
		t.Fatalf("Failed to create preview template: %v", err)
	}

	// Create mock nav_tree template
	navTreeHTML := `{{define "nav_tree"}}{{end}}`
	if err := os.WriteFile(filepath.Join(viewsDir, "nav_tree.html"), []byte(navTreeHTML), 0644); err != nil {
		t.Fatalf("Failed to create nav_tree template: %v", err)
	}

	// Create test handler with local working directory
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}
	defer os.Chdir(oldWd)

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Failed to change working directory: %v", err)
	}

	handler := PreviewHandler(tmpDir)

	tests := []struct {
		name             string
		path             string
		expectedStatus   int
		expectedContains []string
	}{
		{
			name:             "Preview HTML file",
			path:             "/preview/test-dir/test.html",
			expectedStatus:   http.StatusOK,
			expectedContains: []string{"Test template", "example.com"},
		},
		{
			name:           "Non-existent file",
			path:           "/preview/test-dir/nonexistent.html",
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "Directory should redirect",
			path:           "/preview/test-dir",
			expectedStatus: http.StatusFound, // 302 redirect
		},
		{
			name:           "Non-HTML file",
			path:           "/preview/test-dir/image.png",
			expectedStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := http.NewRequest("GET", tt.path, nil)
			if err != nil {
				t.Fatal(err)
			}

			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			if status := rr.Code; status != tt.expectedStatus {
				t.Errorf("handler returned wrong status code: got %v want %v",
					status, tt.expectedStatus)
			}

			// Check contents if expected
			if tt.expectedStatus == http.StatusOK && tt.expectedContains != nil {
				for _, expected := range tt.expectedContains {
					if !strings.Contains(rr.Body.String(), expected) {
						t.Errorf("handler response doesn't contain %q", expected)
					}
				}
			}

			// Check redirect for directories
			if tt.expectedStatus == http.StatusFound {
				location := rr.Header().Get("Location")
				if strings.HasPrefix(tt.path, "/preview") && !strings.HasPrefix(location, "/") {
					t.Errorf("redirect location incorrect for directory: %q", location)
				}
			}
		})
	}
}

func TestProcessTemplateContent(t *testing.T) {
	// Save the original templateVars map
	originalVars := templateVars

	// Create a simplified templateVars map for testing
	templateVars = map[string]string{
		"{{.FirstName}}": "John",
		"{{.Email}}":     "john.doe@example.com",
	}

	// Restore the original templateVars after the test
	defer func() {
		templateVars = originalVars
	}()

	// Create temp directory for testing
	tmpDir := createTestTemplateDir(t)
	defer os.RemoveAll(tmpDir)

	tests := []struct {
		name     string
		content  string
		reqPath  string
		expected string
	}{
		{
			name:     "Replace BaseURL",
			content:  "Image: <img src=\"{{.BaseURL}}/image.png\">",
			reqPath:  "test-dir/page.html",
			expected: "Image: <img src=\"/templates/test-dir/image.png\">",
		},
		{
			name:     "Replace template variables",
			content:  "Hello {{.FirstName}}, your email is {{.Email}}",
			reqPath:  "test-dir/page.html",
			expected: "Hello John, your email is john.doe@example.com",
		},
		{
			name:     "Fix relative image paths",
			content:  "<img src=\"images/logo.png\">",
			reqPath:  "test-dir/page.html",
			expected: "<img src=\"/templates/test-dir/images/logo.png\">",
		},
		{
			name:     "Don't change absolute URLs",
			content:  "<img src=\"https://example.com/image.png\">",
			reqPath:  "test-dir/page.html",
			expected: "https://example.com/image.png",
		},
		{
			name:     "Preserve JavaScript comments",
			content:  "<script>// hello world\nfunction test() { // another comment\n  return true;\n}</script>",
			reqPath:  "test-dir/page.html",
			expected: "// hello world",
		},
		{
			name:     "Fix double slashes in URLs only",
			content:  "<img src=\"/path//to//image.png\"> <script>// comment</script>",
			reqPath:  "test-dir/page.html",
			expected: "// comment",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := processTemplateContent(tt.content, tt.reqPath, tmpDir)
			if !strings.Contains(result, tt.expected) {
				t.Errorf("processTemplateContent() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestProcessTemplateContentAssetFallback(t *testing.T) {
	// Save the original templateVars map
	originalVars := templateVars

	// Create a simplified templateVars map for testing
	templateVars = map[string]string{
		"{{.FirstName}}": "John",
		"{{.Email}}":     "john.doe@example.com",
	}

	// Restore the original templateVars after the test
	defer func() {
		templateVars = originalVars
	}()

	// Create temp directory structure for testing
	tmpDir, err := os.MkdirTemp("", "template-asset-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create template directory
	templateDir := filepath.Join(tmpDir, "Microsoft", "Emails", "Test")
	if err := os.MkdirAll(templateDir, 0755); err != nil {
		t.Fatalf("Failed to create template dir: %v", err)
	}

	// Create assets directory
	assetsDir := filepath.Join(tmpDir, "assets", "microsoft")
	if err := os.MkdirAll(assetsDir, 0755); err != nil {
		t.Fatalf("Failed to create assets dir: %v", err)
	}

	// Create an asset file in the assets directory
	logoPath := filepath.Join(assetsDir, "microsoft-logo.png")
	if err := os.WriteFile(logoPath, []byte("fake logo data"), 0644); err != nil {
		t.Fatalf("Failed to create logo file: %v", err)
	}

	tests := []struct {
		name     string
		content  string
		reqPath  string
		expected string
	}{
		{
			name:     "BaseURL asset fallback to global assets",
			content:  "<img src=\"{{.BaseURL}}/microsoft/microsoft-logo.png\" alt=\"Microsoft\">",
			reqPath:  "Microsoft/Emails/Test/email.html",
			expected: "<img src=\"/templates/assets/microsoft/microsoft-logo.png\" alt=\"Microsoft\">",
		},
		{
			name:     "Relative asset fallback to global assets",
			content:  "<img src=\"microsoft/microsoft-logo.png\" alt=\"Microsoft\">",
			reqPath:  "Microsoft/Emails/Test/email.html",
			expected: "<img src=\"/templates/assets/microsoft/microsoft-logo.png\" alt=\"Microsoft\">",
		},
		{
			name:     "Local asset takes precedence",
			content:  "<img src=\"local-image.png\" alt=\"Local\">",
			reqPath:  "Microsoft/Emails/Test/email.html",
			expected: "<img src=\"/templates/Microsoft/Emails/Test/local-image.png\" alt=\"Local\">",
		},
		{
			name:     "BaseURL with assets prefix fallback",
			content:  "<img src=\"{{.BaseURL}}/assets/microsoft/microsoft-logo.png\" alt=\"Microsoft\">",
			reqPath:  "Microsoft/Emails/Test/email.html",
			expected: "<img src=\"/templates/assets/microsoft/microsoft-logo.png\" alt=\"Microsoft\">",
		},
	}

	// Create a local asset file in template directory to test precedence
	localImagePath := filepath.Join(templateDir, "local-image.png")
	if err := os.WriteFile(localImagePath, []byte("local image data"), 0644); err != nil {
		t.Fatalf("Failed to create local image file: %v", err)
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := processTemplateContent(tt.content, tt.reqPath, tmpDir)
			if !strings.Contains(result, tt.expected) {
				t.Errorf("processTemplateContent() = %v, want to contain %v", result, tt.expected)
			}
		})
	}
}

// Helper function to create a test directory structure
func createTestTemplateDir(t *testing.T) string {
	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "template-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	// Create test directory structure
	testDir := filepath.Join(tmpDir, "test-dir")
	if err := os.Mkdir(testDir, 0755); err != nil {
		t.Fatalf("Failed to create test dir: %v", err)
	}

	// Create test HTML file
	testHTML := `<!DOCTYPE html>
<html>
<head>
    <title>Test template</title>
</head>
<body>
    <h1>Hello {{.FirstName}}</h1>
    <p>Your email is {{.Email}}</p>
    <a href="{{.URL}}">Click here</a>
    <img src="{{.BaseURL}}/images/logo.png">
</body>
</html>`

	if err := os.WriteFile(filepath.Join(testDir, "test.html"), []byte(testHTML), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Create test image file
	if err := os.WriteFile(filepath.Join(testDir, "image.png"), []byte("fake PNG data"), 0644); err != nil {
		t.Fatalf("Failed to write test image: %v", err)
	}

	return tmpDir
}
