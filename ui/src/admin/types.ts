export type UserRole = 'admin';

export interface AdminUser {
  id: string;
  username: string;
  role: UserRole;
  created_at: string;
  updated_at: string;
}

export interface CreateUserRequest {
  username: string;
  password: string;
}

export interface UpdateUserRequest {
  username?: string;
  password?: string;
}

export interface APIKeyStatus {
  user_id?: string;
  has_api_key: boolean;
}

export interface GeneratedAPIKey {
  api_key: string;
}

export interface AdminStats {
  [key: string]: unknown;
}

export interface AdminInfo {
  version?: string;
  build_time?: string;
  git_commit?: string;
  go_version?: string;
  uptime?: string;
  uptime_seconds?: number;
  [key: string]: unknown;
}
