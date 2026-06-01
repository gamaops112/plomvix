import { useTheme } from '../../theme/ThemeContext';
import { Button } from '@/components/ui/button';
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card';
import { Sun, Moon } from 'lucide-react';
import { ColorEditor } from './ColorEditor';
import { TypographyEditor } from './TypographyEditor';
import { SpacingEditor } from './SpacingEditor';
import { ShadowEditor } from './ShadowEditor';
import { ComponentPreview } from './ComponentPreview';
import { ImportExportPanel } from './ImportExportPanel';

export function DevDesignPage() {
  const { theme, loading, saveDraft, resetToDefault, setMode, mode } = useTheme();

  if (loading) {
    return (
      <Card>
        <CardContent className="p-8 text-center">
          <p className="text-muted-foreground">Loading theme...</p>
        </CardContent>
      </Card>
    );
  }

  if (!theme.dev_panel) {
    return (
      <Card>
        <CardHeader>
          <CardTitle>Developer Design Panel</CardTitle>
        </CardHeader>
        <CardContent>
          <p>The design panel is disabled. Set <code>dev_panel: true</code> in <code>theme.json</code> to enable it.</p>
        </CardContent>
      </Card>
    );
  }

  return (
    <div className="space-y-8">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold">Developer Design Panel</h1>
        <div className="flex gap-2">
          <Button variant="secondary" onClick={() => setMode(mode === 'light' ? 'dark' : 'light')}>
            {mode === 'light' ? <Sun className="h-4 w-4" /> : <Moon className="h-4 w-4" />} {mode === 'light' ? 'Light' : 'Dark'}
          </Button>
          <Button variant="default" onClick={saveDraft}>Save</Button>
          <Button variant="ghost" onClick={resetToDefault}>Reset</Button>
        </div>
      </div>

      <p className="text-muted-foreground mb-8">Edit design tokens and preview components in real time. Changes are applied live.</p>

      <section>
        <h2 className="text-lg font-semibold mb-4">Colors</h2>
        <ColorEditor />
      </section>

      <section>
        <h2 className="text-lg font-semibold mb-4">Typography</h2>
        <TypographyEditor />
      </section>

      <section>
        <h2 className="text-lg font-semibold mb-4">Spacing & Layout</h2>
        <SpacingEditor />
      </section>

      <section>
        <h2 className="text-lg font-semibold mb-4">Shadows</h2>
        <ShadowEditor />
      </section>

      <section>
        <h2 className="text-lg font-semibold mb-4">Component Preview</h2>
        <ComponentPreview />
      </section>

      <section>
        <h2 className="text-lg font-semibold mb-4">Import / Export</h2>
        <ImportExportPanel />
      </section>
    </div>
  );
}
