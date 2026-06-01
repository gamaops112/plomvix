export function ComponentPreview() {
  return (
    <div className="preview-section">
      <h3>Button</h3>
      <div style={{ display: 'flex', gap: '0.5rem', marginBottom: '1rem' }}>
        <button className="button button-primary">Primary</button>
        <button className="button button-secondary">Secondary</button>
        <button className="button button-ghost">Ghost</button>
      </div>

      <h3>Input</h3>
      <input
        type="text"
        placeholder="Sample input"
        style={{
          padding: 'var(--plx-spacing-sm)',
          borderRadius: 'var(--plx-radius-md)',
          border: '1px solid var(--plx-color-border)',
          background: 'var(--plx-color-surface)',
          color: 'var(--plx-color-text)',
        }}
      />

      <h3>Table Row</h3>
      <table style={{ width: '100%', borderCollapse: 'collapse', margin: '1rem 0' }}>
        <tbody>
          <tr style={{ borderBottom: '1px solid var(--plx-color-border)' }}>
            <td style={{ padding: '0.5rem' }}>Cell 1</td>
            <td style={{ padding: '0.5rem' }}>Cell 2</td>
            <td style={{ padding: '0.5rem' }}>Cell 3</td>
          </tr>
        </tbody>
      </table>

      <h3>Card</h3>
      <div
        style={{
          padding: '1.5rem',
          borderRadius: 'var(--plx-radius-lg)',
          background: 'var(--plx-color-surface)',
          border: '1px solid var(--plx-color-border)',
          boxShadow: 'var(--plx-shadow-sm)',
        }}
      >
        <h4>Card Title</h4>
        <p style={{ color: 'var(--plx-color-text-muted)' }}>Card content with muted text.</p>
      </div>

      <h3>Badge</h3>
      <div style={{ display: 'flex', gap: '0.5rem', margin: '1rem 0' }}>
        {(['primary', 'success', 'error', 'warning', 'info'] as const).map((v) => (
          <span
            key={v}
            style={{
              padding: '0.125rem 0.5rem',
              borderRadius: 'var(--plx-radius-sm)',
              fontSize: 'var(--plx-font-size-xs)',
              fontWeight: 500,
              background: `var(--plx-color-${v})`,
              color: '#fff',
            }}
          >
            {v}
          </span>
        ))}
      </div>

      <h3>Chart Placeholder</h3>
      <div
        style={{
          height: '120px',
          borderRadius: 'var(--plx-radius-md)',
          background: 'var(--plx-color-background-muted)',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          color: 'var(--plx-color-text-muted)',
          fontSize: 'var(--plx-font-size-sm)',
        }}
      >
        Chart Placeholder
      </div>

      <h3>Modal Mockup</h3>
      <div
        style={{
          padding: '1.5rem',
          borderRadius: 'var(--plx-radius-lg)',
          background: 'var(--plx-color-surface)',
          border: '1px solid var(--plx-color-border)',
          boxShadow: 'var(--plx-shadow-lg)',
          maxWidth: '400px',
        }}
      >
        <h4>Modal Title</h4>
        <p style={{ color: 'var(--plx-color-text-muted)' }}>This is a modal dialog mockup.</p>
        <div style={{ display: 'flex', gap: '0.5rem', marginTop: '1rem' }}>
          <button className="button button-primary">Confirm</button>
          <button className="button button-secondary">Cancel</button>
        </div>
      </div>

      <h3>Sidebar Item</h3>
      <div
        style={{
          padding: '0.5rem 0.75rem',
          borderRadius: 'var(--plx-radius-md)',
          background: 'var(--plx-color-primary)',
          color: '#fff',
          fontSize: 'var(--plx-font-size-sm)',
          margin: '0.25rem 0',
        }}
      >
        Active Sidebar Item
      </div>

      <h3>Navbar Item</h3>
      <div
        style={{
          display: 'inline-block',
          padding: '0.5rem 1rem',
          borderRadius: 'var(--plx-radius-md)',
          color: 'var(--plx-color-text)',
          fontSize: 'var(--plx-font-size-sm)',
          borderBottom: '2px solid var(--plx-color-primary)',
        }}
      >
        Active Nav Item
      </div>
    </div>
  );
}
