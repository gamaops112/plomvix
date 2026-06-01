import { useTheme } from '../../theme/ThemeContext';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';

export function TypographyEditor() {
  const { draft, setDraftTheme } = useTheme();
  const typo = draft.tokens.typography;

  const update = (key: string, value: string) => {
    const updated = { ...draft };
    (updated.tokens.typography as unknown as Record<string, string>)[key] = value;
    setDraftTheme(updated);
  };

  return (
    <div>
      <h3 className="text-md font-semibold mb-2">Font Family</h3>
      <Input
        type="text"
        value={typo.font_family}
        onChange={(e) => update('font_family', e.target.value)}
      />
      <h3 className="text-md font-semibold mb-2">Font Sizes</h3>
      <div className="grid grid-cols-2 md:grid-cols-3 gap-4">
        {(['font_size_xs', 'font_size_sm', 'font_size_base', 'font_size_lg', 'font_size_xl'] as const).map((key) => (
          <div key={key} className="space-y-1">
            <Label>{key}</Label>
            <Input
              type="text"
              value={typo[key]}
              onChange={(e) => update(key, e.target.value)}
            />
            <span className="text-xs text-muted-foreground" style={{ fontSize: `var(--plx-${key.replace(/_/g, '-')})` }}>
              Aa
            </span>
          </div>
        ))}
      </div>
      <h3 className="text-md font-semibold mb-2">Font Weights</h3>
      <div className="grid grid-cols-2 md:grid-cols-3 gap-4">
        {(['font_weight_regular', 'font_weight_medium', 'font_weight_semibold', 'font_weight_bold'] as const).map((key) => (
          <div key={key} className="space-y-1">
            <Label>{key}</Label>
            <Input
              type="text"
              value={typo[key]}
              onChange={(e) => update(key, e.target.value)}
            />
          </div>
        ))}
      </div>
    </div>
  );
}
