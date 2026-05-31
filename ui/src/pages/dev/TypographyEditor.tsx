import { useTheme } from '../../theme/ThemeContext';

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
      <h3>Font Family</h3>
      <input
        type="text"
        value={typo.font_family}
        onChange={(e) => update('font_family', e.target.value)}
        className="token-input"
      />
      <h3>Font Sizes</h3>
      <div className="token-grid">
        {(['font_size_xs', 'font_size_sm', 'font_size_base', 'font_size_lg', 'font_size_xl'] as const).map((key) => (
          <div key={key} className="token-field">
            <label>{key}</label>
            <input
              type="text"
              value={typo[key]}
              onChange={(e) => update(key, e.target.value)}
              className="token-input"
            />
            <span className="token-preview" style={{ fontSize: `var(--plx-${key.replace(/_/g, '-')})` }}>
              Aa
            </span>
          </div>
        ))}
      </div>
      <h3>Font Weights</h3>
      <div className="token-grid">
        {(['font_weight_regular', 'font_weight_medium', 'font_weight_semibold', 'font_weight_bold'] as const).map((key) => (
          <div key={key} className="token-field">
            <label>{key}</label>
            <input
              type="text"
              value={typo[key]}
              onChange={(e) => update(key, e.target.value)}
              className="token-input"
            />
          </div>
        ))}
      </div>
    </div>
  );
}
