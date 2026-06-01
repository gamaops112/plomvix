import { useTheme } from '../../theme/ThemeContext';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';

export function SpacingEditor() {
  const { draft, setDraftTheme } = useTheme();

  const update = (section: 'radii' | 'spacing' | 'layout', key: string, value: string) => {
    const updated = { ...draft };
    (updated.tokens[section] as unknown as Record<string, string>)[key] = value;
    setDraftTheme(updated);
  };

  // Radii skip xs
  const radiiKeys = ['sm', 'md', 'lg', 'xl'] as const;
  const spacingKeys = ['xs', 'sm', 'md', 'lg', 'xl'] as const;
  const layoutKeys = [
    { key: 'sidebar_width', label: 'Sidebar Width' },
    { key: 'navbar_height', label: 'Navbar Height' },
    { key: 'transition_speed', label: 'Transition Speed' },
  ] as const;

  return (
    <div>
      <h3 className="text-md font-semibold mb-2">Radii</h3>
      <div className="grid grid-cols-2 md:grid-cols-3 gap-4">
        {radiiKeys.map((k) => (
          <div key={k} className="space-y-1">
            <Label>{k}</Label>
            <Input
              type="text"
              value={draft.tokens.radii[k]}
              onChange={(e) => update('radii', k, e.target.value)}
            />
            <div className="radius-preview" style={{ borderRadius: `var(--plx-radius-${k})` }} />
          </div>
        ))}
      </div>
      <h3 className="text-md font-semibold mb-2">Spacing</h3>
      <div className="grid grid-cols-2 md:grid-cols-3 gap-4">
        {spacingKeys.map((k) => (
          <div key={k} className="space-y-1">
            <Label>{k}</Label>
            <Input
              type="text"
              value={draft.tokens.spacing[k]}
              onChange={(e) => update('spacing', k, e.target.value)}
            />
            <div className="spacing-preview" style={{ width: `var(--plx-spacing-${k})`, height: `var(--plx-spacing-${k})` }} />
          </div>
        ))}
      </div>
      <h3 className="text-md font-semibold mb-2">Layout</h3>
      <div className="grid grid-cols-2 md:grid-cols-3 gap-4">
        {layoutKeys.map(({ key, label }) => (
          <div key={key} className="space-y-1">
            <Label>{label}</Label>
            <Input
              type="text"
              value={(draft.tokens.layout as unknown as Record<string, string>)[key]}
              onChange={(e) => update('layout', key, e.target.value)}
            />
          </div>
        ))}
      </div>
    </div>
  );
}
