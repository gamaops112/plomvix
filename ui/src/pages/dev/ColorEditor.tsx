import { useTheme } from '../../theme/ThemeContext';
import { Label } from '@/components/ui/label';

export function ColorEditor() {
  const { draft, setDraftTheme } = useTheme();

  const groups: { label: string; colors: Record<string, string> }[] = [
    { label: 'Light Colors', colors: draft.tokens.colors as unknown as Record<string, string> },
    { label: 'Dark Colors', colors: draft.tokens.dark_colors as unknown as Record<string, string> },
  ];

  const update = (group: 'colors' | 'dark_colors', key: string, value: string) => {
    const updated = { ...draft };
    (updated.tokens[group] as unknown as Record<string, string>)[key] = value;
    setDraftTheme(updated);
  };

  return (
    <div>
      {groups.map((g, gi) => (
        <div key={g.label} className="mb-6">
          <h3 className="text-md font-semibold mb-2">{g.label}</h3>
          <div className="grid grid-cols-2 md:grid-cols-3 gap-4">
            {Object.entries(g.colors).map(([key, val]) => (
              <div key={key} className="space-y-1">
                <Label>{key}</Label>
                <div className="flex items-center gap-2">
                  <input
                    type="color"
                    value={val}
                    onChange={(e) => update(gi === 0 ? 'colors' : 'dark_colors', key, e.target.value)}
                  />
                  <input
                    type="text"
                    value={val}
                    onChange={(e) => update(gi === 0 ? 'colors' : 'dark_colors', key, e.target.value)}
                    className="flex-1 h-9 px-3 rounded-md border bg-background text-sm"
                  />
                </div>
              </div>
            ))}
          </div>
        </div>
      ))}
    </div>
  );
}
