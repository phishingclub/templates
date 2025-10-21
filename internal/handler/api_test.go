package handler

import (
	"archive/zip"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateCampaigns(t *testing.T) {
	// Create temporary directory for test
	tmpDir, err := os.MkdirTemp("", "campaign-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	tests := []struct {
		name        string
		setup       func(string) error
		expectError bool
		errorType   string
		errorValue  string
	}{
		{
			name: "No campaigns should pass",
			setup: func(baseDir string) error {
				return nil
			},
			expectError: false,
		},
		{
			name: "Single campaign should pass",
			setup: func(baseDir string) error {
				// Create a single campaign
				campaignDir := filepath.Join(baseDir, "microsoft-login")
				if err := os.MkdirAll(campaignDir, 0755); err != nil {
					return err
				}

				// Create data.yaml
				dataYaml := `name: "Microsoft Login Alert"
emails:
  - name: "Microsoft Login Alert"
    file: "email.html"
    envelope from: "security@microsoft.com"
    from: "Microsoft Security <security@microsoft.com>"
    subject: "Unusual Login Activity"
landing_pages:
  - name: "Microsoft Login Page"
    file: "landing.html"`

				if err := os.WriteFile(filepath.Join(campaignDir, "data.yaml"), []byte(dataYaml), 0644); err != nil {
					return err
				}

				// Create email.html
				if err := os.WriteFile(filepath.Join(campaignDir, "email.html"), []byte("<html>Email content</html>"), 0644); err != nil {
					return err
				}

				return nil
			},
			expectError: false,
		},
		{
			name: "Duplicate campaign names should fail",
			setup: func(baseDir string) error {
				// Create two campaigns with same name
				campaign1Dir := filepath.Join(baseDir, "microsoft-login")
				campaign2Dir := filepath.Join(baseDir, "ms-login-alert")

				for _, dir := range []string{campaign1Dir, campaign2Dir} {
					if err := os.MkdirAll(dir, 0755); err != nil {
						return err
					}

					// Both have the same campaign name
					dataYaml := `name: "Microsoft Login Alert"`
					if err := os.WriteFile(filepath.Join(dir, "data.yaml"), []byte(dataYaml), 0644); err != nil {
						return err
					}

					if err := os.WriteFile(filepath.Join(dir, "email.html"), []byte("<html>Email content</html>"), 0644); err != nil {
						return err
					}
				}

				return nil
			},
			expectError: true,
			errorType:   "name",
			errorValue:  "Microsoft Login Alert",
		},
		{
			name: "Same folder name should now pass (auto-resolved)",
			setup: func(baseDir string) error {
				// Create two campaigns in folders with same name (different paths)
				campaign1Dir := filepath.Join(baseDir, "company1", "login-alert")
				campaign2Dir := filepath.Join(baseDir, "company2", "login-alert")

				for _, dir := range []string{campaign1Dir, campaign2Dir} {
					if err := os.MkdirAll(dir, 0755); err != nil {
						return err
					}

					// Give each campaign a different name
					campaignName := "Campaign " + strings.Replace(dir, "/", "_", -1)
					dataYaml := `name: "` + campaignName + `"`
					if err := os.WriteFile(filepath.Join(dir, "data.yaml"), []byte(dataYaml), 0644); err != nil {
						return err
					}

					if err := os.WriteFile(filepath.Join(dir, "email.html"), []byte("<html>Email content</html>"), 0644); err != nil {
						return err
					}
				}

				return nil
			},
			expectError: false,
		},
		{
			name: "Email and page with same name in same campaign should pass",
			setup: func(baseDir string) error {
				campaignDir := filepath.Join(baseDir, "microsoft-login")
				if err := os.MkdirAll(campaignDir, 0755); err != nil {
					return err
				}

				// Email and landing page have same name - this should be allowed
				dataYaml := `name: "Microsoft Login Campaign"
emails:
  - name: "Microsoft Login Alert"
    file: "email.html"
    envelope from: "security@microsoft.com"
    from: "Microsoft Security <security@microsoft.com>"
    subject: "Unusual Login Activity"
landing_pages:
  - name: "Microsoft Login Alert"
    file: "landing.html"`

				if err := os.WriteFile(filepath.Join(campaignDir, "data.yaml"), []byte(dataYaml), 0644); err != nil {
					return err
				}

				if err := os.WriteFile(filepath.Join(campaignDir, "email.html"), []byte("<html>Email content</html>"), 0644); err != nil {
					return err
				}

				if err := os.WriteFile(filepath.Join(campaignDir, "landing.html"), []byte("<html>Landing page</html>"), 0644); err != nil {
					return err
				}

				return nil
			},
			expectError: false,
		},
		{
			name: "Campaign without data.yaml uses folder name",
			setup: func(baseDir string) error {
				// Create two campaigns with same folder name but no data.yaml
				// They will use folder name as campaign name, causing name conflict
				campaign1Dir := filepath.Join(baseDir, "login-alert")
				campaign2Dir := filepath.Join(baseDir, "subfolder", "login-alert")

				for _, dir := range []string{campaign1Dir, campaign2Dir} {
					if err := os.MkdirAll(dir, 0755); err != nil {
						return err
					}

					if err := os.WriteFile(filepath.Join(dir, "email.html"), []byte("<html>Email content</html>"), 0644); err != nil {
						return err
					}
				}

				return nil
			},
			expectError: true,
			errorType:   "name",
			errorValue:  "login-alert",
		},
		{
			name: "Different campaign names with same folder now pass (auto-resolved)",
			setup: func(baseDir string) error {
				// Create two campaigns with different names but same folder name
				campaign1Dir := filepath.Join(baseDir, "companyA", "login-alert")
				campaign2Dir := filepath.Join(baseDir, "companyB", "login-alert")

				// Campaign 1
				if err := os.MkdirAll(campaign1Dir, 0755); err != nil {
					return err
				}
				dataYaml1 := `name: "Microsoft Login Alert"`
				if err := os.WriteFile(filepath.Join(campaign1Dir, "data.yaml"), []byte(dataYaml1), 0644); err != nil {
					return err
				}
				if err := os.WriteFile(filepath.Join(campaign1Dir, "email.html"), []byte("<html>Email content</html>"), 0644); err != nil {
					return err
				}

				// Campaign 2
				if err := os.MkdirAll(campaign2Dir, 0755); err != nil {
					return err
				}
				dataYaml2 := `name: "Google Login Alert"`
				if err := os.WriteFile(filepath.Join(campaign2Dir, "data.yaml"), []byte(dataYaml2), 0644); err != nil {
					return err
				}
				if err := os.WriteFile(filepath.Join(campaign2Dir, "email.html"), []byte("<html>Email content</html>"), 0644); err != nil {
					return err
				}

				return nil
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clean the temp directory
			os.RemoveAll(tmpDir)
			os.MkdirAll(tmpDir, 0755)

			// Setup test scenario
			if err := tt.setup(tmpDir); err != nil {
				t.Fatalf("Failed to setup test: %v", err)
			}

			// Run validation
			err := validateCampaigns(tmpDir)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
					return
				}

				dupErr, ok := err.(DuplicateError)
				if !ok {
					t.Errorf("Expected DuplicateError but got: %T - %v", err, err)
					return
				}

				if dupErr.Type != tt.errorType {
					t.Errorf("Expected error type %s but got %s. Full error: %v", tt.errorType, dupErr.Type, dupErr)
				}

				if dupErr.Value != tt.errorValue {
					t.Errorf("Expected error value %s but got %s. Full error: %v", tt.errorValue, dupErr.Value, dupErr)
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}
			}
		})
	}
}

func TestDuplicateError(t *testing.T) {
	tests := []struct {
		name     string
		dupError DuplicateError
		expected string
	}{
		{
			name: "Name duplicate error",
			dupError: DuplicateError{
				Type:      "name",
				Value:     "Microsoft Login",
				Campaigns: []string{"campaign1", "campaign2"},
			},
			expected: "Duplicate campaign name 'Microsoft Login' found in: campaign1, campaign2",
		},
		{
			name: "Folder duplicate error",
			dupError: DuplicateError{
				Type:      "folder",
				Value:     "login-alert",
				Campaigns: []string{"company1/login-alert", "company2/login-alert"},
			},
			expected: "Multiple campaigns found in folder 'login-alert': company1/login-alert, company2/login-alert",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := tt.dupError.Error()
			if actual != tt.expected {
				t.Errorf("Expected error message:\n%s\nBut got:\n%s", tt.expected, actual)
			}
		})
	}
}

func TestValidateCampaignsSkipsPrivate(t *testing.T) {
	// Create temporary directory for test
	tmpDir, err := os.MkdirTemp("", "campaign-private-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create private directory with campaigns (should be skipped)
	privateDir := filepath.Join(tmpDir, "private", "client-company")
	if err := os.MkdirAll(privateDir, 0755); err != nil {
		t.Fatalf("Failed to create private dir: %v", err)
	}

	// Add a campaign in private folder
	dataYaml := `name: "Client Specific Campaign"`
	if err := os.WriteFile(filepath.Join(privateDir, "data.yaml"), []byte(dataYaml), 0644); err != nil {
		t.Fatalf("Failed to create private data.yaml: %v", err)
	}

	if err := os.WriteFile(filepath.Join(privateDir, "email.html"), []byte("<html>Private email</html>"), 0644); err != nil {
		t.Fatalf("Failed to create private email.html: %v", err)
	}

	// Create a public campaign with same name (should be detected if private isn't skipped)
	publicDir := filepath.Join(tmpDir, "generic-service")
	if err := os.MkdirAll(publicDir, 0755); err != nil {
		t.Fatalf("Failed to create public dir: %v", err)
	}

	if err := os.WriteFile(filepath.Join(publicDir, "data.yaml"), []byte(dataYaml), 0644); err != nil {
		t.Fatalf("Failed to create public data.yaml: %v", err)
	}

	if err := os.WriteFile(filepath.Join(publicDir, "email.html"), []byte("<html>Public email</html>"), 0644); err != nil {
		t.Fatalf("Failed to create public email.html: %v", err)
	}

	// Validation should pass because private folder is skipped
	err = validateCampaigns(tmpDir)
	if err != nil {
		t.Errorf("Expected no error but got: %v (private folder should be skipped)", err)
	}
}

func TestValidateCampaignsSkipsPrivateCaseInsensitive(t *testing.T) {
	// Create temporary directory for test
	tmpDir, err := os.MkdirTemp("", "campaign-private-case-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Test both "private" and "Private" folders
	testCases := []string{"private", "Private", "PRIVATE"}

	for _, privateFolder := range testCases {
		t.Run(fmt.Sprintf("Skip %s folder", privateFolder), func(t *testing.T) {
			// Clean the temp directory
			os.RemoveAll(tmpDir)
			os.MkdirAll(tmpDir, 0755)

			// Create private directory with campaigns (should be skipped)
			privateDirPath := filepath.Join(tmpDir, privateFolder, "client-company")
			if err := os.MkdirAll(privateDirPath, 0755); err != nil {
				t.Fatalf("Failed to create %s dir: %v", privateFolder, err)
			}

			// Add a campaign in private folder
			dataYaml := `name: "Client Specific Campaign"`
			if err := os.WriteFile(filepath.Join(privateDirPath, "data.yaml"), []byte(dataYaml), 0644); err != nil {
				t.Fatalf("Failed to create %s data.yaml: %v", privateFolder, err)
			}

			if err := os.WriteFile(filepath.Join(privateDirPath, "email.html"), []byte("<html>Private email</html>"), 0644); err != nil {
				t.Fatalf("Failed to create %s email.html: %v", privateFolder, err)
			}

			// Create a public campaign with same name (should be detected if private isn't skipped)
			publicDir := filepath.Join(tmpDir, "generic-service")
			if err := os.MkdirAll(publicDir, 0755); err != nil {
				t.Fatalf("Failed to create public dir: %v", err)
			}

			if err := os.WriteFile(filepath.Join(publicDir, "data.yaml"), []byte(dataYaml), 0644); err != nil {
				t.Fatalf("Failed to create public data.yaml: %v", err)
			}

			if err := os.WriteFile(filepath.Join(publicDir, "email.html"), []byte("<html>Public email</html>"), 0644); err != nil {
				t.Fatalf("Failed to create public email.html: %v", err)
			}

			// Validation should pass because private folder is skipped
			err = validateCampaigns(tmpDir)
			if err != nil {
				t.Errorf("Expected no error but got: %v (%s folder should be skipped)", err, privateFolder)
			}
		})
	}
}

func TestValidateCampaignsAllowsSameFolderInDifferentOrgDirs(t *testing.T) {
	// Create temporary directory for test
	tmpDir, err := os.MkdirTemp("", "campaign-org-dirs-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create the structure that was causing the error:
	// Contoso/Emails/Chat beta invite/
	// Contoso/Landing Pages/Chat beta invite/
	emailDir := filepath.Join(tmpDir, "Contoso", "Emails", "Chat beta invite")
	landingDir := filepath.Join(tmpDir, "Contoso", "Landing Pages", "Chat beta invite")

	for _, dir := range []string{emailDir, landingDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("Failed to create dir %s: %v", dir, err)
		}

		// Both can have the same campaign name - this should be allowed
		dataYaml := `name: "Chat Beta Invite Campaign"`
		if err := os.WriteFile(filepath.Join(dir, "data.yaml"), []byte(dataYaml), 0644); err != nil {
			t.Fatalf("Failed to create data.yaml in %s: %v", dir, err)
		}

		if err := os.WriteFile(filepath.Join(dir, "email.html"), []byte("<html>Content</html>"), 0644); err != nil {
			t.Fatalf("Failed to create email.html in %s: %v", dir, err)
		}
	}

	// This should pass - same folder names in different organizational directories should be allowed
	err = validateCampaigns(tmpDir)
	if err != nil {
		t.Errorf("Expected no error but got: %v (same folder names in different org dirs should be allowed)", err)
	}
}

func TestAutoNumberingResolvesConflicts(t *testing.T) {
	// Create temporary directory for test
	tmpDir, err := os.MkdirTemp("", "auto-numbering-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create structure that would cause folder conflicts
	emailDir := filepath.Join(tmpDir, "Contoso", "Emails", "Chat beta invite")
	landingDir := filepath.Join(tmpDir, "Contoso", "Landing Pages", "Chat beta invite")

	for _, dir := range []string{emailDir, landingDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("Failed to create dir %s: %v", dir, err)
		}

		dataYaml := `name: "Chat Beta Invite Campaign"`
		if err := os.WriteFile(filepath.Join(dir, "data.yaml"), []byte(dataYaml), 0644); err != nil {
			t.Fatalf("Failed to create data.yaml in %s: %v", dir, err)
		}

		if err := os.WriteFile(filepath.Join(dir, "email.html"), []byte("<html>Content</html>"), 0644); err != nil {
			t.Fatalf("Failed to create email.html in %s: %v", dir, err)
		}
	}

	// Validation should pass now since folder conflicts are auto-resolved during export
	err = validateCampaigns(tmpDir)
	if err != nil {
		t.Errorf("Expected no error but got: %v (folder conflicts should be auto-resolved)", err)
	}
}

func TestExportCreatesHashedFolders(t *testing.T) {
	// Create temporary directory for test
	tmpDir, err := os.MkdirTemp("", "export-hashed-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create structure that would cause folder conflicts
	emailDir := filepath.Join(tmpDir, "Contoso", "Emails", "Chat beta invite")
	landingDir := filepath.Join(tmpDir, "Contoso", "Landing Pages", "Chat beta invite")

	for _, dir := range []string{emailDir, landingDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("Failed to create dir %s: %v", dir, err)
		}

		dataYaml := `name: "Chat Beta Invite Campaign"`
		if err := os.WriteFile(filepath.Join(dir, "data.yaml"), []byte(dataYaml), 0644); err != nil {
			t.Fatalf("Failed to create data.yaml in %s: %v", dir, err)
		}

		if err := os.WriteFile(filepath.Join(dir, "email.html"), []byte("<html>Content</html>"), 0644); err != nil {
			t.Fatalf("Failed to create email.html in %s: %v", dir, err)
		}
	}

	// Create a buffer to capture zip output
	var buf bytes.Buffer
	zipWriter := zip.NewWriter(&buf)

	// Test the addPhishingTemplates function
	err = addPhishingTemplates(zipWriter, tmpDir)
	if err != nil {
		t.Fatalf("addPhishingTemplates failed: %v", err)
	}

	err = zipWriter.Close()
	if err != nil {
		t.Fatalf("Failed to close zip writer: %v", err)
	}

	// Read the zip and verify folder names
	zipReader, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatalf("Failed to create zip reader: %v", err)
	}

	foundFolders := make(map[string]bool)
	for _, file := range zipReader.File {
		if strings.HasPrefix(file.Name, "templates/") {
			parts := strings.Split(file.Name, "/")
			if len(parts) >= 2 {
				folderName := parts[1]
				foundFolders[folderName] = true
			}
		}
	}

	// Should have "Chat beta invite" and "Chat beta invite-<hash>"
	if len(foundFolders) != 2 {
		t.Errorf("Expected exactly 2 folders, but found %d: %v", len(foundFolders), foundFolders)
	}

	hasOriginal := false
	hasHashed := false
	for folderName := range foundFolders {
		if folderName == "Chat beta invite" {
			hasOriginal = true
		} else if strings.HasPrefix(folderName, "Chat beta invite-") && len(folderName) == len("Chat beta invite")+9 {
			// Should be original name + "-" + 8 char hash
			hasHashed = true
		}
	}

	if !hasOriginal {
		t.Errorf("Expected original folder 'Chat beta invite' not found. Found folders: %v", foundFolders)
	}
	if !hasHashed {
		t.Errorf("Expected hashed folder 'Chat beta invite-<hash>' not found. Found folders: %v", foundFolders)
	}
}

func TestExportSkipsPrivateFolders(t *testing.T) {
	// Create temporary directory for test
	tmpDir, err := os.MkdirTemp("", "export-private-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create private directory with campaign
	privateDir := filepath.Join(tmpDir, "private", "client-specific")
	if err := os.MkdirAll(privateDir, 0755); err != nil {
		t.Fatalf("Failed to create private dir: %v", err)
	}

	dataYaml := `name: "Client Specific Campaign"`
	if err := os.WriteFile(filepath.Join(privateDir, "data.yaml"), []byte(dataYaml), 0644); err != nil {
		t.Fatalf("Failed to create private data.yaml: %v", err)
	}

	if err := os.WriteFile(filepath.Join(privateDir, "email.html"), []byte("<html>Private email</html>"), 0644); err != nil {
		t.Fatalf("Failed to create private email.html: %v", err)
	}

	// Create public campaign
	publicDir := filepath.Join(tmpDir, "generic-service")
	if err := os.MkdirAll(publicDir, 0755); err != nil {
		t.Fatalf("Failed to create public dir: %v", err)
	}

	publicDataYaml := `name: "Generic Service Campaign"`
	if err := os.WriteFile(filepath.Join(publicDir, "data.yaml"), []byte(publicDataYaml), 0644); err != nil {
		t.Fatalf("Failed to create public data.yaml: %v", err)
	}

	if err := os.WriteFile(filepath.Join(publicDir, "email.html"), []byte("<html>Public email</html>"), 0644); err != nil {
		t.Fatalf("Failed to create public email.html: %v", err)
	}

	// Test that private folders are skipped during export validation
	err = validateCampaigns(tmpDir)
	if err != nil {
		t.Errorf("Expected no error but got: %v (private folder should be skipped)", err)
	}
}

func TestValidateCampaignsWithAssets(t *testing.T) {
	// Create temporary directory for test
	tmpDir, err := os.MkdirTemp("", "campaign-assets-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create assets directory (should be skipped)
	assetsDir := filepath.Join(tmpDir, "assets")
	if err := os.MkdirAll(assetsDir, 0755); err != nil {
		t.Fatalf("Failed to create assets dir: %v", err)
	}

	// Add a file in assets (should not be considered a campaign)
	if err := os.WriteFile(filepath.Join(assetsDir, "logo.png"), []byte("fake image"), 0644); err != nil {
		t.Fatalf("Failed to create asset file: %v", err)
	}

	// Create a legitimate campaign
	campaignDir := filepath.Join(tmpDir, "microsoft-login")
	if err := os.MkdirAll(campaignDir, 0755); err != nil {
		t.Fatalf("Failed to create campaign dir: %v", err)
	}

	dataYaml := `name: "Microsoft Login Alert"`
	if err := os.WriteFile(filepath.Join(campaignDir, "data.yaml"), []byte(dataYaml), 0644); err != nil {
		t.Fatalf("Failed to create data.yaml: %v", err)
	}

	if err := os.WriteFile(filepath.Join(campaignDir, "email.html"), []byte("<html>Email content</html>"), 0644); err != nil {
		t.Fatalf("Failed to create email.html: %v", err)
	}

	// Validation should pass (assets directory should be ignored)
	err = validateCampaigns(tmpDir)
	if err != nil {
		t.Errorf("Expected no error but got: %v", err)
	}
}
