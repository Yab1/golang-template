package notification

import (
	"bytes"
	"fmt"
	"html/template"
	"regexp"
	"strings"
	texttemplate "text/template"
)

func stringData(data map[string]any) map[string]string {
	out := map[string]string{}
	for k, v := range data {
		if v == nil {
			continue
		}
		out[k] = fmt.Sprint(v)
	}
	return out
}

var htmlTag = regexp.MustCompile(`<[^>]*>`)

func renderText(body string, data map[string]string) (string, error) {
	tpl, err := texttemplate.New("n").Option("missingkey=zero").Parse(body)
	if err != nil {
		return "", fmt.Errorf("template: %w", err)
	}
	var buf bytes.Buffer
	if err := tpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("template: %w", err)
	}
	return strings.TrimSpace(buf.String()), nil
}

func renderHTML(body string, data map[string]string) (string, error) {
	tpl, err := template.New("n").Option("missingkey=zero").Parse(body)
	if err != nil {
		return "", fmt.Errorf("template: %w", err)
	}
	var buf bytes.Buffer
	if err := tpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("template: %w", err)
	}
	return strings.TrimSpace(buf.String()), nil
}

func renderBody(format, body string, data map[string]any) (string, error) {
	fields := stringData(data)
	if format == FormatHTML {
		return renderHTML(body, fields)
	}
	return renderText(body, fields)
}

func plainFromHTML(body string) string {
	return strings.TrimSpace(htmlTag.ReplaceAllString(body, " "))
}

func fallbackTemplate(channel, category string) Template {
	subject := "Notice"
	switch category {
	case CategoryWelcome:
		subject = "Welcome"
	case CategoryAlert:
		subject = "Alert"
	case CategoryStatus:
		subject = "Status update"
	}
	format := FormatText
	body := "{{.message}}"
	if channel == ChannelEmail {
		format = FormatHTML
		body = "<p>{{.message}}</p>"
	}
	return Template{Channel: channel, Category: category, Locale: "en", Subject: subject, Body: body, Format: format, IsActive: true}
}
