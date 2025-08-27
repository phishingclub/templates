package handler

import (
	"fmt"
	"html/template"
	"path/filepath"
	"strings"
)

// Template functions for use in HTML templates
var TemplateFuncs = template.FuncMap{
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
}

// InitTemplates initializes templates with the required functions
func InitTemplates(tmpl *template.Template) *template.Template {
	return tmpl.Funcs(TemplateFuncs)
}