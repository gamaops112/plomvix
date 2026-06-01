import { Link } from 'react-router-dom';
import { Card, CardHeader, CardTitle, CardDescription, CardContent } from '@/components/ui/card';
import { Button } from '@/components/ui/button';

export function NotFoundPage() {
  return (
    <div className="flex items-center justify-center min-h-screen bg-background p-4">
      <Card className="w-full max-w-sm">
        <CardHeader className="text-center">
          <CardTitle className="text-6xl">404</CardTitle>
          <CardDescription>Page not found.</CardDescription>
        </CardHeader>
        <CardContent className="flex justify-center pb-6">
          <Button asChild variant="outline">
            <Link to="/app/explore">Go Home</Link>
          </Button>
        </CardContent>
      </Card>
    </div>
  );
}
