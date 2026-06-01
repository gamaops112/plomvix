import { useTheme } from '../theme/ThemeContext';
import { Button } from '@/components/ui/button';
import { Sun, Moon } from 'lucide-react';

export function ThemeModeToggle() {
  const { mode, setMode } = useTheme();

  return (
    <Button
      variant="ghost"
      className="h-9 w-9 p-0 rounded-full"
      onClick={() => setMode(mode === 'light' ? 'dark' : 'light')}
      aria-label={`Switch to ${mode === 'light' ? 'dark' : 'light'} mode`}
    >
      {mode === 'light' ? <Sun className="h-[1.2rem] w-[1.2rem]" /> : <Moon className="h-[1.2rem] w-[1.2rem]" />}
    </Button>
  );
}
