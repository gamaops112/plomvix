import { useTheme } from '../../theme/ThemeContext';

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
      <h3>Radii</h3>
      <div className="token-grid">
        {radiiKeys.map((k) => (
          <div key={k} className="token-field">
            <label>{k}</label>
            <input
              type="text"
              value={draft.tokens.radii[k]}
              onChange={(e) => update('radii', k, e.target.value)}
              className="token-input"
            />
            <div className="radius-preview" style={{ borderRadius: `var(--plx-radius-${k})` }} />
          </div>
        ))}
      </div>
      <h3>Spacing</h3>
      <div className="token-grid">
        {spacingKeys.map((k) => (
          <div key={k} className="token-field">
            <label>{k}</label>
            <input
              type="text"
              value={draft.tokens.spacing[k]}
              onChange={(e) => update('spacing', k, e.target.value)}
              className="token-input"
            />
            <div className="spacing-preview" style={{ width: `var(--plx-spacing-${k})`, height: `var(--plx-spacing-${k})` }} />
          </div>
        ))}
      </div>
      <h3>Layout</h3>
      <div className="token-grid">
        {layoutKeys.map(({ key, label }) => (
          <div key={key} className="token-field">
            <label>{label}</label>
            <input
              type="text"
              value={(draft.tokens.layout as unknown as Record<string, string>)[key]}
              onChange={(e) => update('layout', key, e.target.value)}
              className="token-input"
            />
          </div>
        ))}
      </div>
    </div>
  );
}
