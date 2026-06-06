export interface Host {
  id: string
  name: string
  os: string | null
  cpu: number
  diskLatency: number
  rx: number
  tx: number
  memTotal: number
  memUsage: number
}

export const mockHosts: Host[] = [
  { id: 'h1',  name: '041ecb195a9f',  os: null,     cpu: 0,    diskLatency: 0,    rx: 0,     tx: 0,     memTotal: 0,   memUsage: 0    },
  { id: 'h2',  name: '69e2aeee9842',  os: null,     cpu: 0,    diskLatency: 0,    rx: 0,     tx: 0,     memTotal: 0,   memUsage: 0    },
  { id: 'h3',  name: '94ebaa02dec7',  os: null,     cpu: 0,    diskLatency: 0,    rx: 0,     tx: 0,     memTotal: 0,   memUsage: 0    },
  { id: 'h4',  name: 'gke-demo-co-1', os: 'Ubuntu', cpu: 23.7, diskLatency: 4.9,  rx: 11.1,  tx: 14.5,  memTotal: 16.8, memUsage: 49.9 },
  { id: 'h5',  name: 'gke-demo-co-2', os: 'Ubuntu', cpu: 13.1, diskLatency: 1.4,  rx: 16.0,  tx: 20.4,  memTotal: 16.8, memUsage: 63.6 },
  { id: 'h6',  name: 'gke-demo-co-3', os: 'Ubuntu', cpu: 20.6, diskLatency: 3.7,  rx: 13.5,  tx: 41.9,  memTotal: 16.8, memUsage: 49.0 },
  { id: 'h7',  name: 'ip-192-168-1',  os: 'Ubuntu', cpu: 3.6,  diskLatency: 5.2,  rx: 121.3, tx: 172.2, memTotal: 16.8, memUsage: 8.0  },
  { id: 'h8',  name: 'ip-192-168-2',  os: 'Ubuntu', cpu: 67.2, diskLatency: 12.1, rx: 45.2,  tx: 38.7,  memTotal: 32.0, memUsage: 78.4 },
  { id: 'h9',  name: 'ip-192-168-3',  os: 'CentOS',cpu: 45.8, diskLatency: 8.3,  rx: 22.4,  tx: 19.8,  memTotal: 8.0,  memUsage: 54.2 },
  { id: 'h10', name: 'ip-192-168-4',  os: 'Debian',cpu: 8.2,  diskLatency: 2.1,  rx: 5.4,   tx: 4.2,   memTotal: 16.0, memUsage: 31.7 },
  { id: 'h11', name: 'ip-192-168-5',  os: 'Ubuntu', cpu: 91.4, diskLatency: 22.7, rx: 87.3,  tx: 92.1,  memTotal: 64.0, memUsage: 88.9 },
]

export const generateTimeSeries = (base: number, variance: number, points = 60): number[] =>
  Array.from({ length: points }, () =>
    +(base + (Math.random() - 0.5) * variance * 2).toFixed(2)
  )

export const metricsTimestamps = Array.from({ length: 60 }, (_, i) => {
  const d = new Date()
  d.setMinutes(d.getMinutes() - (59 - i))
  return d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
})

export const metricsExplorerData = {
  timestamps: metricsTimestamps,
  series: {
    'host-prod-01': generateTimeSeries(23.7, 8),
    'host-prod-02': generateTimeSeries(67.2, 12),
    'host-prod-03': generateTimeSeries(45.8, 10),
    'host-staging': generateTimeSeries(8.2, 5),
  },
}

export const hostCardsData = [
  { label: 'Hosts',            value: '11',       unit: '',          color: '#06b6d4', sparkline: generateTimeSeries(11, 1, 20)   },
  { label: 'CPU Usage (avg)',  value: '12.28',    unit: '%',         color: '#f59e0b', sparkline: generateTimeSeries(12.28, 4, 20) },
  { label: 'Memory Usage (avg)',value: '36.03',   unit: '%',         color: '#8b5cf6', sparkline: generateTimeSeries(36.03, 6, 20) },
  { label: 'Network In (RX)',  value: '8.19',     unit: 'Mbit',      color: '#06b6d4', sparkline: generateTimeSeries(8.19, 2, 20)  },
  { label: 'Network Out (TX)', value: '15.41',    unit: 'Mbit',      color: '#f97316', sparkline: generateTimeSeries(15.41, 4, 20) },
]

export const metricOptions = [
  'system.cpu.usage',
  'system.memory.usage',
  'system.network.in.bytes',
  'system.network.out.bytes',
  'system.disk.io.read',
  'system.disk.io.write',
  'system.load.1',
  'system.load.5',
  'system.load.15',
  'service.request.rate',
  'service.error.rate',
  'service.latency.p50',
  'service.latency.p95',
  'service.latency.p99',
]

export const aggOptions = ['avg', 'sum', 'min', 'max', 'count', 'p50', 'p95', 'p99']
export const groupByOptions = ['host', 'service', 'region', 'os', 'cloud_provider']

export const mockProcesses = [
  { pid: 1842, name: 'node',     cpu: 18.2, mem: 4.2,  status: 'running' },
  { pid: 2341, name: 'python3',  cpu: 12.4, mem: 2.8,  status: 'running' },
  { pid: 891,  name: 'nginx',    cpu: 2.1,  mem: 0.4,  status: 'running' },
  { pid: 3421, name: 'redis',    cpu: 1.8,  mem: 1.2,  status: 'running' },
  { pid: 4892, name: 'postgres', cpu: 8.7,  mem: 6.4,  status: 'running' },
]

export const serviceMetrics = [
  { service: 'api-gateway',     reqs: 4821, errorPct: 0.4, p50: 45,  p95: 124,  p99: 312 },
  { service: 'auth-service',    reqs: 2043, errorPct: 0.1, p50: 23,  p95: 67,   p99: 98  },
  { service: 'user-service',    reqs: 1932, errorPct: 1.8, p50: 312, p95: 891,  p99: 1200 },
  { service: 'payment-service', reqs: 721,  errorPct: 0.2, p50: 67,  p95: 198,  p99: 445 },
  { service: 'search-service',  reqs: 443,  errorPct: 8.4, p50: 5012,p95: 0,    p99: 0   },
  { service: 'cache-service',   reqs: 8211, errorPct: 0.0, p50: 4,   p95: 12,   p99: 18  },
  { service: 'queue-service',   reqs: 987,  errorPct: 0.3, p50: 445, p95: 892,  p99: 1400 },
]

export const presetMetrics = [
  { label: 'CPU Usage',   metric: 'system.cpu.usage',       unit: '%',    color: '#06b6d4' },
  { label: 'Memory Usage',metric: 'system.memory.usage',     unit: '%',    color: '#8b5cf6' },
  { label: 'Network In',  metric: 'system.network.in.bytes', unit: 'MB/s', color: '#10b981' },
  { label: 'Disk IO',     metric: 'system.disk.io.read',     unit: 'MB/s', color: '#f59e0b' },
]

export function getUsageColor(value: number): string {
  if (value > 80) return '#ef4444'
  if (value > 50) return '#f59e0b'
  return '#10b981'
}
