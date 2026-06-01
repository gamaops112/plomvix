import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Badge } from '@/components/ui/badge';
import { Table, TableBody, TableRow, TableCell } from '@/components/ui/table';
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card';

export function ComponentPreview() {
  return (
    <div>
      <h3>Button</h3>
      <div style={{ display: 'flex', gap: '0.5rem', marginBottom: '1rem' }}>
        <Button variant="default">Primary</Button>
        <Button variant="secondary">Secondary</Button>
        <Button variant="ghost">Ghost</Button>
      </div>

      <h3>Input</h3>
      <Input placeholder="Sample input" />

      <h3>Table Row</h3>
      <Table style={{ margin: '1rem 0' }}>
        <TableBody>
          <TableRow>
            <TableCell>Cell 1</TableCell>
            <TableCell>Cell 2</TableCell>
            <TableCell>Cell 3</TableCell>
          </TableRow>
        </TableBody>
      </Table>

      <h3>Card</h3>
      <Card>
        <CardHeader>
          <CardTitle>Card Title</CardTitle>
        </CardHeader>
        <CardContent>
          <p className="text-muted-foreground">Card content with muted text.</p>
        </CardContent>
      </Card>

      <h3>Badge</h3>
      <div style={{ display: 'flex', gap: '0.5rem', margin: '1rem 0' }}>
        {(['default', 'secondary', 'destructive', 'outline'] as const).map((v) => (
          <Badge key={v} variant={v}>
            {v}
          </Badge>
        ))}
      </div>

      <h3>Chart Placeholder</h3>
      <div className="h-[120px] rounded-md bg-muted flex items-center justify-center text-muted-foreground text-sm">
        Chart Placeholder
      </div>

      <h3>Modal Mockup</h3>
      <Card className="max-w-[400px]">
        <CardHeader>
          <CardTitle>Modal Title</CardTitle>
        </CardHeader>
        <CardContent>
          <p className="text-muted-foreground">This is a modal dialog mockup.</p>
          <div style={{ display: 'flex', gap: '0.5rem', marginTop: '1rem' }}>
            <Button variant="default">Confirm</Button>
            <Button variant="secondary">Cancel</Button>
          </div>
        </CardContent>
      </Card>

      <h3>Sidebar Item</h3>
      <div className="px-3 py-2 rounded-md text-sm text-primary-foreground bg-primary inline-block my-1">
        Active Sidebar Item
      </div>

      <h3>Navbar Item</h3>
      <div className="inline-block px-4 py-2 rounded-md text-sm border-b-2 border-primary">
        Active Nav Item
      </div>
    </div>
  );
}
