export interface Trace {
  id: string
  rootService: string
  rootOp: string
  duration: number
  spans: number
  errors: number
  status: 'error' | 'slow' | 'ok'
  time: string
}

export const mockTraces: Trace[] = [
  { id: 'a3f9c2b1', rootService: 'api-gateway',     rootOp: 'POST /checkout',        duration: 892,  spans: 12, errors: 2, status: 'error', time: '14:23:01' },
  { id: 'b7d1e4c2', rootService: 'api-gateway',     rootOp: 'GET /users/profile',    duration: 124,  spans: 4,  errors: 0, status: 'ok',    time: '14:22:58' },
  { id: 'c2a8f7d3', rootService: 'payment-service', rootOp: 'processPayment',        duration: 445,  spans: 7,  errors: 0, status: 'ok',    time: '14:22:55' },
  { id: 'd5b3c1e4', rootService: 'search-service',  rootOp: 'search/query',          duration: 5012, spans: 3,  errors: 1, status: 'error', time: '14:22:51' },
  { id: 'e9f4a2b5', rootService: 'auth-service',    rootOp: 'validateToken',         duration: 23,   spans: 2,  errors: 0, status: 'ok',    time: '14:22:49' },
  { id: 'f1c7b9d6', rootService: 'user-service',    rootOp: 'getUserById',           duration: 1312, spans: 5,  errors: 0, status: 'slow',  time: '14:22:47' },
  { id: 'g4e2a8c7', rootService: 'api-gateway',     rootOp: 'GET /products',         duration: 67,   spans: 3,  errors: 0, status: 'ok',    time: '14:22:44' },
  { id: 'h8d1f3b8', rootService: 'api-gateway',     rootOp: 'DELETE /cart/item',     duration: 234,  spans: 6,  errors: 0, status: 'ok',    time: '14:22:41' },
  { id: 'i2b9e7a9', rootService: 'notification-svc',rootOp: 'sendEmailNotification', duration: 891,  spans: 4,  errors: 1, status: 'error', time: '14:22:38' },
  { id: 'j6a4c2f0', rootService: 'payment-service', rootOp: 'refundTransaction',     duration: 2341, spans: 9,  errors: 0, status: 'slow',  time: '14:22:35' },
  { id: 'k1f8b6d1', rootService: 'api-gateway',     rootOp: 'GET /orders',           duration: 189,  spans: 5,  errors: 0, status: 'ok',    time: '14:22:32' },
  { id: 'l5c3a9e2', rootService: 'search-service',  rootOp: 'indexDocument',         duration: 3421, spans: 2,  errors: 1, status: 'error', time: '14:22:29' },
]

export interface SpanNode {
  id: string
  service: string
  operation: string
  startOffset: number
  duration: number
  status: 'ok' | 'error'
  depth: number
  children: SpanNode[]
}

export const serviceColors: Record<string, string> = {
  'api-gateway':      '#06b6d4',
  'auth-service':     '#8b5cf6',
  'user-service':     '#10b981',
  'payment-service':  '#f59e0b',
  'search-service':   '#f97316',
  'cache-service':    '#ec4899',
  'storage-service':  '#a78bfa',
  'notification-svc': '#34d399',
  'queue-service':    '#fbbf24',
  'analytics-svc':    '#60a5fa',
}

export const mockSpanTree = {
  traceId: 'a3f9c2b1',
  totalDuration: 892,
  rootService: 'api-gateway',
  rootOperation: 'POST /checkout',
  rootSpan: {
    id: 's1', service: 'api-gateway', operation: 'POST /checkout',
    startOffset: 0, duration: 892, status: 'error' as const, depth: 0,
    children: [
      {
        id: 's2', service: 'auth-service', operation: 'validateToken',
        startOffset: 12, duration: 45, status: 'ok' as const, depth: 1,
        children: [],
      },
      {
        id: 's3', service: 'user-service', operation: 'getUserById',
        startOffset: 58, duration: 312, status: 'ok' as const, depth: 1,
        children: [
          {
            id: 's4', service: 'cache-service', operation: 'cache.get',
            startOffset: 62, duration: 8, status: 'ok' as const, depth: 2, children: [],
          },
          {
            id: 's5', service: 'storage-service', operation: 'db.query',
            startOffset: 72, duration: 287, status: 'error' as const, depth: 2, children: [],
          },
        ],
      },
      {
        id: 's6', service: 'payment-service', operation: 'processPayment',
        startOffset: 380, duration: 445, status: 'ok' as const, depth: 1,
        children: [
          {
            id: 's7', service: 'storage-service', operation: 'db.query',
            startOffset: 390, duration: 398, status: 'ok' as const, depth: 2, children: [],
          },
        ],
      },
      {
        id: 's8', service: 'search-service', operation: 'search/query',
        startOffset: 124, duration: 512, status: 'error' as const, depth: 1,
        children: [
          {
            id: 's9', service: 'cache-service', operation: 'cache.get',
            startOffset: 128, duration: 4, status: 'ok' as const, depth: 2, children: [],
          },
        ],
      },
    ],
  } as SpanNode,
}
