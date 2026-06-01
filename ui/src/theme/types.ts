export type ThemeMode = 'light' | 'dark';

export interface Theme {
  version: number;
  dev_panel: boolean;
  mode: ThemeMode;
  tokens: ThemeTokens;
}

export interface ThemeTokens {
  colors: ColorTokens;
  dark_colors: ColorTokens;
  typography: TypographyTokens;
  radii: ScaleTokens;
  spacing: ScaleTokens;
  shadows: ShadowTokens;
  layout: LayoutTokens;
}

export interface ColorTokens {
  primary: string;
  secondary: string;
  accent: string;
  background: string;
  background_muted: string;
  surface: string;
  surface_muted: string;
  text: string;
  text_muted: string;
  border: string;
  error: string;
  warning: string;
  success: string;
  info: string;
}

export interface TypographyTokens {
  font_family: string;
  font_size_xs: string;
  font_size_sm: string;
  font_size_base: string;
  font_size_lg: string;
  font_size_xl: string;
  font_weight_regular: string;
  font_weight_medium: string;
  font_weight_semibold: string;
  font_weight_bold: string;
}

export interface ScaleTokens {
  xs: string;
  sm: string;
  md: string;
  lg: string;
  xl: string;
}

export interface ShadowTokens {
  sm: string;
  md: string;
  lg: string;
}

export interface LayoutTokens {
  sidebar_width: string;
  navbar_height: string;
  transition_speed: string;
}
