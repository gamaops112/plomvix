import { useTheme } from '../../theme/ThemeContext';
import { useAppEvents } from '../../events/AppEventProvider';
import type { Theme } from '../../theme/types';
import { Button } from '@/components/ui/button';
import { Label } from '@/components/ui/label';

export function ImportExportPanel() {
  const { draft, setDraftTheme } = useTheme();
  const { emit } = useAppEvents();

  const handleExport = () => {
    const blob = new Blob([JSON.stringify(draft, null, 2)], { type: 'application/json' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = 'plomvix-theme.json';
    a.click();
    URL.revokeObjectURL(url);
    emit({ type: 'toast:add', payload: { kind: 'success', title: 'Theme exported locally' } });
  };

  const handleImport = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;

    const reader = new FileReader();
    reader.onload = (ev) => {
      try {
        const parsed: Theme = JSON.parse(ev.target?.result as string);
        if (!parsed.version || !parsed.tokens) {
          throw new Error('Invalid theme file: missing version or tokens');
        }
        setDraftTheme(parsed);
        emit({ type: 'toast:add', payload: { kind: 'success', title: 'Theme imported' } });
      } catch (err) {
        emit({
          type: 'toast:add',
          payload: { kind: 'error', title: 'Invalid JSON', message: (err as Error).message },
        });
      }
    };
    reader.readAsText(file);
  };

  return (
    <div>
      <div className="flex gap-2 mb-4">
        <Button variant="secondary" onClick={handleExport}>
          Export (Local)
        </Button>
        <Label className="cursor-pointer inline-flex items-center justify-center rounded-md text-sm font-medium h-9 px-4 py-2 bg-secondary text-secondary-foreground hover:bg-secondary/80">
          Import
          <input
            type="file"
            accept=".json"
            onChange={handleImport}
            className="hidden"
          />
        </Label>
      </div>
      <p className="text-sm text-muted-foreground">
        Import/export works locally. Backend save/reset requires admin login.
      </p>
    </div>
  );
}
