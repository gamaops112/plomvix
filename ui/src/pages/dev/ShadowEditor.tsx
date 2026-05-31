import { useTheme } from '../../theme/ThemeContext';

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
        <div key={level} className="shadow-field">
          <label>{level}</label>
          <input
            type="text"
            value={shadows[level]}
            onChange={(e) => update(level, e.target.value)}
            className="token-input"
          />
          <div
            className="shadow-preview-card"
            style={{
              boxShadow: `var(--plx-shadow-${level})`,
              padding: '1rem',
              background: 'var(--plx-color-surface)',
              borderRadius: 'var(--plx-radius-md)',
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
