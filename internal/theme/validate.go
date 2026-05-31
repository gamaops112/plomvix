package theme

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

func Validate(t *Theme) error {
	var errs []string

	if t.Version != 1 {
		errs = append(errs, fmt.Sprintf("version must be 1, got: %d", t.Version))
	}
	if t.Mode != "light" && t.Mode != "dark" {
		errs = append(errs, fmt.Sprintf(`mode must be "light" or "dark", got: %q`, t.Mode))
	}

	validateColors("colors", t.Tokens.Colors, &errs)
	validateColors("dark_colors", t.Tokens.DarkColors, &errs)
	validateTypography(t.Tokens.Typography, &errs)
	validateScale("radii", t.Tokens.Radii, false, &errs)
	validateScale("spacing", t.Tokens.Spacing, true, &errs)
	validateShadows(t.Tokens.Shadows, &errs)
	validateLayout(t.Tokens.Layout, &errs)

	if len(errs) > 0 {
		return fmt.Errorf("plomvix theme validation failed:\n  - %s", strings.Join(errs, "\n  - "))
	}
	return nil
}

var hexColorRE = regexp.MustCompile(`^#[0-9a-fA-F]{3}([0-9a-fA-F]{3})?$`)
var cssLengthRE = regexp.MustCompile(`^\d+(\.\d+)?\s*(px|rem|em)$`)
var cssDurationRE = regexp.MustCompile(`^\d+(\.\d+)?\s*(ms|s)$`)

func isHexColor(s string) bool {
	return hexColorRE.MatchString(strings.TrimSpace(s))
}

func isCSSLength(s string) bool {
	return cssLengthRE.MatchString(strings.TrimSpace(s))
}

func isCSSDuration(s string) bool {
	return cssDurationRE.MatchString(strings.TrimSpace(s))
}

func isFontWeight(s string) bool {
	w, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return false
	}
	return w >= 100 && w <= 900
}

func validateColors(prefix string, c ColorTokens, errs *[]string) {
	fields := map[string]string{
		"primary": c.Primary, "secondary": c.Secondary, "accent": c.Accent,
		"background": c.Background, "background_muted": c.BackgroundMuted,
		"surface": c.Surface, "surface_muted": c.SurfaceMuted,
		"text": c.Text, "text_muted": c.TextMuted, "border": c.Border,
		"error": c.Error, "warning": c.Warning, "success": c.Success, "info": c.Info,
	}
	for name, val := range fields {
		if strings.TrimSpace(val) == "" {
			*errs = append(*errs, fmt.Sprintf("tokens.%s.%s must not be empty", prefix, name))
		} else if !isHexColor(val) {
			*errs = append(*errs, fmt.Sprintf("tokens.%s.%s must be a valid hex color, got: %q", prefix, name, val))
		}
	}
}

func validateTypography(t TypographyTokens, errs *[]string) {
	if strings.TrimSpace(t.FontFamily) == "" {
		*errs = append(*errs, "tokens.typography.font_family must not be empty")
	}
	sizes := map[string]string{
		"font_size_xs": t.FontSizeXS, "font_size_sm": t.FontSizeSM,
		"font_size_base": t.FontSizeBase, "font_size_lg": t.FontSizeLG,
		"font_size_xl": t.FontSizeXL,
	}
	for name, val := range sizes {
		if !isCSSLength(val) {
			*errs = append(*errs, fmt.Sprintf("tokens.typography.%s must be a valid CSS length, got: %q", name, val))
		}
	}
	weights := map[string]string{
		"font_weight_regular": t.FontWeightRegular, "font_weight_medium": t.FontWeightMedium,
		"font_weight_semibold": t.FontWeightSemibold, "font_weight_bold": t.FontWeightBold,
	}
	for name, val := range weights {
		if !isFontWeight(val) {
			*errs = append(*errs, fmt.Sprintf("tokens.typography.%s must be a valid font weight (100-900), got: %q", name, val))
		}
	}
}

func validateScale(prefix string, s ScaleTokens, requireXS bool, errs *[]string) {
	fields := map[string]string{"sm": s.SM, "md": s.MD, "lg": s.LG, "xl": s.XL}
	if requireXS {
		fields["xs"] = s.XS
	}
	for name, val := range fields {
		if !isCSSLength(val) {
			*errs = append(*errs, fmt.Sprintf("tokens.%s.%s must be a valid CSS length, got: %q", prefix, name, val))
		}
	}
}

func validateShadows(s ShadowTokens, errs *[]string) {
	fields := map[string]string{"sm": s.SM, "md": s.MD, "lg": s.LG}
	for name, val := range fields {
		if strings.TrimSpace(val) == "" {
			*errs = append(*errs, fmt.Sprintf("tokens.shadows.%s must not be empty", name))
		}
	}
}

func validateLayout(l LayoutTokens, errs *[]string) {
	if !isCSSLength(l.SidebarWidth) {
		*errs = append(*errs, fmt.Sprintf("tokens.layout.sidebar_width must be a valid CSS length, got: %q", l.SidebarWidth))
	}
	if !isCSSLength(l.NavbarHeight) {
		*errs = append(*errs, fmt.Sprintf("tokens.layout.navbar_height must be a valid CSS length, got: %q", l.NavbarHeight))
	}
	if !isCSSDuration(l.TransitionSpeed) {
		*errs = append(*errs, fmt.Sprintf("tokens.layout.transition_speed must be a valid CSS duration, got: %q", l.TransitionSpeed))
	}
}
