export interface FiringAlert {
  id: string
  severity: 'critical' | 'high' | 'warning' | 'info'
  name: string
  service: string
  condition: string
  value: string
  duration: string
  started: string
  status: 'firing' | 'resolved'
  assignee: string | null
  silenced: boolean
}

export const mockFiringAlerts: FiringAlert[] = [
  { id: 'al1', severity: 'critical', name: 'search-service Down', service: 'search-service', condition: 'Error rate > 5%', value: '8.4%', duration: '12m', started: '14:11:00', status: 'firing', assignee: null, silenced: false },
  { id: 'al2', severity: 'critical', name: 'High DB Latency', service: 'storage-service', condition: 'P99 latency > 2s', value: '4.2s', duration: '8m', started: '14:15:00', status: 'firing', assignee: 'John D.', silenced: false },
  { id: 'al3', severity: 'high', name: 'User Service Slow', service: 'user-service', condition: 'P95 latency > 500ms', value: '891ms', duration: '22m', started: '14:01:00', status: 'firing', assignee: null, silenced: false },
  { id: 'al4', severity: 'high', name: 'Queue Depth Threshold', service: 'queue-service', condition: 'Queue depth > 1000', value: '1,847', duration: '15m', started: '14:08:00', status: 'firing', assignee: null, silenced: false },
  { id: 'al5', severity: 'high', name: 'Payment Timeout Spike', service: 'payment-service', condition: 'Timeout rate > 1%', value: '2.3%', duration: '5m', started: '14:18:00', status: 'firing', assignee: 'Jane S.', silenced: false },
  { id: 'al6', severity: 'warning', name: 'High Memory Usage', service: 'api-gateway', condition: 'Memory usage > 80%', value: '84%', duration: '34m', started: '13:49:00', status: 'firing', assignee: null, silenced: false },
  { id: 'al7', severity: 'warning', name: 'CPU Spike Detected', service: 'user-service', condition: 'CPU usage > 70%', value: '78%', duration: '18m', started: '14:05:00', status: 'firing', assignee: null, silenced: true },
  { id: 'al8', severity: 'info', name: 'Auto-scaling Triggered', service: 'api-gateway', condition: 'Instance count changed', value: '+2', duration: '2m', started: '14:21:00', status: 'firing', assignee: null, silenced: false },
]

export interface AlertRule {
  id: string
  name: string
  service: string
  condition: string
  severity: 'critical' | 'high' | 'warning' | 'info'
  status: 'enabled' | 'disabled'
  lastFired: string
  notifications: string[]
}

export const mockAlertRules: AlertRule[] = [
  { id: 'r1', name: 'High Error Rate', service: 'any', condition: 'error_rate > 5%', severity: 'critical', status: 'enabled', lastFired: '12m ago', notifications: ['slack', 'pagerduty'] },
  { id: 'r2', name: 'High Latency P99', service: 'any', condition: 'latency_p99 > 2s', severity: 'critical', status: 'enabled', lastFired: '8m ago', notifications: ['slack', 'pagerduty'] },
  { id: 'r3', name: 'Service Down', service: 'any', condition: 'availability < 99%', severity: 'critical', status: 'enabled', lastFired: '2h ago', notifications: ['slack', 'pagerduty', 'email'] },
  { id: 'r4', name: 'High Memory Usage', service: 'any', condition: 'memory_usage > 80%', severity: 'warning', status: 'enabled', lastFired: '34m ago', notifications: ['slack'] },
  { id: 'r5', name: 'CPU Spike', service: 'any', condition: 'cpu_usage > 70%', severity: 'warning', status: 'enabled', lastFired: '18m ago', notifications: ['slack'] },
  { id: 'r6', name: 'Queue Depth', service: 'queue-service', condition: 'queue_depth > 1000', severity: 'high', status: 'enabled', lastFired: '15m ago', notifications: ['slack'] },
  { id: 'r7', name: 'Payment Timeout', service: 'payment-service', condition: 'timeout_rate > 1%', severity: 'high', status: 'enabled', lastFired: '5m ago', notifications: ['slack', 'pagerduty'] },
  { id: 'r8', name: 'Disk Space Low', service: 'any', condition: 'disk_usage > 85%', severity: 'warning', status: 'disabled', lastFired: '3d ago', notifications: ['email'] },
  { id: 'r9', name: 'SSL Cert Expiry', service: 'any', condition: 'cert_expiry < 30d', severity: 'info', status: 'enabled', lastFired: 'never', notifications: ['email'] },
  { id: 'r10', name: 'Auto-scale Event', service: 'any', condition: 'instance_count_changed', severity: 'info', status: 'enabled', lastFired: '2m ago', notifications: ['slack'] },
]

export const mockChannels = [
  { id: 'c1', type: 'slack', name: 'Slack #alerts', status: 'connected', lastTest: '2h ago' },
  { id: 'c2', type: 'slack', name: 'Slack #incidents', status: 'connected', lastTest: '1d ago' },
  { id: 'c3', type: 'pagerduty', name: 'PagerDuty On-call', status: 'connected', lastTest: '6h ago' },
  { id: 'c4', type: 'email', name: 'Ops Team Email', status: 'connected', lastTest: '3d ago' },
  { id: 'c5', type: 'webhook', name: 'Custom Webhook', status: 'error', lastTest: '1h ago' },
]

export const severityColors: Record<string, string> = {
  critical: '#ef4444',
  high: '#f97316',
  warning: '#f59e0b',
  info: '#06b6d4',
  resolved: '#10b981',
}

export const generateTimeSeries = (base: number, variance: number, points = 60): number[] =>
  Array.from({ length: points }, () =>
    +(base + (Math.random() - 0.5) * variance * 2).toFixed(2)
  )
