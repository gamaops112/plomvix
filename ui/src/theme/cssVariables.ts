import type { Theme, ThemeMode } from './types';

const VAR_MAP: Record<string, string> = {
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
  sm: '--plx-radius-sm',
  md: '--plx-radius-md',
  lg: '--plx-radius-lg',
  xl: '--plx-radius-xl',
};

export function themeToCSSVariables(theme: Theme, mode?: ThemeMode): Record<string, string> {
  const activeMode = mode ?? theme.mode;
  const colors = activeMode === 'dark' ? theme.tokens.dark_colors : theme.tokens.colors;
  const vars: Record<string, string> = {};

  for (const [key, cssVar] of Object.entries(VAR_MAP)) {
    if (key in colors) {
      vars[cssVar] = (colors as unknown as Record<string, string>)[key];
    }
  }

  const typo = theme.tokens.typography;
  vars['--plx-font-family'] = typo.font_family;
  vars['--plx-font-size-xs'] = typo.font_size_xs;
  vars['--plx-font-size-sm'] = typo.font_size_sm;
  vars['--plx-font-size-base'] = typo.font_size_base;
  vars['--plx-font-size-lg'] = typo.font_size_lg;
  vars['--plx-font-size-xl'] = typo.font_size_xl;
  vars['--plx-font-weight-regular'] = typo.font_weight_regular;
  vars['--plx-font-weight-medium'] = typo.font_weight_medium;
  vars['--plx-font-weight-semibold'] = typo.font_weight_semibold;
  vars['--plx-font-weight-bold'] = typo.font_weight_bold;

  return vars;
}

export function applyTheme(theme: Theme, mode?: ThemeMode): void {
  const vars = themeToCSSVariables(theme, mode);
  const root = document.documentElement;

  // Remove old plx vars first
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
