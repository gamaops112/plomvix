export type UserRole = 'admin';

export interface AuthUser {
  id: string;
  username: string;
  role: UserRole;
  created_at: string;
  updated_at: string;
}

export interface LoginRequest {
  username: string;
  password: string;
}

export interface LoginResponseData {
  token?: string;
  user: AuthUser;
  expires_at?: string;
}

export interface AuthState {
  user: AuthUser | null;
  loading: boolean;
  authenticated: boolean;
}
