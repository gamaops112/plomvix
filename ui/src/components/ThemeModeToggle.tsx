import { useTheme } from '../theme/ThemeContext';
import { Button } from './ui/Button';

export function ThemeModeToggle() {
  const { mode, setMode } = useTheme();
  const label = mode === 'light' ? '☀️ Light' : '🌙 Dark';

  return (
    <Button
      variant="ghost"
      onClick={() => setMode(mode === 'light' ? 'dark' : 'light')}
      aria-label={`Switch to ${mode === 'light' ? 'dark' : 'light'} mode`}
    >
      {label}
    </Button>
  );
}
