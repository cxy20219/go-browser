package snapshot

import (
	"fmt"
	"strings"
	"time"

	"github.com/playwright-community/playwright-go"
)

// Snapshot represents a page snapshot with element refs
type Snapshot struct {
	URL       string       `json:"url" yaml:"url"`
	Title     string       `json:"title" yaml:"title"`
	Timestamp time.Time    `json:"timestamp" yaml:"timestamp"`
	Elements  []ElementRef `json:"elements" yaml:"elements"`
}

// ElementRef represents a reference to a page element
type ElementRef struct {
	Ref     string `json:"ref" yaml:"ref"`
	Tag     string `json:"tag" yaml:"tag"`
	Text    string `json:"text,omitempty" yaml:"text,omitempty"`
	HTML    string `json:"html,omitempty" yaml:"html,omitempty"`
	Visible bool   `json:"visible" yaml:"visible"`
	// Selector is used internally to resolve snapshot refs back to elements.
	Selector string `json:"selector,omitempty" yaml:"selector,omitempty"`
	// Additional attributes for identification
	Role        string `json:"role,omitempty" yaml:"role,omitempty"`
	Name        string `json:"name,omitempty" yaml:"name,omitempty"`
	TestID      string `json:"testId,omitempty" yaml:"testId,omitempty"`
	Type        string `json:"type,omitempty" yaml:"type,omitempty"`
	Placeholder string `json:"placeholder,omitempty" yaml:"placeholder,omitempty"`
}

// Generator generates element references
type Generator struct {
	counter int
}

// NewGenerator creates a new ref generator
func NewGenerator() *Generator {
	return &Generator{counter: 0}
}

// Next generates the next element ref (e0, e1, ..., e9, ea, eb, ...)
func (g *Generator) Next() string {
	ref := fmt.Sprintf("e%d", g.counter)
	g.counter++
	return ref
}

// Reset resets the counter
func (g *Generator) Reset() {
	g.counter = 0
}

// GenerateSnapshot generates a snapshot of the page
func GenerateSnapshot(page playwright.Page, depth int) (*Snapshot, error) {
	url := page.URL()

	title, err := page.Title()
	if err != nil {
		return nil, fmt.Errorf("failed to get page title: %w", err)
	}

	snapshot := &Snapshot{
		URL:       url,
		Title:     title,
		Timestamp: time.Now(),
		Elements:  make([]ElementRef, 0),
	}

	// Get element info using JavaScript
	elements, err := getElementInfo(page, depth)
	if err != nil {
		return nil, fmt.Errorf("failed to get element info: %w", err)
	}

	snapshot.Elements = elements
	return snapshot, nil
}

// getElementInfo extracts element information from the page
func getElementInfo(page playwright.Page, depth int) ([]ElementRef, error) {
	script := fmt.Sprintf(`
		() => {
			const maxDepth = %d;
			const visibleOnly = true;
			const excludeTags = ['SCRIPT', 'STYLE', 'NOSCRIPT', 'IFRAME', 'OBJECT', 'EMBED'];
			const interactiveTags = ['A', 'BUTTON', 'INPUT', 'SELECT', 'TEXTAREA'];
			const refAttr = 'data-go-browser-ref';
			let refCounter = 0;

			window.__goBrowserRefToElement = {};
			document.querySelectorAll('[' + refAttr + ']').forEach(el => {
				el.removeAttribute(refAttr);
			});

			function cssEscape(value) {
				if (window.CSS && typeof window.CSS.escape === 'function') {
					return window.CSS.escape(value);
				}
				return String(value).replace(/["\\]/g, '\\$&');
			}

			function nextRef(element) {
				const ref = 'e' + refCounter++;
				element.setAttribute(refAttr, ref);
				window.__goBrowserRefToElement[ref] = element;
				return ref;
			}

			function refSelector(ref) {
				return '[' + refAttr + '="' + cssEscape(ref) + '"]';
			}

			function getVisible(element) {
				const style = window.getComputedStyle(element);
				const rect = element.getBoundingClientRect();

				if (excludeTags.includes(element.tagName)) return false;
				if (visibleOnly && (style.display === 'none' || style.visibility === 'hidden')) return false;
				if (rect.width === 0 || rect.height === 0) return false;

				return true;
			}

			function getAttributes(el) {
				const attrs = {};
				for (const attr of el.attributes) {
					attrs[attr.name] = attr.value;
				}
				return attrs;
			}

			function traverse(root, currentDepth) {
				if (currentDepth > maxDepth) return [];

				const results = [];
				for (const node of Array.from(root.children)) {
					if (!getVisible(node)) continue;

					const tag = node.tagName.toLowerCase();
					const attrs = getAttributes(node);
					const role = attrs['role'] || '';
					const testId = attrs['data-testid'] || attrs['data-test-id'] || '';
					const name = node.getAttribute('name') ||
						node.getAttribute('aria-label') ||
						node.getAttribute('aria-labelledby') || '';
					const type = attrs['type'] || '';
					const placeholder = node.getAttribute('placeholder') || '';
					const isInteractive = interactiveTags.includes(tag) || role;

					if (isInteractive || testId || node.children.length === 0) {
						const ref = nextRef(node);
						const text = node.innerText?.trim().substring(0, 100) || '';
						const html = node.outerHTML?.substring(0, 200) || '';

						results.push({
							ref: ref,
							tag: tag,
							text: text,
							html: html,
							visible: true,
							selector: refSelector(ref),
							role: role,
							name: name,
							testId: testId,
							type: type,
							placeholder: placeholder
						});
					}

					results.push(...traverse(node, currentDepth + 1));
					if (results.length >= 200) break;
				}

				return results.slice(0, 200); // Limit to 200 elements
			}

			return traverse(document.body, 0);
		}
	`, depth)

	result, err := page.Evaluate(script)
	if err != nil {
		return nil, err
	}

	elementsData, ok := result.([]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected result type from page evaluation")
	}

	elements := make([]ElementRef, 0, len(elementsData))

	for _, e := range elementsData {
		elem, ok := e.(map[string]interface{})
		if !ok {
			continue
		}

		elementRef := ElementRef{
			Visible: true,
		}

		if ref, ok := elem["ref"].(string); ok {
			elementRef.Ref = ref
		}
		if tag, ok := elem["tag"].(string); ok {
			elementRef.Tag = tag
		}
		if text, ok := elem["text"].(string); ok {
			elementRef.Text = text
		}
		if html, ok := elem["html"].(string); ok {
			elementRef.HTML = html
		}
		if role, ok := elem["role"].(string); ok {
			elementRef.Role = role
		}
		if name, ok := elem["name"].(string); ok {
			elementRef.Name = name
		}
		if testId, ok := elem["testId"].(string); ok {
			elementRef.TestID = testId
		}
		if typ, ok := elem["type"].(string); ok {
			elementRef.Type = typ
		}
		if placeholder, ok := elem["placeholder"].(string); ok {
			elementRef.Placeholder = placeholder
		}
		if visible, ok := elem["visible"].(bool); ok {
			elementRef.Visible = visible
		}
		if selector, ok := elem["selector"].(string); ok {
			elementRef.Selector = selector
		}

		elements = append(elements, elementRef)
	}

	return elements, nil
}

// FormatSnapshot formats a snapshot for display
func FormatSnapshot(s *Snapshot) string {
	var sb strings.Builder

	sb.WriteString("### Page\n")
	sb.WriteString(fmt.Sprintf("- Page URL: %s\n", s.URL))
	sb.WriteString(fmt.Sprintf("- Page Title: %s\n", s.Title))
	sb.WriteString(fmt.Sprintf("- Timestamp: %s\n", s.Timestamp.Format(time.RFC3339)))
	sb.WriteString("\n### Elements\n")

	for _, elem := range s.Elements {
		sb.WriteString(fmt.Sprintf("[%s] %s", elem.Ref, elem.Tag))
		if elem.Role != "" {
			sb.WriteString(fmt.Sprintf(" role=%q", elem.Role))
		}
		if elem.Name != "" {
			sb.WriteString(fmt.Sprintf(" name=%q", elem.Name))
		}
		if elem.TestID != "" {
			sb.WriteString(fmt.Sprintf(" testId=%q", elem.TestID))
		}
		if elem.Type != "" {
			sb.WriteString(fmt.Sprintf(" type=%q", elem.Type))
		}
		if elem.Placeholder != "" {
			sb.WriteString(fmt.Sprintf(" placeholder=%q", elem.Placeholder))
		}
		if elem.Text != "" {
			sb.WriteString(fmt.Sprintf(" %q", truncate(elem.Text, 50)))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
