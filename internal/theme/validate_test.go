package theme

import (
	"testing"
)

func validTheme() *Theme {
	t := DefaultTheme()
	return t
}

func TestValidateValidDefaultTheme(t *testing.T) {
	if err := Validate(validTheme()); err != nil {
		t.Fatalf("expected valid default theme, got: %v", err)
	}
}

func TestValidateInvalidVersion(t *testing.T) {
	tm := validTheme()
	tm.Version = 2
	if err := Validate(tm); err == nil {
		t.Fatal("expected error for invalid version, got nil")
	}
}

func TestValidateInvalidMode(t *testing.T) {
	tm := validTheme()
	tm.Mode = "system"
	if err := Validate(tm); err == nil {
		t.Fatal("expected error for invalid mode, got nil")
	}
}

func TestValidateInvalidHexColor(t *testing.T) {
	tm := validTheme()
	tm.Tokens.Colors.Primary = "not-a-color"
	if err := Validate(tm); err == nil {
		t.Fatal("expected error for invalid hex color, got nil")
	}
}

func TestValidateEmptyRequiredColor(t *testing.T) {
	tm := validTheme()
	tm.Tokens.Colors.Primary = ""
	if err := Validate(tm); err == nil {
		t.Fatal("expected error for empty required color, got nil")
	}
}

func TestValidateInvalidCSSLength(t *testing.T) {
	tm := validTheme()
	tm.Tokens.Spacing.MD = "10"
	if err := Validate(tm); err == nil {
		t.Fatal("expected error for invalid CSS length, got nil")
	}
}

func TestValidateInvalidCSSDuration(t *testing.T) {
	tm := validTheme()
	tm.Tokens.Layout.TransitionSpeed = "160"
	if err := Validate(tm); err == nil {
		t.Fatal("expected error for invalid CSS duration, got nil")
	}
}

func TestValidateInvalidFontWeight(t *testing.T) {
	tm := validTheme()
	tm.Tokens.Typography.FontWeightBold = "bold"
	if err := Validate(tm); err == nil {
		t.Fatal("expected error for invalid font weight, got nil")
	}
}

func TestValidateEmptyShadow(t *testing.T) {
	tm := validTheme()
	tm.Tokens.Shadows.MD = ""
	if err := Validate(tm); err == nil {
		t.Fatal("expected error for empty shadow, got nil")
	}
}

func TestValidateMultipleErrors(t *testing.T) {
	tm := validTheme()
	tm.Version = 99
	tm.Mode = "invalid"
	if err := Validate(tm); err == nil {
		t.Fatal("expected error for multiple invalid fields, got nil")
	}
}
