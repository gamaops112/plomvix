export interface Incident {
  id: string
  title: string
  severity: 'critical' | 'high' | 'warning'
  status: 'open' | 'investigating' | 'resolved'
  affectedServices: string[]
  startedAt: string
  duration: string
  assignee: string | null
  alerts: number
  updates: number
}

export const mockIncidents: Incident[] = [
  { id: 'inc1', title: 'search-service outage', severity: 'critical', status: 'open', affectedServices: ['search-service', 'api-gateway'], startedAt: '14:11:00', duration: '12m', assignee: 'John D.', alerts: 2, updates: 3 },
  { id: 'inc2', title: 'Elevated latency across user-service', severity: 'high', status: 'investigating', affectedServices: ['user-service', 'storage-service'], startedAt: '14:01:00', duration: '22m', assignee: 'Jane S.', alerts: 3, updates: 5 },
  { id: 'inc3', title: 'Payment service timeout spike', severity: 'high', status: 'investigating', affectedServices: ['payment-service'], startedAt: '13:55:00', duration: '28m', assignee: null, alerts: 1, updates: 2 },
  { id: 'inc4', title: 'API gateway memory pressure', severity: 'warning', status: 'resolved', affectedServices: ['api-gateway'], startedAt: '12:30:00', duration: '45m', assignee: 'John D.', alerts: 1, updates: 4 },
]

export const timelineEvents = [
  { type: 'note', time: '14:23', actor: 'John D.', text: 'Checking Redis cluster health', color: '#06b6d4' },
  { type: 'acknowledged', time: '14:19', actor: 'System', text: 'Alert acknowledged', color: '#8b5cf6' },
  { type: 'assignment', time: '14:15', actor: 'System', text: 'John D. assigned as incident commander', color: '#8b5cf6' },
  { type: 'status_change', time: '14:13', actor: 'System', text: 'Incident severity updated: warning → critical', color: '#f59e0b' },
  { type: 'created', time: '14:11', actor: 'System', text: 'Incident created automatically from alert', color: '#f97316' },
  { type: 'alert', time: '14:11', actor: 'System', text: 'Alert fired: search-service Down (error rate 8.4%)', color: '#ef4444' },
]
