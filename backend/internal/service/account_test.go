package service

import (
	"testing"
)

// ─── hexColorRegex ────────────────────────────────────────────────────────────

func TestHexColor_Valid(t *testing.T) {
	valid := []string{
		"#6366F1",
		"#F59E0B",
		"#000000",
		"#FFFFFF",
		"#ff0000",
		"#aabbcc",
		"#1A2B3C",
	}
	for _, c := range valid {
		if !hexColorRegex.MatchString(c) {
			t.Errorf("expected %q to be a valid hex color", c)
		}
	}
}

func TestHexColor_Invalid(t *testing.T) {
	invalid := []string{
		"#ZZZZZZ",   // non-hex chars
		"#12345",    // 5 digits
		"#1234567",  // 7 digits
		"123456",    // missing #
		"",          // empty
		"#",         // only hash
		"#GGGGGG",   // G is not a hex digit
		"#6366F1 ",  // trailing space
		" #6366F1",  // leading space
	}
	for _, c := range invalid {
		if hexColorRegex.MatchString(c) {
			t.Errorf("expected %q to be an INVALID hex color, but it matched", c)
		}
	}
}

// ─── UpdateBranding color validation (unit-level) ─────────────────────────────
//
// These tests directly verify the validation logic from UpdateBranding without
// needing a DB connection, by re-running the same regex check.

func TestBrandingColorValidation_PrimaryValid(t *testing.T) {
	colors := []string{"#6366F1", "#000000", "#abcdef"}
	for _, c := range colors {
		if !hexColorRegex.MatchString(c) {
			t.Errorf("primary color %q should pass validation", c)
		}
	}
}

func TestBrandingColorValidation_PrimaryInvalid(t *testing.T) {
	colors := []string{"#ZZZZZZ", "6366F1", "#6366F", "#GGG"}
	for _, c := range colors {
		if hexColorRegex.MatchString(c) {
			t.Errorf("primary color %q should FAIL validation", c)
		}
	}
}
