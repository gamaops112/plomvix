import { useTheme } from '../../theme/ThemeContext';

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
        <div key={g.label} style={{ marginBottom: '1.5rem' }}>
          <h3>{g.label}</h3>
          <div className="color-grid">
            {Object.entries(g.colors).map(([key, val]) => (
              <div key={key} className="color-field">
                <label>{key}</label>
                <div className="color-input-row">
                  <input
                    type="color"
                    value={val}
                    onChange={(e) => update(gi === 0 ? 'colors' : 'dark_colors', key, e.target.value)}
                  />
                  <input
                    type="text"
                    value={val}
                    onChange={(e) => update(gi === 0 ? 'colors' : 'dark_colors', key, e.target.value)}
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
