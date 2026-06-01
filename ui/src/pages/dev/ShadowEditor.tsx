import { useTheme } from '../../theme/ThemeContext';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';

export function ShadowEditor() {
  const { draft, setDraftTheme } = useTheme();
  const shadows = draft.tokens.shadows;
  const levels = ['sm', 'md', 'lg'] as const;

  const update = (key: string, value: string) => {
    const updated = { ...draft };
    (updated.tokens.shadows as unknown as Record<string, string>)[key] = value;
    setDraftTheme(updated);
  };

  return (
    <div>
      {levels.map((level) => (
        <div key={level} className="mb-4 space-y-1">
          <Label>{level}</Label>
          <Input
            type="text"
            value={shadows[level]}
            onChange={(e) => update(level, e.target.value)}
          />
          <div
            className="p-4 rounded-md border bg-card"
            style={{
              boxShadow: `var(--plx-shadow-${level})`,
              margin: '0.5rem 0',
            }}
          >
            {level} shadow preview
          </div>
        </div>
      ))}
    </div>
  );
}
