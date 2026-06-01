import { cn } from '@/lib/utils';
import { useAppEvents } from '../../events/AppEventProvider';

export function ToastViewport() {
  const { toasts, emit } = useAppEvents();

  return (
    <div className="fixed bottom-4 right-4 flex flex-col gap-2 z-[9999] max-w-[360px]" aria-live="polite" aria-relevant="additions removals">
      {toasts.map((toast) => (
        <div
          className={cn("flex items-start justify-between p-3 rounded-lg bg-card border shadow-lg", toast.kind === 'success' && 'border-l-4 border-l-[var(--plx-color-success)]', toast.kind === 'error' && 'border-l-4 border-l-[var(--plx-color-error)]', toast.kind === 'warning' && 'border-l-4 border-l-[var(--plx-color-warning)]', toast.kind === 'info' && 'border-l-4 border-l-[var(--plx-color-info)]')}
          key={toast.id}
          role="status"
        >
          <div>
            <strong className="text-sm block">{toast.title}</strong>
            {toast.message ? <p className="text-xs text-muted-foreground mt-1">{toast.message}</p> : null}
          </div>
          <button
            type="button"
            className="bg-none border-none text-xl cursor-pointer text-muted-foreground hover:text-foreground p-0 pl-2 leading-none"
            aria-label={`Dismiss ${toast.title}`}
            onClick={() => emit({ type: 'toast:remove', payload: { id: toast.id } })}
          >
            ×
          </button>
        </div>
      ))}
    </div>
  );
}
