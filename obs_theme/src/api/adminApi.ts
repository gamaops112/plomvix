import { apiGet, apiPost, apiPatch, apiDelete } from './client'

export interface User {
  id: string
  username: string
  role: string
  created_at: string
  updated_at: string
}

export interface AdminStats {
  version: string
  env: string
  uptime_seconds: number
  pid: number
  go_version: string
  os_arch: string
  wal: { segment_count: number; active_segment: number; active_size_bytes: number; total_entries: number }
  hot: { total_writes: number; total_data_writes: number; data_dir: string }
  cold: { parquet_files: number; records_moved: number; last_flush_at: string; last_flush_duration_ms: number }
}

export interface AdminInfo {
  version: string
  build_time: string
  git_commit: string
  go_version: string
  os_arch: string
  uptime_seconds: number
}

export const listUsers       = ()                                               => apiGet<User[]>('/admin/users')
export const createUser      = (body: { username: string; password: string })  => apiPost<User>('/admin/users', body)
export const updateUser      = (id: string, body: { username?: string; password?: string }) =>
                                 apiPatch<User>(`/admin/users/${id}`, body)
export const deleteUser      = (id: string)                                     => apiDelete<{ message: string }>(`/admin/users/${id}`)
export const generateAPIKey  = (id: string)                                     => apiPost<{ api_key: string }>(`/admin/users/${id}/apikey`)
export const revokeAPIKey    = (id: string)                                     => apiDelete<{ message: string }>(`/admin/users/${id}/apikey`)
export const getAPIKeyStatus = (id: string)                                     => apiGet<{ has_key: boolean }>(`/admin/users/${id}/apikey/status`)
export const getAdminStats   = ()                                               => apiGet<AdminStats>('/admin/stats')
export const getAdminInfo    = ()                                               => apiGet<AdminInfo>('/admin/info')
