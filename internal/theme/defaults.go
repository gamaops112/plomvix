package theme

const DefaultPath = "theme.json"

// DefaultTheme returns a new Theme struct with all default design tokens.
func DefaultTheme() *Theme {
	return &Theme{
		Version:  1,
		DevPanel: true,
		Mode:     "light",
		Tokens: Tokens{
			Colors: ColorTokens{
				Primary:         "#2563eb",
				Secondary:       "#64748b",
				Accent:          "#14b8a6",
				Background:      "#ffffff",
				BackgroundMuted: "#f8fafc",
				Surface:         "#ffffff",
				SurfaceMuted:    "#f1f5f9",
				Text:            "#0f172a",
				TextMuted:       "#64748b",
				Border:          "#e2e8f0",
				Error:           "#dc2626",
				Warning:         "#f59e0b",
				Success:         "#16a34a",
				Info:            "#0ea5e9",
			},
			DarkColors: ColorTokens{
				Primary:         "#60a5fa",
				Secondary:       "#94a3b8",
				Accent:          "#2dd4bf",
				Background:      "#020617",
				BackgroundMuted: "#0f172a",
				Surface:         "#111827",
				SurfaceMuted:    "#1e293b",
				Text:            "#f8fafc",
				TextMuted:       "#94a3b8",
				Border:          "#334155",
				Error:           "#f87171",
				Warning:         "#fbbf24",
				Success:         "#4ade80",
				Info:            "#38bdf8",
			},
			Typography: TypographyTokens{
				FontFamily:         "Inter, ui-sans-serif, system-ui, sans-serif",
				FontSizeXS:         "0.75rem",
				FontSizeSM:         "0.875rem",
				FontSizeBase:       "1rem",
				FontSizeLG:         "1.125rem",
				FontSizeXL:         "1.25rem",
				FontWeightRegular:  "400",
				FontWeightMedium:   "500",
				FontWeightSemibold: "600",
				FontWeightBold:     "700",
			},
			Radii: ScaleTokens{
				SM: "0.375rem",
				MD: "0.5rem",
				LG: "0.75rem",
				XL: "1rem",
			},
			Spacing: ScaleTokens{
				XS: "0.25rem",
				SM: "0.5rem",
				MD: "1rem",
				LG: "1.5rem",
				XL: "2rem",
			},
			Shadows: ShadowTokens{
				SM: "0 1px 2px rgba(15, 23, 42, 0.08)",
				MD: "0 8px 20px rgba(15, 23, 42, 0.12)",
				LG: "0 16px 40px rgba(15, 23, 42, 0.16)",
			},
			Layout: LayoutTokens{
				SidebarWidth:    "260px",
				NavbarHeight:    "56px",
				TransitionSpeed: "160ms",
			},
		},
	}
}
