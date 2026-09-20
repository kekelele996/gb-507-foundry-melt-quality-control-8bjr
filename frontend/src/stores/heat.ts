
import { createEntityStore } from './factory';
import type { Heat } from '../types/domain';
export const useHeatStore = createEntityStore<Heat>();
