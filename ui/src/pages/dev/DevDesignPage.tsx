import { useTheme } from '../../theme/ThemeContext';
import { Button } from '../../components/ui/Button';
import { ColorEditor } from './ColorEditor';
import { TypographyEditor } from './TypographyEditor';
import { SpacingEditor } from './SpacingEditor';
import { ShadowEditor } from './ShadowEditor';
import { ComponentPreview } from './ComponentPreview';
import { ImportExportPanel } from './ImportExportPanel';

export function DevDesignPage() {
  const { theme, loading, saveDraft, resetToDefault, setMode, mode } = useTheme();

  if (loading) {
    return <div className="page-card"><p>Loading theme...</p></div>;
  }

  if (!theme.dev_panel) {
    return (
      <div className="page-card">
        <h1>Developer Design Panel</h1>
        <p>The design panel is disabled. Set <code>dev_panel: true</code> in <code>theme.json</code> to enable it.</p>
      </div>
    );
  }

  return (
    <div className="dev-design-page">
      <div className="dev-design-header">
        <h1>Developer Design Panel</h1>
        <div className="dev-design-actions">
          <Button variant="secondary" onClick={() => setMode(mode === 'light' ? 'dark' : 'light')}>
            Preview: {mode === 'light' ? 'Light ☀️' : 'Dark 🌙'}
          </Button>
          <Button variant="primary" onClick={saveDraft}>Save</Button>
          <Button variant="ghost" onClick={resetToDefault}>Reset</Button>
        </div>
      </div>

      <p>Edit design tokens and preview components in real time. Changes are applied live.</p>

      <section>
        <h2>Colors</h2>
        <ColorEditor />
      </section>

      <section>
        <h2>Typography</h2>
        <TypographyEditor />
      </section>

      <section>
        <h2>Spacing & Layout</h2>
        <SpacingEditor />
      </section>

      <section>
        <h2>Shadows</h2>
        <ShadowEditor />
      </section>

      <section>
        <h2>Component Preview</h2>
        <ComponentPreview />
      </section>

      <section>
        <h2>Import / Export</h2>
        <ImportExportPanel />
      </section>
    </div>
  );
}
