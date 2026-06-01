import { useAppEvents } from '../../events/AppEventProvider';

export function ToastViewport() {
  const { toasts, emit } = useAppEvents();

  return (
    <div className="toast-viewport" aria-live="polite" aria-relevant="additions removals">
      {toasts.map((toast) => (
        <div className={`toast toast-${toast.kind}`} key={toast.id} role="status">
          <div>
            <strong>{toast.title}</strong>
            {toast.message ? <p>{toast.message}</p> : null}
          </div>
          <button
            type="button"
            className="toast-close"
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
