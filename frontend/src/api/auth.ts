import { request } from './client';
import type { UserSession } from '../types/domain';

export async function login(username: string, password: string): Promise<UserSession> {
  return (await request<UserSession>('/auth/login', {
    method: 'POST', body: JSON.stringify({ username: username.trim(), password }),
  })).data;
}
