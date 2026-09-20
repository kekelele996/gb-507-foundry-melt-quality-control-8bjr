import { request } from './client';
import type { HeatReleaseReview, ReleasePairingView } from '../types/domain';

export async function listReleaseReviews(page = 1, pageSize = 20, search = '', status = '') {
  const query = new URLSearchParams({ page: String(page), pageSize: String(pageSize), search });
  if (status) query.set('status', status);
  return request<HeatReleaseReview[]>(`/release-reviews?${query.toString()}`);
}

export async function getReleaseBoard() {
  return request<ReleasePairingView[]>('/release-board');
}

export async function openReleaseReview(heatCode: string, name: string, description = '') {
  return request<HeatReleaseReview>('/release-reviews', {
    method: 'POST',
    body: JSON.stringify({ heatCode, name, description }),
  });
}

export type ReleaseVerdict = 'accepted' | 'remelted' | 'scrapped';

export async function judgeReleaseReview(id: number, target: ReleaseVerdict, expectedVersion: number, reason = '') {
  return request<HeatReleaseReview>(`/release-reviews/${id}/judge`, {
    method: 'POST',
    body: JSON.stringify({ target, expectedVersion, reason }),
  });
}

export async function withdrawReleaseReview(id: number) {
  return request<void>(`/release-reviews/${id}`, { method: 'DELETE' });
}
