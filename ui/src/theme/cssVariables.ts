import type { Theme, ThemeMode } from './types';

const COLOR_VAR_MAP: Record<string, string> = {
  primary: '--plx-color-primary',
  secondary: '--plx-color-secondary',
  accent: '--plx-color-accent',
  background: '--plx-color-background',
  background_muted: '--plx-color-background-muted',
  surface: '--plx-color-surface',
  surface_muted: '--plx-color-surface-muted',
  text: '--plx-color-text',
  text_muted: '--plx-color-text-muted',
  border: '--plx-color-border',
  error: '--plx-color-error',
  warning: '--plx-color-warning',
  success: '--plx-color-success',
  info: '--plx-color-info',
};

const TYPO_VAR_MAP: Record<string, string> = {
  font_family: '--plx-font-family',
  font_size_xs: '--plx-font-size-xs',
  font_size_sm: '--plx-font-size-sm',
  font_size_base: '--plx-font-size-base',
  font_size_lg: '--plx-font-size-lg',
  font_size_xl: '--plx-font-size-xl',
  font_weight_regular: '--plx-font-weight-regular',
  font_weight_medium: '--plx-font-weight-medium',
  font_weight_semibold: '--plx-font-weight-semibold',
  font_weight_bold: '--plx-font-weight-bold',
};

const RADIUS_VAR_MAP: Record<string, string> = {
  sm: '--plx-radius-sm',
  md: '--plx-radius-md',
  lg: '--plx-radius-lg',
  xl: '--plx-radius-xl',
};

const SPACING_VAR_MAP: Record<string, string> = {
  xs: '--plx-spacing-xs',
  sm: '--plx-spacing-sm',
  md: '--plx-spacing-md',
  lg: '--plx-spacing-lg',
  xl: '--plx-spacing-xl',
};

const SHADOW_VAR_MAP: Record<string, string> = {
  sm: '--plx-shadow-sm',
  md: '--plx-shadow-md',
  lg: '--plx-shadow-lg',
};

const LAYOUT_VAR_MAP: Record<string, string> = {
  sidebar_width: '--plx-sidebar-width',
  navbar_height: '--plx-navbar-height',
  transition_speed: '--plx-transition-speed',
};

function mapTokens(
  source: Record<string, string> | undefined,
  varMap: Record<string, string>,
): Record<string, string> {
  const vars: Record<string, string> = {};
  if (!source) return vars;
  for (const [key, cssVar] of Object.entries(varMap)) {
    if (key in source) {
      vars[cssVar] = source[key];
    }
  }
  return vars;
}

export function themeToCSSVariables(theme: Theme, mode?: ThemeMode): Record<string, string> {
  const activeMode = mode ?? theme.mode;
  const colors = activeMode === 'dark' ? theme.tokens.dark_colors : theme.tokens.colors;
  let vars: Record<string, string> = {};

  vars = { ...vars, ...mapTokens(colors as unknown as Record<string, string>, COLOR_VAR_MAP) };
  vars = { ...vars, ...mapTokens(theme.tokens.typography as unknown as Record<string, string>, TYPO_VAR_MAP) };
  vars = { ...vars, ...mapTokens(theme.tokens.radii as unknown as Record<string, string>, RADIUS_VAR_MAP) };
  vars = { ...vars, ...mapTokens(theme.tokens.spacing as unknown as Record<string, string>, SPACING_VAR_MAP) };
  vars = { ...vars, ...mapTokens(theme.tokens.shadows as unknown as Record<string, string>, SHADOW_VAR_MAP) };
  vars = { ...vars, ...mapTokens(theme.tokens.layout as unknown as Record<string, string>, LAYOUT_VAR_MAP) };

  return vars;
}

export function applyTheme(theme: Theme, mode?: ThemeMode): void {
  const vars = themeToCSSVariables(theme, mode);
  const root = document.documentElement;

  const keysToRemove: string[] = [];
  const style = root.style;
  for (let i = 0; i < style.length; i++) {
    const prop = style.item(i);
    if (prop.startsWith('--plx-')) {
      keysToRemove.push(prop);
    }
  }
  for (const k of keysToRemove) {
    style.removeProperty(k);
  }

  for (const [name, value] of Object.entries(vars)) {
    style.setProperty(name, value);
  }
}
