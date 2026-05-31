package theme

// Theme is the complete persisted Plomvix design-token document.
type Theme struct {
	Version  int    `json:"version"`
	DevPanel bool   `json:"dev_panel"`
	Mode     string `json:"mode"`
	Tokens   Tokens `json:"tokens"`
}

// Tokens groups all design-token categories.
type Tokens struct {
	Colors     ColorTokens      `json:"colors"`
	DarkColors ColorTokens      `json:"dark_colors"`
	Typography TypographyTokens `json:"typography"`
	Radii      ScaleTokens      `json:"radii"`
	Spacing    ScaleTokens      `json:"spacing"`
	Shadows    ShadowTokens     `json:"shadows"`
	Layout     LayoutTokens     `json:"layout"`
}

// ColorTokens contains semantic color tokens.
type ColorTokens struct {
	Primary         string `json:"primary"`
	Secondary       string `json:"secondary"`
	Accent          string `json:"accent"`
	Background      string `json:"background"`
	BackgroundMuted string `json:"background_muted"`
	Surface         string `json:"surface"`
	SurfaceMuted    string `json:"surface_muted"`
	Text            string `json:"text"`
	TextMuted       string `json:"text_muted"`
	Border          string `json:"border"`
	Error           string `json:"error"`
	Warning         string `json:"warning"`
	Success         string `json:"success"`
	Info            string `json:"info"`
}

// TypographyTokens contains font-related tokens.
type TypographyTokens struct {
	FontFamily         string `json:"font_family"`
	FontSizeXS         string `json:"font_size_xs"`
	FontSizeSM         string `json:"font_size_sm"`
	FontSizeBase       string `json:"font_size_base"`
	FontSizeLG         string `json:"font_size_lg"`
	FontSizeXL         string `json:"font_size_xl"`
	FontWeightRegular  string `json:"font_weight_regular"`
	FontWeightMedium   string `json:"font_weight_medium"`
	FontWeightSemibold string `json:"font_weight_semibold"`
	FontWeightBold     string `json:"font_weight_bold"`
}

// ScaleTokens contains common spacing or radius scale values.
type ScaleTokens struct {
	XS string `json:"xs"`
	SM string `json:"sm"`
	MD string `json:"md"`
	LG string `json:"lg"`
	XL string `json:"xl"`
}

// ShadowTokens contains elevation tokens.
type ShadowTokens struct {
	SM string `json:"sm"`
	MD string `json:"md"`
	LG string `json:"lg"`
}

// LayoutTokens contains app-shell sizing tokens.
type LayoutTokens struct {
	SidebarWidth    string `json:"sidebar_width"`
	NavbarHeight    string `json:"navbar_height"`
	TransitionSpeed string `json:"transition_speed"`
}
