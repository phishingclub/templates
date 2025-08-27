package handler

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestValidatePath(t *testing.T) {
	// Use a fake base directory - no need to create it
	baseDir := "/fake/base/dir"

	tests := []struct {
		name        string
		reqPath     string
		expectError bool
		description string
	}{
		// Valid paths
		{
			name:        "ValidFile",
			reqPath:     "test.html",
			expectError: false,
			description: "Normal file access should work",
		},
		{
			name:        "ValidSubdirFile",
			reqPath:     "subdir/sub.html",
			expectError: false,
			description: "Subdirectory file access should work",
		},
		{
			name:        "ValidPathWithSlash",
			reqPath:     "/test.html",
			expectError: true,
			description: "Leading slash should be rejected as absolute path",
		},
		{
			name:        "EmptyPath",
			reqPath:     "",
			expectError: false,
			description: "Empty path should resolve to base directory",
		},
		{
			name:        "DotPath",
			reqPath:     ".",
			expectError: false,
			description: "Current directory reference should work",
		},
		{
			name:        "ValidPathWithDotSlash",
			reqPath:     "./test.html",
			expectError: false,
			description: "Dot slash prefix should work",
		},
		{
			name:        "DeepValidPath",
			reqPath:     "templates/phishing/microsoft/login.html",
			expectError: false,
			description: "Deep valid path should work",
		},

		// Directory traversal attempts - these should all fail
		{
			name:        "BasicDotDot",
			reqPath:     "../",
			expectError: true,
			description: "Basic parent directory traversal should be blocked",
		},
		{
			name:        "DotDotWithFile",
			reqPath:     "../etc/passwd",
			expectError: true,
			description: "Parent directory with file access should be blocked",
		},
		{
			name:        "MultipleDotDot",
			reqPath:     "../../etc/passwd",
			expectError: true,
			description: "Multiple parent directory traversal should be blocked",
		},
		{
			name:        "DeepDotDot",
			reqPath:     "../../../../../../../etc/passwd",
			expectError: true,
			description: "Deep directory traversal should be blocked",
		},
		{
			name:        "DotDotInMiddle",
			reqPath:     "subdir/../../../etc/passwd",
			expectError: true,
			description: "Directory traversal in middle of path should be blocked",
		},
		{
			name:        "WindowsBackslash",
			reqPath:     "..\\",
			expectError: true,
			description: "Windows-style backslash traversal should be blocked",
		},
		{
			name:        "WindowsBackslashWithFile",
			reqPath:     "..\\..\\windows\\system32\\config\\sam",
			expectError: true,
			description: "Windows backslash with system file should be blocked",
		},
		{
			name:        "MixedSlashes",
			reqPath:     "../\\../etc/passwd",
			expectError: true,
			description: "Mixed forward and backslashes should be blocked",
		},
		{
			name:        "AbsolutePath",
			reqPath:     "/etc/passwd",
			expectError: true,
			description: "Absolute paths outside base should be blocked",
		},
		{
			name:        "AbsoluteWindowsPath",
			reqPath:     "C:\\Windows\\System32\\config\\sam",
			expectError: true,
			description: "Absolute Windows paths should be blocked",
		},
		{
			name:        "OverlongPath",
			reqPath:     strings.Repeat("../", 100) + "etc/passwd",
			expectError: true,
			description: "Overlong traversal paths should be blocked",
		},
		{
			name:        "DotDotWithValidSubpath",
			reqPath:     "subdir/../../etc/passwd",
			expectError: true,
			description: "Valid subpath with traversal should be blocked",
		},
		{
			name:        "SlashDotDot",
			reqPath:     "//../etc/passwd",
			expectError: true,
			description: "Slash dot dot traversal should be blocked",
		},
		{
			name:        "BackslashDotDot",
			reqPath:     "\\..\\etc\\passwd",
			expectError: true,
			description: "Backslash dot dot traversal should be blocked",
		},
		{
			name:        "RealisticAttack1",
			reqPath:     "templates/phishing/microsoft/../../../etc/passwd",
			expectError: true,
			description: "Realistic attack through template structure should be blocked",
		},
		{
			name:        "RealisticAttack2",
			reqPath:     "templates/phishing/microsoft/../../../../etc/shadow",
			expectError: true,
			description: "Deep realistic attack should be blocked",
		},
		{
			name:        "RealisticAttack3",
			reqPath:     "templates/../../../proc/self/environ",
			expectError: true,
			description: "Attack through templates dir should be blocked",
		},
		{
			name:        "WindowsRealisticAttack",
			reqPath:     "templates\\phishing\\microsoft\\..\\..\\..\\windows\\system32\\config\\sam",
			expectError: true,
			description: "Windows-style realistic attack should be blocked",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := validatePath(baseDir, tt.reqPath)
			
			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error for path %q but got none. Result: %s", tt.reqPath, result)
				}
				// Double-check: even if no error, ensure result is still safe
				if err == nil && result != "" {
					if !isPathSafe(baseDir, result) {
						t.Errorf("Path %q resulted in unsafe location: %s", tt.reqPath, result)
					}
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error for valid path %q: %v", tt.reqPath, err)
				}
				// Verify the result is within the base directory
				if result != "" && !isPathSafe(baseDir, result) {
					t.Errorf("Valid path %q resulted in location outside base: %s", tt.reqPath, result)
				}
			}
		})
	}
}

func TestValidatePathEdgeCases(t *testing.T) {
	baseDir := "/test/base"

	edgeCases := []struct {
		name        string
		reqPath     string
		expectError bool
	}{
		{
			name:        "EmptyString",
			reqPath:     "",
			expectError: false,
		},
		{
			name:        "SingleDot",
			reqPath:     ".",
			expectError: false,
		},
		{
			name:        "DoubleDot",
			reqPath:     "..",
			expectError: true,
		},
		{
			name:        "TripleDot",
			reqPath:     "...",
			expectError: true,
		},
		{
			name:        "DotSlash",
			reqPath:     "./",
			expectError: false,
		},
		{
			name:        "SlashDot",
			reqPath:     "/.",
			expectError: true,
		},
		{
			name:        "SlashDotSlash",
			reqPath:     "/./",
			expectError: true,
		},
		{
			name:        "DotDotSlash",
			reqPath:     "../",
			expectError: true,
		},
		{
			name:        "SlashDotDotSlash",
			reqPath:     "/../",
			expectError: true,
		},
		{
			name:        "OnlySlashes",
			reqPath:     "///",
			expectError: true,
		},
		{
			name:        "OnlyBackslashes",
			reqPath:     "\\\\\\",
			expectError: true,
		},
	}

	for _, tc := range edgeCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := validatePath(baseDir, tc.reqPath)
			if tc.expectError && err == nil {
				t.Errorf("Expected error for edge case %q but got none", tc.reqPath)
			}
			if !tc.expectError && err != nil {
				t.Errorf("Unexpected error for valid edge case %q: %v", tc.reqPath, err)
			}
		})
	}
}

func TestValidatePathNormalization(t *testing.T) {
	baseDir := "/secure/base"

	// Test various path normalization scenarios
	tests := []struct {
		name        string
		reqPath     string
		expectError bool
	}{
		{
			name:        "ExtraSlashes",
			reqPath:     "dir//file.html",
			expectError: false,
		},
		{
			name:        "TrailingSlash",
			reqPath:     "dir/",
			expectError: false,
		},
		{
			name:        "DotInPath",
			reqPath:     "dir/./file.html",
			expectError: false,
		},
		{
			name:        "MultipleDots",
			reqPath:     "dir/././file.html",
			expectError: false,
		},
		{
			name:        "DotDotTraversal",
			reqPath:     "dir/../file.html",
			expectError: true, // Block all .. usage for security
		},
		{
			name:        "DotDotEscape",
			reqPath:     "../file.html",
			expectError: true, // This escapes base
		},
		{
			name:        "ComplexValidPath",
			reqPath:     "a/b/../c/./d/file.html",
			expectError: true, // Block all .. usage for security
		},
		{
			name:        "ComplexEscapePath",
			reqPath:     "a/b/../../c/file.html",
			expectError: true, // Contains .. which is blocked
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := validatePath(baseDir, tt.reqPath)
			
			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error for path %q but got none. Result: %s", tt.reqPath, result)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error for valid path %q: %v", tt.reqPath, err)
				}
			}
		})
	}
}

func TestValidatePathEncodingBypass(t *testing.T) {
	baseDir := "/test/base"

	// Test various encoding bypass attempts that attackers commonly use
	// to circumvent basic ".." detection in path validation functions.
	// These tests ensure our validatePath function handles sophisticated
	// encoding attacks that could lead to directory traversal.
	encodingTests := []struct {
		name        string
		reqPath     string
		expectError bool
		description string
	}{
		// URL encoding attempts
		{
			name:        "URLEncodedDotDot",
			reqPath:     "%2e%2e/",
			expectError: true,
			description: "URL encoded .. should be blocked",
		},
		{
			name:        "URLEncodedSlash",
			reqPath:     "%2e%2e%2f",
			expectError: true,
			description: "URL encoded ../ should be blocked",
		},
		{
			name:        "URLEncodedBackslash",
			reqPath:     "%2e%2e%5c",
			expectError: true,
			description: "URL encoded ..\\ should be blocked",
		},
		{
			name:        "PartialURLEncoding",
			reqPath:     "..%2f",
			expectError: true,
			description: "Partial URL encoding should be blocked",
		},
		{
			name:        "MixedURLEncoding",
			reqPath:     "%2e.%2f",
			expectError: true,
			description: "Mixed URL encoding should be blocked",
		},
		
		// Double URL encoding
		{
			name:        "DoubleURLEncodedDot",
			reqPath:     "%252e%252e/",
			expectError: true,
			description: "Double URL encoded .. should be blocked",
		},
		{
			name:        "DoubleURLEncodedSlash",
			reqPath:     "%252e%252e%252f",
			expectError: true,
			description: "Double URL encoded ../ should be blocked",
		},
		
		// Unicode encoding attempts
		{
			name:        "UnicodeDot",
			reqPath:     "\u002e\u002e/",
			expectError: true,
			description: "Unicode encoded .. should be blocked",
		},
		{
			name:        "UnicodeSlash",
			reqPath:     "\u002e\u002e\u002f",
			expectError: true,
			description: "Unicode encoded ../ should be blocked",
		},
		{
			name:        "UnicodeBackslash",
			reqPath:     "\u002e\u002e\u005c",
			expectError: true,
			description: "Unicode encoded ..\\ should be blocked",
		},
		
		// UTF-8 encoding attempts
		{
			name:        "UTF8OverlongDot",
			reqPath:     "\xc0\xae\xc0\xae/",
			expectError: true,
			description: "UTF-8 overlong encoded .. should be blocked",
		},
		{
			name:        "UTF8OverlongSlash",
			reqPath:     "\xc0\xae\xc0\xae\xc0\xaf",
			expectError: true,
			description: "UTF-8 overlong encoded ../ should be blocked",
		},
		
		// Null byte injection
		{
			name:        "NullByteAfterDotDot",
			reqPath:     "../\x00etc/passwd",
			expectError: true,
			description: "Null byte after .. should be blocked",
		},
		{
			name:        "NullByteInMiddle",
			reqPath:     "..\x00./etc/passwd",
			expectError: true,
			description: "Null byte in middle should be blocked",
		},
		{
			name:        "NullByteTraversal",
			reqPath:     "../etc/passwd\x00.html",
			expectError: true,
			description: "Null byte after path should be blocked",
		},
		
		// Alternative directory separators
		{
			name:        "AlternativeSlash1",
			reqPath:     "..\u2215etc\u2215passwd",
			expectError: true,
			description: "Unicode division slash should be blocked",
		},
		{
			name:        "AlternativeSlash2",
			reqPath:     "..\u2044etc\u2044passwd",
			expectError: true,
			description: "Unicode fraction slash should be blocked",
		},
		
		// HTML entity encoding
		{
			name:        "HTMLEntityDot",
			reqPath:     "&#46;&#46;/",
			expectError: true,
			description: "HTML entity encoded .. should be blocked",
		},
		{
			name:        "HTMLEntitySlash",
			reqPath:     "&#46;&#46;&#47;",
			expectError: true,
			description: "HTML entity encoded ../ should be blocked",
		},
		{
			name:        "HTMLHexEntity",
			reqPath:     "&#x2e;&#x2e;&#x2f;",
			expectError: true,
			description: "HTML hex entity encoded ../ should be blocked",
		},
		
		// Base64 attempts (should fail as invalid paths)
		{
			name:        "Base64DotDot",
			reqPath:     "Li4v", // base64 for "../"
			expectError: false, // This would be treated as a filename
			description: "Base64 encoded path should be treated as filename",
		},
		
		// Space and tab variations
		{
			name:        "SpacesInDotDot",
			reqPath:     ". ./",
			expectError: true,
			description: "Spaces in .. should be blocked",
		},
		{
			name:        "TabsInDotDot",
			reqPath:     ".\t./",
			expectError: true,
			description: "Tabs in .. should be blocked",
		},
		{
			name:        "LeadingSpaces",
			reqPath:     " ../",
			expectError: true,
			description: "Leading spaces before .. should be blocked",
		},
		{
			name:        "TrailingSpaces",
			reqPath:     "../ ",
			expectError: true,
			description: "Trailing spaces after .. should be blocked",
		},
		
		// Case variations (should not matter on most filesystems)
		{
			name:        "UppercaseDrive",
			reqPath:     "C:/windows/system32/config/sam",
			expectError: true,
			description: "Uppercase drive letter should be blocked",
		},
		{
			name:        "LowercaseDrive",
			reqPath:     "c:/windows/system32/config/sam",
			expectError: true,
			description: "Lowercase drive letter should be blocked",
		},
		
		// Multiple encoding layers
		{
			name:        "TripleEncoded",
			reqPath:     "%25252e%25252e%25252f",
			expectError: true,
			description: "Triple encoded ../ should be blocked",
		},
		
		// Platform-specific bypasses
		{
			name:        "WindowsUNCPath",
			reqPath:     "\\\\server\\share\\file",
			expectError: true,
			description: "Windows UNC path should be blocked",
		},
		{
			name:        "WindowsDevicePath",
			reqPath:     "\\\\.\\C:\\windows\\system32",
			expectError: true,
			description: "Windows device path should be blocked",
		},
		
		// Control characters
		{
			name:        "CarriageReturn",
			reqPath:     "..\r/etc/passwd",
			expectError: true,
			description: "Carriage return in path should be blocked",
		},
		{
			name:        "LineFeed",
			reqPath:     "..\n/etc/passwd",
			expectError: true,
			description: "Line feed in path should be blocked",
		},
		{
			name:        "VerticalTab",
			reqPath:     "..\v/etc/passwd",
			expectError: true,
			description: "Vertical tab in path should be blocked",
		},
		{
			name:        "FormFeed",
			reqPath:     "..\f/etc/passwd",
			expectError: true,
			description: "Form feed in path should be blocked",
		},
	}

	for _, tt := range encodingTests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := validatePath(baseDir, tt.reqPath)
			
			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error for encoded path %q but got none. Result: %s. %s", tt.reqPath, result, tt.description)
				}
				// Double-check: even if no error, ensure result is still safe
				if err == nil && result != "" && !isPathSafe(baseDir, result) {
					t.Errorf("Encoded path %q resulted in unsafe location: %s", tt.reqPath, result)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error for path %q: %v. %s", tt.reqPath, err, tt.description)
				}
				// Verify the result is within the base directory
				if result != "" && !isPathSafe(baseDir, result) {
					t.Errorf("Path %q resulted in location outside base: %s", tt.reqPath, result)
				}
			}
		})
	}
}

func BenchmarkValidatePath(b *testing.B) {
	baseDir := "/benchmark/base"
	testPaths := []string{
		"normal/file.html",
		"../../../etc/passwd",
		"subdir/another/file.css",
		"../../../../../../../../etc/shadow",
		"valid/path/with/many/segments/file.js",
		"templates/phishing/microsoft/../../../etc/passwd",
		"..\\..\\windows\\system32\\config\\sam",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, path := range testPaths {
			validatePath(baseDir, path)
		}
	}
}

// isPathSafe verifies that a resolved path is within the base directory.
// This helper function provides defense-in-depth verification that our
// validatePath function correctly contains paths within the allowed boundary.
// 
// Returns false if:
// - The result path escapes the base directory
// - Path resolution fails
// - The path would allow access to files outside the intended scope
//
// This function complements the validatePath security checks and should
// continue to pass even when using Go 1.24's os.Root implementation.
func isPathSafe(baseDir, resultPath string) bool {
	absBase, err := filepath.Abs(baseDir)
	if err != nil {
		return false
	}
	
	absResult, err := filepath.Abs(resultPath)
	if err != nil {
		return false
	}
	
	return strings.HasPrefix(absResult, absBase+string(filepath.Separator)) || absResult == absBase
}