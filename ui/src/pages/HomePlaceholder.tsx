import { Button } from '@/components/ui/button';
import { useAppEvents } from '../events/AppEventProvider';
import { Card, CardHeader, CardTitle, CardDescription, CardContent } from '@/components/ui/card';

export function HomePlaceholder() {
  const { emit } = useAppEvents();
  return (
    <Card>
      <CardHeader>
        <CardTitle>Plomvix</CardTitle>
        <CardDescription>UI foundation — Sprint 11</CardDescription>
      </CardHeader>
      <CardContent>
        <Button
          onClick={() =>
            emit({ type: 'toast:add', payload: { kind: 'success', title: 'Toast works!' } })
          }
        >
          Test toast
        </Button>
      </CardContent>
    </Card>
  );
}
