package commands

import "github.com/playwright-community/playwright-go"

// floatPtr returns a pointer to a float64
func floatPtr(v int) *float64 {
	f := float64(v)
	return &f
}

// pageGotoOptions creates PageGotoOptions with timeout
func pageGotoOptions(timeout int) playwright.PageGotoOptions {
	return playwright.PageGotoOptions{
		Timeout: floatPtr(timeout),
	}
}

// locatorClickOptions creates LocatorClickOptions with timeout
func locatorClickOptions(timeout int) playwright.LocatorClickOptions {
	return playwright.LocatorClickOptions{
		Timeout: floatPtr(timeout),
	}
}

// locatorDblclickOptions creates LocatorDblclickOptions with timeout
func locatorDblclickOptions(timeout int) playwright.LocatorDblclickOptions {
	return playwright.LocatorDblclickOptions{
		Timeout: floatPtr(timeout),
	}
}

// locatorHoverOptions creates LocatorHoverOptions with timeout
func locatorHoverOptions(timeout int) playwright.LocatorHoverOptions {
	return playwright.LocatorHoverOptions{
		Timeout: floatPtr(timeout),
	}
}

// locatorCheckOptions creates LocatorCheckOptions with timeout
func locatorCheckOptions(timeout int) playwright.LocatorCheckOptions {
	return playwright.LocatorCheckOptions{
		Timeout: floatPtr(timeout),
	}
}

// locatorUncheckOptions creates LocatorUncheckOptions with timeout
func locatorUncheckOptions(timeout int) playwright.LocatorUncheckOptions {
	return playwright.LocatorUncheckOptions{
		Timeout: floatPtr(timeout),
	}
}

// keyboardTypeOptions creates KeyboardTypeOptions with delay
func keyboardTypeOptions(delay int) playwright.KeyboardTypeOptions {
	return playwright.KeyboardTypeOptions{
		Delay: floatPtr(delay),
	}
}

// locatorFillOptions creates LocatorFillOptions
func locatorFillOptions() playwright.LocatorFillOptions {
	return playwright.LocatorFillOptions{}
}

// locatorPressOptions creates LocatorPressOptions
func locatorPressOptions() playwright.LocatorPressOptions {
	return playwright.LocatorPressOptions{}
}

// locatorSelectOptionOptions creates LocatorSelectOptionOptions
func locatorSelectOptionOptions() playwright.LocatorSelectOptionOptions {
	return playwright.LocatorSelectOptionOptions{}
}

// locatorDragToOptions creates LocatorDragToOptions
func locatorDragToOptions() playwright.LocatorDragToOptions {
	return playwright.LocatorDragToOptions{}
}
