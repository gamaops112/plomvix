import { useTheme } from '../../theme/ThemeContext';
import { useAppEvents } from '../../events/AppEventProvider';
import type { Theme } from '../../theme/types';
import { Button } from '../../components/ui/Button';

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
      <div style={{ display: 'flex', gap: '0.5rem', marginBottom: '1rem' }}>
        <Button variant="secondary" onClick={handleExport}>
          Export (Local)
        </Button>
        <label className="button button-secondary" style={{ cursor: 'pointer' }}>
          Import
          <input
            type="file"
            accept=".json"
            onChange={handleImport}
            style={{ display: 'none' }}
          />
        </label>
      </div>
      <p style={{ fontSize: 'var(--plx-font-size-sm)', color: 'var(--plx-color-text-muted)' }}>
        Import/export works locally. Backend save/reset requires admin login.
      </p>
    </div>
  );
}
