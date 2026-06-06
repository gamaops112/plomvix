export const colorCardData = [
  {
    label: 'Total Services',
    value: '42',
    unit: 'services',
    change: '+3',
    changeType: 'positive' as const,
    color: '#06b6d4',
    sparkline: [28,30,29,32,31,35,33,36,38,37,39,40,38,41,40,42,41,43,42,42],
  },
  {
    label: 'Request Rate',
    value: '12.4k',
    unit: 'req/s',
    change: '+8.2%',
    changeType: 'positive' as const,
    color: '#8b5cf6',
    sparkline: [8.1,8.4,9.0,8.8,9.2,10.1,10.8,11.2,10.9,11.5,11.8,12.0,11.7,12.1,12.3,12.4,12.2,12.5,12.3,12.4],
  },
  {
    label: 'Error Rate',
    value: '0.8',
    unit: '%',
    change: '-0.2%',
    changeType: 'positive' as const,
    color: '#ef4444',
    sparkline: [1.2,1.1,1.3,1.0,0.9,1.1,1.0,0.8,0.9,1.0,0.8,0.9,0.7,0.8,0.9,0.8,0.7,0.8,0.9,0.8],
  },
  {
    label: 'Avg Latency',
    value: '124',
    unit: 'ms',
    change: '+12ms',
    changeType: 'negative' as const,
    color: '#f59e0b',
    sparkline: [98,102,108,112,105,118,115,120,118,122,119,124,121,123,125,122,124,126,123,124],
  },
]

export const statTileData = [
  { label: 'Hosts', value: '128', delta: '+2', deltaType: 'positive' as const },
  { label: 'Containers', value: '847', delta: '+14', deltaType: 'positive' as const },
  { label: 'Processes', value: '2,341', delta: 'stable', deltaType: 'neutral' as const },
  { label: 'Log Events/s', value: '8.2k', delta: '+1.1k', deltaType: 'positive' as const },
  { label: 'Active Traces', value: '234', delta: '-12', deltaType: 'negative' as const },
  { label: 'Open Alerts', value: '7', delta: '+2', deltaType: 'negative' as const },
  { label: 'Incidents', value: '1', delta: 'stable', deltaType: 'neutral' as const },
]

export const timeSeriesData = {
  timestamps: Array.from({ length: 60 }, (_, i) => {
    const d = new Date()
    d.setMinutes(d.getMinutes() - (59 - i))
    return d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
  }),
  requestRate: Array.from({ length: 60 }, (_, i) => {
    const base = 11000 + Math.sin(i / 10) * 2000 + Math.sin(i / 5) * 800
    return Math.round(base + Math.random() * 500)
  }),
  errorRate: Array.from({ length: 60 }, (_, i) => {
    let base = 0.6 + Math.abs(Math.sin(i / 15)) * 0.4 + Math.random() * 0.2
    if (i === 35) base = 1.8
    return Math.round(base * 10) / 10
  }),
}

export const serviceHealthData = [
  { name: 'api-gateway',      status: 'healthy'  as const, latency: '45ms',  uptime: '99.9%' },
  { name: 'auth-service',     status: 'healthy'  as const, latency: '23ms',  uptime: '100%' },
  { name: 'user-service',     status: 'degraded' as const, latency: '312ms', uptime: '99.1%' },
  { name: 'payment-service',  status: 'healthy'  as const, latency: '67ms',  uptime: '99.8%' },
  { name: 'notification-svc', status: 'healthy'  as const, latency: '18ms',  uptime: '100%' },
  { name: 'search-service',   status: 'down'     as const, latency: '—',     uptime: '97.2%' },
  { name: 'storage-service',  status: 'healthy'  as const, latency: '89ms',  uptime: '99.9%' },
  { name: 'cache-service',    status: 'healthy'  as const, latency: '4ms',   uptime: '100%' },
  { name: 'queue-service',    status: 'degraded' as const, latency: '445ms', uptime: '98.7%' },
  { name: 'analytics-svc',    status: 'healthy'  as const, latency: '134ms', uptime: '99.5%' },
]

export const serviceMapData = {
  nodes: [
    { id: 'gateway',  name: 'api-gateway',     x: 300, y: 200, symbolSize: 40, status: 'healthy' as const },
    { id: 'auth',     name: 'auth-service',     x: 150, y: 100, symbolSize: 30, status: 'healthy' as const },
    { id: 'user',     name: 'user-service',     x: 150, y: 300, symbolSize: 30, status: 'degraded' as const },
    { id: 'payment',  name: 'payment-service',  x: 450, y: 100, symbolSize: 30, status: 'healthy' as const },
    { id: 'search',   name: 'search-service',   x: 450, y: 300, symbolSize: 30, status: 'down' as const },
    { id: 'cache',    name: 'cache-service',    x: 600, y: 200, symbolSize: 25, status: 'healthy' as const },
    { id: 'storage',  name: 'storage-service',  x: 300, y: 350, symbolSize: 25, status: 'healthy' as const },
  ],
  edges: [
    { source: 'gateway', target: 'auth' },
    { source: 'gateway', target: 'user' },
    { source: 'gateway', target: 'payment' },
    { source: 'gateway', target: 'search' },
    { source: 'search',  target: 'cache' },
    { source: 'user',    target: 'storage' },
    { source: 'payment', target: 'storage' },
  ],
}

export const recentAlertsData = [
  { id: 1, severity: 'critical' as const, title: 'search-service is down',           service: 'search-service',   time: '2m ago',  status: 'firing' as const },
  { id: 2, severity: 'warning'  as const, title: 'High latency on user-service',     service: 'user-service',     time: '8m ago',  status: 'firing' as const },
  { id: 3, severity: 'warning'  as const, title: 'Queue depth above threshold',      service: 'queue-service',    time: '15m ago', status: 'firing' as const },
  { id: 4, severity: 'info'     as const, title: 'Deployment completed',             service: 'auth-service',     time: '22m ago', status: 'resolved' as const },
  { id: 5, severity: 'critical' as const, title: 'Payment timeout spike',            service: 'payment-service',  time: '34m ago', status: 'resolved' as const },
  { id: 6, severity: 'info'     as const, title: 'Auto-scaled: added 3 instances',  service: 'api-gateway',      time: '41m ago', status: 'resolved' as const },
]

export const logsPreviewData = [
  { level: 'ERROR' as const, time: '14:23:01', service: 'search-service',  message: 'Connection refused: redis:6379' },
  { level: 'WARN'  as const, time: '14:22:58', service: 'user-service',    message: 'Response time exceeded 300ms threshold' },
  { level: 'INFO'  as const, time: '14:22:55', service: 'auth-service',    message: 'Token refreshed for user_id=8821' },
  { level: 'INFO'  as const, time: '14:22:54', service: 'api-gateway',     message: 'GET /api/v2/users 200 45ms' },
  { level: 'ERROR' as const, time: '14:22:51', service: 'search-service',  message: 'Timeout after 5000ms waiting for index' },
  { level: 'DEBUG' as const, time: '14:22:49', service: 'cache-service',   message: 'Cache hit ratio: 94.2%' },
  { level: 'INFO'  as const, time: '14:22:47', service: 'payment-service', message: 'Transaction processed: txn_id=pp_9921' },
  { level: 'WARN'  as const, time: '14:22:44', service: 'queue-service',   message: 'Queue depth: 1847 (threshold: 1000)' },
]

export const tracesPreviewData = [
  { traceId: 'a3f9c2',  service: 'api-gateway',    operation: 'POST /checkout',      duration: '892ms', status: 'error' as const, spans: 12 },
  { traceId: 'b7d1e4',  service: 'api-gateway',    operation: 'GET /users/profile',  duration: '124ms', status: 'ok'    as const, spans: 4  },
  { traceId: 'c2a8f7',  service: 'payment-service',operation: 'processPayment',      duration: '445ms', status: 'ok'    as const, spans: 7  },
  { traceId: 'd5b3c1',  service: 'search-service', operation: 'search/query',        duration: '5012ms',status: 'error' as const, spans: 3  },
  { traceId: 'e9f4a2',  service: 'auth-service',   operation: 'validateToken',       duration: '23ms',  status: 'ok'    as const, spans: 2  },
  { traceId: 'f1c7b9',  service: 'user-service',   operation: 'getUserById',         duration: '312ms', status: 'slow'  as const, spans: 5  },
]

export const logLevelColors: Record<string, string> = {
  ERROR: '#ef4444',
  WARN: '#f59e0b',
  INFO: '#06b6d4',
  DEBUG: '#8b93a8',
  TRACE: '#4d566b',
}
