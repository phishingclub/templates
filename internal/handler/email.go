package handler

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/smtp"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// EmailData represents the structure of data.yaml for email templates
type EmailData struct {
	Name   string `yaml:"name"`
	Emails []struct {
		Name         string `yaml:"name"`
		File         string `yaml:"file"`
		EnvelopeFrom string `yaml:"envelope from"`
		From         string `yaml:"from"`
		Subject      string `yaml:"subject"`
	} `yaml:"emails"`
	LandingPages []struct {
		Name string `yaml:"name"`
		File string `yaml:"file"`
	} `yaml:"landing_pages"`
}

// SendEmailRequest represents the JSON request for sending emails
type SendEmailRequest struct {
	TemplatePath string `json:"templatePath"`
	To           string `json:"to"`
}

// SendEmailResponse represents the JSON response
type SendEmailResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// IsEmailTemplate checks if a given template path represents an email template
func IsEmailTemplate(baseDir, templatePath string) (bool, *EmailData, error) {
	// Get the directory containing the template
	templateDir := filepath.Dir(templatePath)
	dataYamlPath := filepath.Join(baseDir, templateDir, "data.yaml")

	// Check if data.yaml exists
	if _, err := os.Stat(dataYamlPath); os.IsNotExist(err) {
		return false, nil, nil
	}

	// Read and parse data.yaml
	yamlData, err := os.ReadFile(dataYamlPath)
	if err != nil {
		return false, nil, err
	}

	var emailData EmailData
	if err := yaml.Unmarshal(yamlData, &emailData); err != nil {
		return false, nil, err
	}

	// Check if the template file is listed in the emails section
	templateFile := filepath.Base(templatePath)
	for _, email := range emailData.Emails {
		if email.File == templateFile {
			return true, &emailData, nil
		}
	}

	return false, &emailData, nil
}

// SendTestEmailHandler handles POST requests to send test emails
func SendTestEmailHandler(baseDir string, serverAddr string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		w.Header().Set("Content-Type", "application/json")

		// Parse JSON request
		var req SendEmailRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeErrorResponse(w, "Invalid JSON request", http.StatusBadRequest)
			return
		}

		// Validate template path
		fsPath, err := validatePath(baseDir, req.TemplatePath)
		if err != nil {
			writeErrorResponse(w, "Invalid template path", http.StatusBadRequest)
			return
		}

		// Check if file exists
		if _, err := os.Stat(fsPath); os.IsNotExist(err) {
			writeErrorResponse(w, "Template not found", http.StatusNotFound)
			return
		}

		// Check if this is an email template
		isEmail, emailData, err := IsEmailTemplate(baseDir, req.TemplatePath)
		if err != nil {
			writeErrorResponse(w, "Error reading template data: "+err.Error(), http.StatusInternalServerError)
			return
		}

		if !isEmail {
			writeErrorResponse(w, "Not an email template", http.StatusBadRequest)
			return
		}

		// Find the email configuration for this template
		templateFile := filepath.Base(req.TemplatePath)
		var emailConfig *struct {
			Name         string `yaml:"name"`
			File         string `yaml:"file"`
			EnvelopeFrom string `yaml:"envelope from"`
			From         string `yaml:"from"`
			Subject      string `yaml:"subject"`
		}

		for i := range emailData.Emails {
			if emailData.Emails[i].File == templateFile {
				emailConfig = &emailData.Emails[i]
				break
			}
		}

		if emailConfig == nil {
			writeErrorResponse(w, "Email configuration not found", http.StatusInternalServerError)
			return
		}

		// Read and process the email template
		content, err := os.ReadFile(fsPath)
		if err != nil {
			writeErrorResponse(w, "Failed to read template: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// Process the template content with variables for email
		processedContent := processTemplateContentForEmail(string(content), req.TemplatePath, baseDir, serverAddr)

		// Set default recipient
		to := "test@example.com"
		if req.To != "" {
			to = req.To
		}

		// Send the email
		err = sendSMTPEmail(emailConfig.From, to, emailConfig.Subject, processedContent)
		if err != nil {
			writeErrorResponse(w, "Failed to send email: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// Return success response
		response := SendEmailResponse{
			Success: true,
			Message: fmt.Sprintf("Email sent successfully to %s", to),
		}

		json.NewEncoder(w).Encode(response)
	}
}

// sendSMTPEmail sends an email via SMTP to Mailpit
func sendSMTPEmail(from, to, subject, htmlBody string) error {
	// Mailpit SMTP configuration
	smtpHost := "mailer" // Docker service name
	smtpPort := "1025"

	// Extract email address from "Name <email@domain.com>" format for SMTP commands
	fromEmail := extractEmailAddress(from)

	// Create message
	msg := fmt.Sprintf("From: %s\r\n", from)
	msg += fmt.Sprintf("To: %s\r\n", to)
	msg += fmt.Sprintf("Subject: %s\r\n", subject)
	msg += "MIME-Version: 1.0\r\n"
	msg += "Content-Type: text/html; charset=UTF-8\r\n"
	msg += "\r\n"
	msg += htmlBody

	// Connect to SMTP server
	conn, err := smtp.Dial(smtpHost + ":" + smtpPort)
	if err != nil {
		return fmt.Errorf("failed to connect to SMTP server: %v", err)
	}
	defer conn.Close()

	// Start TLS if available (Mailpit supports it)
	if ok, _ := conn.Extension("STARTTLS"); ok {
		config := &tls.Config{
			ServerName:         smtpHost,
			InsecureSkipVerify: true, // For development only
		}
		if err = conn.StartTLS(config); err != nil {
			// If TLS fails, continue without it (Mailpit accepts both)
		}
	}

	// Set sender (use extracted email address only)
	if err := conn.Mail(fromEmail); err != nil {
		return fmt.Errorf("failed to set sender: %v", err)
	}

	// Set recipient
	if err := conn.Rcpt(to); err != nil {
		return fmt.Errorf("failed to set recipient: %v", err)
	}

	// Send message
	w, err := conn.Data()
	if err != nil {
		return fmt.Errorf("failed to start data transfer: %v", err)
	}

	_, err = io.WriteString(w, msg)
	if err != nil {
		return fmt.Errorf("failed to write message: %v", err)
	}

	err = w.Close()
	if err != nil {
		return fmt.Errorf("failed to close data transfer: %v", err)
	}

	return nil
}

// extractEmailAddress extracts just the email address from formats like "Name <email@domain.com>"
func extractEmailAddress(emailString string) string {
	// Handle "Name <email@domain.com>" format
	if start := strings.Index(emailString, "<"); start != -1 {
		if end := strings.Index(emailString[start:], ">"); end != -1 {
			return emailString[start+1 : start+end]
		}
	}
	// Return original if no angle brackets found
	return emailString
}

// CheckEmailTemplateHandler returns whether a template is an email template
func CheckEmailTemplateHandler(baseDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		templatePath := r.URL.Query().Get("path")
		if templatePath == "" {
			writeErrorResponse(w, "No template path provided", http.StatusBadRequest)
			return
		}

		// Validate template path
		_, err := validatePath(baseDir, templatePath)
		if err != nil {
			writeErrorResponse(w, "Invalid template path", http.StatusBadRequest)
			return
		}

		// Check if this is an email template
		isEmail, emailData, err := IsEmailTemplate(baseDir, templatePath)
		if err != nil {
			writeErrorResponse(w, "Error checking template: "+err.Error(), http.StatusInternalServerError)
			return
		}

		response := map[string]interface{}{
			"isEmail": isEmail,
		}

		if isEmail && emailData != nil {
			// Find the specific email config for this template
			templateFile := filepath.Base(templatePath)
			for _, email := range emailData.Emails {
				if email.File == templateFile {
					response["emailConfig"] = map[string]string{
						"name":    email.Name,
						"from":    email.From,
						"subject": email.Subject,
					}
					break
				}
			}
		}

		json.NewEncoder(w).Encode(response)
	}
}

// processTemplateContentForEmail processes template content specifically for email sending
// Uses localhost URLs for working links and assets in Mailpit
func processTemplateContentForEmail(content, reqPath, baseDir, serverAddr string) string {
	// For emails viewed in Mailpit, use localhost and point to assets directory
	baseURL := fmt.Sprintf("http://localhost%s/templates/assets", serverAddr)

	// Use direct string replacement with rewritten URLs for email
	content = strings.Replace(content, "{{.BaseURL}}", baseURL, -1)
	content = strings.Replace(content, "{{.URL}}", fmt.Sprintf("http://localhost%s/raw/%s", serverAddr, strings.TrimSuffix(reqPath, filepath.Ext(reqPath))+".html"), -1)
	content = strings.Replace(content, "{{.TrackingURL}}", fmt.Sprintf("http://localhost%s/api/track/clicked/unique-id", serverAddr), -1)
	content = strings.Replace(content, "{{.Tracker}}", fmt.Sprintf(`<img src="http://localhost%s/api/track/opened/unique-id" alt="" width="1" height="1" border="0" style="height:1px !important;width:1px" />`, serverAddr), -1)

	// Replace other template variables with their original values
	for placeholder, value := range templateVars {
		if placeholder != "{{.BaseURL}}" && placeholder != "{{.URL}}" && placeholder != "{{.TrackingURL}}" && placeholder != "{{.Tracker}}" {
			content = strings.Replace(content, placeholder, value, -1)
		}
	}

	return content
}

// writeErrorResponse writes a JSON error response
func writeErrorResponse(w http.ResponseWriter, message string, statusCode int) {
	w.WriteHeader(statusCode)
	response := SendEmailResponse{
		Success: false,
		Message: message,
	}
	json.NewEncoder(w).Encode(response)
}
