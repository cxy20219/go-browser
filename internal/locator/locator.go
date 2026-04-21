package locator

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/browserless/go-cli-browser/internal/snapshot"
	"github.com/playwright-community/playwright-go"
)

// Resolver resolves refs and selectors to Playwright locators
type Resolver struct {
	page playwright.Page
}

// NewResolver creates a new locator resolver
func NewResolver(page playwright.Page) *Resolver {
	return &Resolver{page: page}
}

// Resolve resolves a string to a Playwright locator
// Supports: refs (e0, e1, ...), CSS selectors, role locators, test ids
func (r *Resolver) Resolve(locatorStr string) (playwright.Locator, error) {
	trimmed := strings.TrimSpace(locatorStr)

	// Check if it's a ref
	if snapshot.IsRef(trimmed) {
		return r.resolveRef(trimmed)
	}

	// Check if it's a role locator string
	if isRoleLocator(trimmed) {
		return r.page.Locator(trimmed), nil
	}

	// Check if it's a getByTestId
	if strings.HasPrefix(trimmed, "getByTestId") {
		return r.page.Locator(trimmed), nil
	}

	// Treat as CSS selector
	return r.page.Locator(trimmed), nil
}

// isRoleLocator checks if the string is a role locator
func isRoleLocator(s string) bool {
	rolePatterns := []string{
		`^getByRole\(`,
		`^getByText\(`,
		`^getByLabel\(`,
		`^getByPlaceholder\(`,
		`^getByAltText\(`,
		`^getByTitle\(`,
		`^getByTestId\(`,
	}
	for _, pattern := range rolePatterns {
		match, _ := regexp.MatchString(pattern, s)
		if match {
			return true
		}
	}
	return false
}

// resolveRef resolves a ref to a locator
func (r *Resolver) resolveRef(ref string) (playwright.Locator, error) {
	// Use JavaScript to find the element by ref
	script := `
		(ref) => {
			const cssEscape = (value) => {
				if (window.CSS && typeof window.CSS.escape === 'function') {
					return window.CSS.escape(value);
				}
				return String(value).replace(/["\\]/g, '\\$&');
			};
			const refSelector = '[data-go-browser-ref="' + cssEscape(ref) + '"]';
			const el = (window.__goBrowserRefToElement && window.__goBrowserRefToElement[ref]) ||
				document.querySelector(refSelector);
			if (el) {
				if (el.getAttribute('data-go-browser-ref') === ref) return refSelector;
				if (el.id) return '#' + cssEscape(el.id);
				let path = '';
				let current = el;
				while (current && current !== document.body) {
					let selector = current.tagName.toLowerCase();
					if (current.id) {
						selector = '#' + current.id;
						path = selector + (path ? ' > ' + path : '');
						break;
					}
					if (current.className) {
						const classes = Array.from(current.classList).slice(0, 2).join('.');
						if (classes) selector += '.' + classes;
					}
					path = selector + (path ? ' > ' + path : '');
					current = current.parentElement;
				}
				return path || current.tagName.toLowerCase();
			}
			return null;
		}
	`

	result, err := r.page.Evaluate(script, ref)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve ref %s: %w", ref, err)
	}

	if result == nil {
		return nil, fmt.Errorf("ref %s not found or element not available", ref)
	}

	selector, ok := result.(string)
	if !ok {
		return nil, fmt.Errorf("invalid selector for ref %s", ref)
	}

	return r.page.Locator(selector), nil
}
