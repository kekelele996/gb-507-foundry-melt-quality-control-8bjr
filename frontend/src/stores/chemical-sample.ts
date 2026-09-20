
import { createEntityStore } from './factory';
import type { ChemicalSample } from '../types/domain';
export const useChemicalSampleStore = createEntityStore<ChemicalSample>();
