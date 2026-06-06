import {
  LayoutDashboard, FileText, GitBranch, Activity,
  Bell, Settings, FlaskConical, Cpu, Users,
  BookOpen, Layers, Radio, ShieldAlert
} from 'lucide-react'

export interface NavItem {
  label: string
  icon: typeof LayoutDashboard
  path: string
}

export interface NavSection {
  label: string
  items: NavItem[]
}

export const navSections: NavSection[] = [
  {
    label: 'Overview',
    items: [
      { label: 'Dashboard',   icon: LayoutDashboard, path: '/' },
    ],
  },
  {
    label: 'Observe',
    items: [
      { label: 'Logs',        icon: FileText,        path: '/logs' },
      { label: 'Traces',      icon: GitBranch,       path: '/traces' },
      { label: 'Metrics',     icon: Activity,        path: '/metrics' },
      { label: 'APM',         icon: Cpu,             path: '/apm' },
    ],
  },
  {
    label: 'Synthetics',
    items: [
      { label: 'Monitors',    icon: Radio,           path: '/synthetics' },
    ],
  },
  {
    label: 'Alerting',
    items: [
      { label: 'Alerts',      icon: Bell,            path: '/alerts' },
      { label: 'Incidents',   icon: ShieldAlert,     path: '/incidents' },
    ],
  },
  {
    label: 'Platform',
    items: [
      { label: 'Users',       icon: Users,           path: '/users' },
      { label: 'Integrations',icon: Layers,          path: '/integrations' },
      { label: 'Settings',    icon: Settings,        path: '/settings' },
    ],
  },
  {
    label: 'Developer',
    items: [
      { label: 'Demo Data',   icon: FlaskConical,    path: '/demo' },
      { label: 'Docs',        icon: BookOpen,        path: '/docs' },
    ],
  },
]
