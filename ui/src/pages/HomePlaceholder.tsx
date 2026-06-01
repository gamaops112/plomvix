import { Button } from '../components/ui/Button';
import { useAppEvents } from '../events/AppEventProvider';

export function HomePlaceholder() {
  const { emit } = useAppEvents();
  return (
    <div className="page-card">
      <h1>Plomvix</h1>
      <p>UI foundation — Sprint 11</p>
      <Button
        onClick={() =>
          emit({ type: 'toast:add', payload: { kind: 'success', title: 'Toast works!' } })
        }
      >
        Test toast
      </Button>
    </div>
  );
}
