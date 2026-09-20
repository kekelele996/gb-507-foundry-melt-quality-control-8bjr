
import { request } from './client';
import type { DomainRecord } from '../types/domain';

export async function listHeat(page = 1, pageSize = 20, search = '') {
  return request<DomainRecord[]>(`/heats?page=${page}&pageSize=${pageSize}&search=${encodeURIComponent(search)}`);
}
export async function createHeat(input: Partial<DomainRecord>) {
  return request<DomainRecord>('/heats', { method: 'POST', body: JSON.stringify(input) });
}
export async function transitionHeat(id: number, status: string, expectedVersion: number, reason: string) {
  return request<DomainRecord>(`/heats/${id}/transition`, {
    method: 'POST', body: JSON.stringify({ status, expectedVersion, reason }),
  });
}
