package snapshot

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/playwright-community/playwright-go"
)

var refPattern = regexp.MustCompile(`^e[a-z0-9]{1,4}$`)

// RefCache stores mappings from refs to CSS selectors
type RefCache struct {
	elements map[string]ElementRef
}

// NewRefCache creates a new ref cache
func NewRefCache() *RefCache {
	return &RefCache{
		elements: make(map[string]ElementRef),
	}
}

// Set stores an element ref
func (c *RefCache) Set(ref string, elem ElementRef) {
	c.elements[ref] = elem
}

// Get retrieves an element ref
func (c *RefCache) Get(ref string) (ElementRef, bool) {
	elem, ok := c.elements[ref]
	return elem, ok
}

// Clear clears all cached refs
func (c *RefCache) Clear() {
	c.elements = make(map[string]ElementRef)
}

// BuildFromSnapshot builds a ref cache from a snapshot
func (c *RefCache) BuildFromSnapshot(snapshot *Snapshot) {
	c.Clear()
	for _, elem := range snapshot.Elements {
		c.elements[elem.Ref] = elem
	}
}

// Selector returns the cached CSS selector for a ref.
func (c *RefCache) Selector(ref string) (string, bool) {
	elem, ok := c.Get(ref)
	if !ok || elem.Selector == "" {
		return "", false
	}
	return elem.Selector, true
}

// IsRef checks if the string looks like a snapshot ref (e0, e1, ...).
func IsRef(s string) bool {
	return refPattern.MatchString(strings.TrimSpace(s))
}

// GenerateCSSPath generates a unique CSS selector for an element
func GenerateCSSPath(page playwright.Page, ref string) (string, error) {
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
			if (!el) return null;

			if (el.getAttribute('data-go-browser-ref') === ref) {
				return refSelector;
			}

			if (el.id) {
				return '#' + cssEscape(el.id);
			}

			let path = '';
			let current = el;
			while (current && current !== document.body) {
				let selector = current.tagName.toLowerCase();

				if (current.id) {
					selector = '#' + current.id;
					path = selector + ' ' + path;
					break;
				}

				// Add class name if available
				const classes = Array.from(current.classList)
					.filter(c => c && !c.match(/^(js-|is-|has-)/))
					.slice(0, 2);
				if (classes.length > 0) {
					selector += '.' + classes.join('.');
				}

				// Add nth-child if needed
				const parent = current.parentElement;
				if (parent) {
					const siblings = Array.from(parent.children).filter(
						c => c.tagName === current.tagName
					);
					if (siblings.length > 1) {
						const index = siblings.indexOf(current) + 1;
						selector += ':nth-child(' + index + ')';
					}
				}

				path = selector + ' ' + path;
				current = current.parentElement;
			}

			return path.trim();
		}
	`

	result, err := page.Evaluate(script, ref)
	if err != nil {
		return "", err
	}

	if result == nil {
		return "", fmt.Errorf("element ref %q not found", ref)
	}

	return result.(string), nil
}

// ResolveRefToSelector resolves a ref to a CSS selector
func ResolveRefToSelector(page playwright.Page, ref string) (string, error) {
	// First try to generate a unique CSS path
	cssPath, err := GenerateCSSPath(page, ref)
	if err != nil {
		return "", err
	}
	return cssPath, nil
}

// ParseLocator parses a locator string and returns the appropriate locator
func ParseLocator(page playwright.Page, locatorStr string) (playwright.Locator, error) {
	// Check if it's a ref
	if IsRef(locatorStr) {
		// This is likely a ref, try to generate a CSS path
		cssPath, err := ResolveRefToSelector(page, locatorStr)
		if err == nil && cssPath != "" {
			return page.Locator(cssPath), nil
		}
	}

	// Check if it's a CSS selector (contains #, ., or starts with [)
	if strings.Contains(locatorStr, "#") ||
		strings.Contains(locatorStr, ".") ||
		strings.HasPrefix(locatorStr, "[") ||
		strings.HasPrefix(locatorStr, " ") {
		return page.Locator(locatorStr), nil
	}

	// Check if it's a role locator (starts with getBy)
	if strings.HasPrefix(locatorStr, "getByRole") ||
		strings.HasPrefix(locatorStr, "getByText") ||
		strings.HasPrefix(locatorStr, "getByLabel") ||
		strings.HasPrefix(locatorStr, "getByPlaceholder") ||
		strings.HasPrefix(locatorStr, "getByAltText") ||
		strings.HasPrefix(locatorStr, "getByTitle") ||
		strings.HasPrefix(locatorStr, "getByTestId") ||
		strings.HasPrefix(locatorStr, "getByRole(") {
		return page.Locator(locatorStr), nil
	}

	// Default: treat as CSS selector
	return page.Locator(locatorStr), nil
}
