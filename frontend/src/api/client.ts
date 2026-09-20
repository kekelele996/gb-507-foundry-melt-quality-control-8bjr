import type { ApiEnvelope, UserSession } from '../types/domain';

const SESSION_KEY = 'foundry-quality-session';

export class ApiError extends Error {
  constructor(public readonly status: number, public readonly code: string, message: string) {
    super(message);
    this.name = 'ApiError';
  }
}

export function readSession(): UserSession | null {
  try {
    const value = JSON.parse(localStorage.getItem(SESSION_KEY) || 'null') as UserSession | null;
    return value?.token ? value : null;
  } catch {
    return null;
  }
}

export function getToken(): string {
  return readSession()?.token || '';
}

export function saveSession(session: UserSession): void {
  localStorage.setItem(SESSION_KEY, JSON.stringify(session));
}

export function clearSession(): void {
  localStorage.removeItem(SESSION_KEY);
}

export async function request<T>(path: string, init: RequestInit = {}): Promise<ApiEnvelope<T>> {
  const headers = new Headers(init.headers);
  headers.set('Accept', 'application/json');
  if (init.body) headers.set('Content-Type', 'application/json');
  const token = getToken();
  if (token) headers.set('Authorization', `Bearer ${token}`);

  let response: Response;
  try {
    response = await fetch(`/api${path}`, { ...init, headers });
  } catch {
    throw new ApiError(0, 'network_error', '无法连接服务，请检查服务状态');
  }
  if (response.status === 204) return { data: undefined as T };

  const payload = await response.json().catch(() => ({ error: 'invalid_response', message: '服务返回了无法解析的响应' }));
  if (!response.ok) {
    if (response.status === 401 && path !== '/auth/login') {
      clearSession();
      window.dispatchEvent(new Event('foundry:session-expired'));
    }
    throw new ApiError(response.status, payload.error || 'request_failed', payload.message || payload.error || `HTTP ${response.status}`);
  }
  return payload as ApiEnvelope<T>;
}
