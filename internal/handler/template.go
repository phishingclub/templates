package handler

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"html"
	"html/template"
	"io"
	"math/rand"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-errors/errors"
	"github.com/yeqown/go-qrcode/v2"
)

const alphaChar = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

// Template functions for use in HTML templates
var TemplateFuncs = template.FuncMap{
	// Original template functions
	"split": func(s, sep string) []string {
		return strings.Split(s, sep)
	},
	"join": func(base, add, sep string) string {
		if base == "" {
			return add
		}
		return base + sep + add
	},
	"basename": func(path string) string {
		return filepath.Base(path)
	},
	"ext": func(path string) string {
		return filepath.Ext(path)
	},
	"dict": func(values ...interface{}) (map[string]interface{}, error) {
		if len(values)%2 != 0 {
			return nil, fmt.Errorf("dict needs an even number of arguments")
		}
		dict := make(map[string]interface{}, len(values)/2)
		for i := 0; i < len(values); i += 2 {
			key, ok := values[i].(string)
			if !ok {
				return nil, fmt.Errorf("dict keys must be strings")
			}
			dict[key] = values[i+1]
		}
		return dict, nil
	},

	// Platform template functions (from templateService.go)
	"urlEscape": func(s string) string {
		return template.URLQueryEscaper(s)
	},
	"htmlEscape": func(s string) string {
		return html.EscapeString(s)
	},
	"randInt": func(n1, n2 int) (int, error) {
		if n1 > n2 {
			return 0, fmt.Errorf("first number must be less than or equal to second number")
		}
		// #nosec
		return rand.Intn(n2-n1+1) + n1, nil
	},
	"randAlpha": RandAlpha,
	"qr":        GenerateQRCode,
	"date": func(format string, offsetSeconds ...int) string {
		offset := 0
		if len(offsetSeconds) > 0 {
			offset = offsetSeconds[0]
		}
		targetTime := time.Now().Add(time.Duration(offset) * time.Second)
		goFormat := convertDateFormat(format)
		return targetTime.Format(goFormat)
	},
	"base64": func(s string) string {
		return base64.StdEncoding.EncodeToString([]byte(s))
	},
}

// GenerateQRCode generates a QR code as HTML table
func GenerateQRCode(args ...any) (template.HTML, error) {
	if len(args) == 0 {
		return "", errors.New("URL is required")
	}

	url, ok := args[0].(string)
	if !ok {
		return "", errors.New("first argument must be a URL string")
	}

	dotSize := 5
	if len(args) > 1 {
		if size, ok := args[1].(int); ok && size > 0 {
			dotSize = size
		}
	}

	var buf bytes.Buffer
	qr, err := qrcode.New(url)
	if err != nil {
		return "", err
	}

	writer := NewQRHTMLWriter(&buf, dotSize)
	if err := qr.Save(writer); err != nil {
		return "", err
	}
	// #nosec
	return template.HTML(buf.String()), nil
}

// RandAlpha returns a random string of the given length
func RandAlpha(length int) (string, error) {
	if length > 32 {
		return "", fmt.Errorf("length must be less than 32")
	}
	b := make([]byte, length)
	for i := range b {
		// #nosec
		b[i] = alphaChar[rand.Intn(len(alphaChar))]
	}
	return string(b), nil
}

// QRHTMLWriter generates QR codes as HTML tables
type QRHTMLWriter struct {
	w       io.Writer
	dotSize int
}

// NewQRHTMLWriter creates a new QR HTML writer
func NewQRHTMLWriter(w io.Writer, dotSize int) *QRHTMLWriter {
	if dotSize <= 0 {
		dotSize = 10
	}
	return &QRHTMLWriter{
		w:       w,
		dotSize: dotSize,
	}
}

// Write writes the QR matrix as an HTML table
func (q *QRHTMLWriter) Write(mat qrcode.Matrix) error {
	if q.w == nil {
		return errors.New("QR writer: writer not initialized")
	}

	if _, err := fmt.Fprint(q.w, `<table cellpadding="0" cellspacing="0" border="0" style="border-collapse: collapse;">`); err != nil {
		return fmt.Errorf("failed to write table opening: %w", err)
	}

	maxW := mat.Width() - 1
	mat.Iterate(qrcode.IterDirection_ROW, func(x, y int, v qrcode.QRValue) {
		if x == 0 {
			fmt.Fprint(q.w, "<tr>")
		}

		color := "#FFFFFF"
		if v.IsSet() {
			color = "#000000"
		}

		fmt.Fprintf(q.w, `<td width="%d" height="%d" bgcolor="%s" style="padding:0; margin:0; font-size:0; line-height:0; width:%dpx; height:%dpx; min-width:%dpx; min-height:%dpx; "></td>`,
			q.dotSize, q.dotSize, color, q.dotSize, q.dotSize, q.dotSize, q.dotSize)

		if x == maxW {
			fmt.Fprint(q.w, "</tr>")
		}
	})

	if _, err := fmt.Fprint(q.w, "</table>"); err != nil {
		return fmt.Errorf("QR writer: failed to write table closing: %w", err)
	}

	return nil
}

// Close closes the writer if it implements io.Closer
func (q *QRHTMLWriter) Close() error {
	if closer, ok := q.w.(io.Closer); ok {
		return closer.Close()
	}
	return nil
}

// convertDateFormat converts readable date format (YmdHis) to Go's reference format
func convertDateFormat(dateFormat string) string {
	goFormat := dateFormat

	// year formats
	goFormat = strings.ReplaceAll(goFormat, "Y", "2006") // 4-digit year
	goFormat = strings.ReplaceAll(goFormat, "y", "06")   // 2-digit year

	// month formats
	goFormat = strings.ReplaceAll(goFormat, "m", "01")      // 2-digit month
	goFormat = strings.ReplaceAll(goFormat, "n", "1")       // month without leading zero
	goFormat = strings.ReplaceAll(goFormat, "M", "Jan")     // short month name
	goFormat = strings.ReplaceAll(goFormat, "F", "January") // full month name

	// day formats
	goFormat = strings.ReplaceAll(goFormat, "d", "02") // 2-digit day
	goFormat = strings.ReplaceAll(goFormat, "j", "2")  // day without leading zero

	// hour formats
	goFormat = strings.ReplaceAll(goFormat, "H", "15") // 24-hour format
	goFormat = strings.ReplaceAll(goFormat, "h", "03") // 12-hour format
	goFormat = strings.ReplaceAll(goFormat, "G", "15") // 24-hour without leading zero (Go doesn't support this exactly)
	goFormat = strings.ReplaceAll(goFormat, "g", "3")  // 12-hour without leading zero

	// minute and second formats
	goFormat = strings.ReplaceAll(goFormat, "i", "04") // minutes
	goFormat = strings.ReplaceAll(goFormat, "s", "05") // seconds

	// am/pm formats
	goFormat = strings.ReplaceAll(goFormat, "A", "PM") // uppercase AM/PM
	goFormat = strings.ReplaceAll(goFormat, "a", "pm") // lowercase am/pm

	return goFormat
}

// InitTemplates initializes templates with the required functions
func InitTemplates(tmpl *template.Template) *template.Template {
	return tmpl.Funcs(TemplateFuncs)
}
