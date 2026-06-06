export interface ServiceData {
  name: string
  status: 'healthy' | 'degraded' | 'down'
  reqRate: number
  errorRate: number
  p50: number
  p95: number | null
  p99: number | null
  instances: number
}

export const mockServices: ServiceData[] = [
  { name: 'api-gateway',      status: 'healthy',  reqRate: 4821, errorRate: 0.4, p50: 45,   p95: 124,  p99: 312,  instances: 3 },
  { name: 'auth-service',     status: 'healthy',  reqRate: 2043, errorRate: 0.1, p50: 23,   p95: 67,   p99: 98,   instances: 2 },
  { name: 'user-service',     status: 'degraded', reqRate: 1932, errorRate: 1.8, p50: 312,  p95: 891,  p99: 1200, instances: 2 },
  { name: 'payment-service',  status: 'healthy',  reqRate: 721,  errorRate: 0.2, p50: 67,   p95: 198,  p99: 445,  instances: 2 },
  { name: 'search-service',   status: 'down',     reqRate: 443,  errorRate: 8.4, p50: 5012, p95: null, p99: null, instances: 1 },
  { name: 'cache-service',    status: 'healthy',  reqRate: 8211, errorRate: 0.0, p50: 4,    p95: 12,   p99: 18,   instances: 3 },
  { name: 'queue-service',    status: 'degraded', reqRate: 987,  errorRate: 0.3, p50: 445,  p95: 892,  p99: 1400, instances: 2 },
  { name: 'storage-service',  status: 'healthy',  reqRate: 3421, errorRate: 0.1, p50: 89,   p95: 234,  p99: 445,  instances: 4 },
  { name: 'notification-svc', status: 'healthy',  reqRate: 234,  errorRate: 0.8, p50: 18,   p95: 45,   p99: 98,   instances: 1 },
]

export const staticErrorData = [
  { name: 'api-gateway', errors: 67, errorPct: 0.4 },
  { name: 'auth-service', errors: 34, errorPct: 0.1 },
  { name: 'user-service', errors: 312, errorPct: 1.8 },
  { name: 'payment-service', errors: 89, errorPct: 0.2 },
  { name: 'search-service', errors: 1241, errorPct: 8.4 },
  { name: 'cache-service', errors: 2, errorPct: 0.0 },
  { name: 'queue-service', errors: 71, errorPct: 0.3 },
]

export const mockErrors = [
  { id: 1, message: 'Connection refused: redis:6379',        service: 'search-service',  count: 1241, users: null, firstSeen: '2d ago', lastSeen: '2m ago' },
  { id: 2, message: 'Response timeout after 5000ms',         service: 'user-service',    count: 312,  users: 89,  firstSeen: '5h ago', lastSeen: '8m ago' },
  { id: 3, message: 'Payment gateway rejected charge',       service: 'payment-service', count: 89,   users: 67,  firstSeen: '1d ago', lastSeen: '34m ago' },
  { id: 4, message: 'JWT token signature invalid',           service: 'auth-service',    count: 34,   users: 34,  firstSeen: '3h ago', lastSeen: '1h ago' },
]

export const generateTimeSeries = (base: number, variance: number, points = 60): number[] =>
  Array.from({ length: points }, () =>
    +(base + (Math.random() - 0.5) * variance * 2).toFixed(2)
  )
