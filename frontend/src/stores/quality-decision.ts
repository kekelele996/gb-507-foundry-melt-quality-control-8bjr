
import { createEntityStore } from './factory';
import type { QualityDecision } from '../types/domain';
export const useQualityDecisionStore = createEntityStore<QualityDecision>();
