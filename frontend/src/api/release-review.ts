import { request } from './client';
import type { Heat, ReleaseAdjudicationRequest, ReleasePanel, QualityDecision } from '../types/domain';

// 炉次放行合议 API：面板候选炉次、配对/读数/阻塞原因视图，以及一次性原子判定。
export function listPanelHeats(): Promise<Heat[]> {
  return request<Heat[]>('/release-panels').then((result) => result.data);
}

export function getReleasePanel(heatCode: string): Promise<ReleasePanel> {
  return request<ReleasePanel>(`/release-panels/${encodeURIComponent(heatCode)}`).then((result) => result.data);
}

export function adjudicateRelease(input: ReleaseAdjudicationRequest): Promise<QualityDecision> {
  return request<QualityDecision>('/release-adjudications', {
    method: 'POST',
    body: JSON.stringify(input),
  }).then((result) => result.data);
}
