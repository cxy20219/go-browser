package output

import (
	"fmt"
	"strings"

	"github.com/browserless/go-cli-browser/internal/snapshot"
)

// Formatter handles output formatting
type Formatter struct {
	Raw bool
}

// NewFormatter creates a new output formatter
func NewFormatter(raw bool) *Formatter {
	return &Formatter{Raw: raw}
}

// FormatSnapshot formats a snapshot for display
func (f *Formatter) FormatSnapshot(s *snapshot.Snapshot) string {
	if f.Raw {
		return formatSnapshotRaw(s)
	}
	return snapshot.FormatSnapshot(s)
}

// FormatPageStatus formats page status information
func (f *Formatter) FormatPageStatus(url, title string) string {
	if f.Raw {
		return url
	}
	return fmt.Sprintf("### Page\n- Page URL: %s\n- Page Title: %s\n", url, title)
}

// FormatError formats an error message
func (f *Formatter) FormatError(err error) string {
	if f.Raw {
		return err.Error()
	}
	return fmt.Sprintf("Error: %s", err.Error())
}

// FormatSuccess formats a success message
func (f *Formatter) FormatSuccess(msg string) string {
	if f.Raw {
		return msg
	}
	return fmt.Sprintf("Success: %s", msg)
}

// FormatList formats a list of items
func (f *Formatter) FormatList(items []string) string {
	if f.Raw {
		return strings.Join(items, "\n")
	}
	var sb strings.Builder
	for i, item := range items {
		sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, item))
	}
	return sb.String()
}

// FormatKeyValue formats key-value pairs
func (f *Formatter) FormatKeyValue(data map[string]string) string {
	if f.Raw {
		var sb strings.Builder
		for k, v := range data {
			sb.WriteString(fmt.Sprintf("%s=%s\n", k, v))
		}
		return sb.String()
	}
	var sb strings.Builder
	for k, v := range data {
		sb.WriteString(fmt.Sprintf("- %s: %s\n", k, v))
	}
	return sb.String()
}

func formatSnapshotRaw(s *snapshot.Snapshot) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("url: %s\n", s.URL))
	sb.WriteString(fmt.Sprintf("title: %s\n", s.Title))
	for _, elem := range s.Elements {
		sb.WriteString(fmt.Sprintf("%s %s\n", elem.Ref, elem.Tag))
	}
	return sb.String()
}
