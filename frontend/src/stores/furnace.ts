
import { createEntityStore } from './factory';
import type { Furnace } from '../types/domain';
export const useFurnaceStore = createEntityStore<Furnace>();
