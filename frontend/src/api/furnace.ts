
import { request } from './client';
import type { DomainRecord } from '../types/domain';

export async function listFurnace(page = 1, pageSize = 20, search = '') {
  return request<DomainRecord[]>(`/furnaces?page=${page}&pageSize=${pageSize}&search=${encodeURIComponent(search)}`);
}
export async function createFurnace(input: Partial<DomainRecord>) {
  return request<DomainRecord>('/furnaces', { method: 'POST', body: JSON.stringify(input) });
}
export async function transitionFurnace(id: number, status: string, expectedVersion: number, reason: string) {
  return request<DomainRecord>(`/furnaces/${id}/transition`, {
    method: 'POST', body: JSON.stringify({ status, expectedVersion, reason }),
  });
}
