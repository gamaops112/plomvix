import { useState } from 'react';
import { useSearchParams } from 'react-router-dom';
import { useAuth } from '../auth/useAuth';
import { Tabs, TabsList, TabsTrigger, TabsContent } from '@/components/ui/tabs';
import { Card, CardContent } from '@/components/ui/card';
import { Skeleton } from '@/components/ui/skeleton';
import { useUsers } from '../admin/hooks/useUsers';
import { UserManagementPanel } from '../admin/components/UserManagementPanel';
import { APIKeyManagementPanel } from '../admin/components/APIKeyManagementPanel';
import { SystemStatsPanel } from '../admin/components/SystemStatsPanel';

const TABS = [
  { value: 'users', label: 'Users' },
  { value: 'api-keys', label: 'API Keys' },
  { value: 'stats', label: 'System Stats' },
] as const;

type TabValue = (typeof TABS)[number]['value'];

export function AdminPage() {
  const { user, loading } = useAuth();
  const { users } = useUsers();
  const [searchParams, setSearchParams] = useSearchParams();
  const [activeTab, setActiveTab] = useState<TabValue>(() => {
    const tab = searchParams.get('tab');
    return TABS.some((t) => t.value === tab) ? (tab as TabValue) : 'users';
  });

  const handleTabChange = (value: string) => {
    const tab = value as TabValue;
    setActiveTab(tab);
    setSearchParams({ tab }, { replace: true });
  };

  if (loading) {
    return (
      <div className="space-y-4 p-6">
        <Skeleton className="h-8 w-48" />
        <Skeleton className="h-4 w-96" />
        <div className="space-y-3 pt-4">
          <Skeleton className="h-4 w-full" />
          <Skeleton className="h-4 w-3/4" />
          <Skeleton className="h-4 w-1/2" />
        </div>
      </div>
    );
  }

  if (!user || user.role !== 'admin') {
    return (
      <div className="flex items-center justify-center min-h-[300px]">
        <Card className="max-w-md w-full">
          <CardContent className="pt-6 text-center space-y-3">
            <div className="text-4xl">🔒</div>
            <h2 className="text-xl font-semibold text-foreground">Access Denied</h2>
            <p className="text-muted-foreground">Admin role required to access this page.</p>
          </CardContent>
        </Card>
      </div>
    );
  }

  return (
    <div className="space-y-6 p-6 max-w-7xl mx-auto">
      <div>
        <h1 className="text-2xl font-bold text-foreground">Administration</h1>
        <p className="text-muted-foreground mt-1">
          Manage users, API keys, and monitor system health.
        </p>
      </div>

      <Tabs value={activeTab} onValueChange={handleTabChange}>
        <TabsList className="mb-4">
          {TABS.map((tab) => (
            <TabsTrigger key={tab.value} value={tab.value}>
              {tab.label}
            </TabsTrigger>
          ))}
        </TabsList>

        <TabsContent value="users">
          <UserManagementPanel />
        </TabsContent>

        <TabsContent value="api-keys">
          <APIKeyManagementPanel users={users} />
        </TabsContent>

        <TabsContent value="stats">
          <SystemStatsPanel />
        </TabsContent>
      </Tabs>
    </div>
  );
}
