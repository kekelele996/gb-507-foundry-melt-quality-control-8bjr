export type HeatState = 'charged' | 'melting' | 'sampling' | 'hold' | 'accepted' | 'rejected';
export const ALL_HEAT_STATE: readonly HeatState[] = ['charged', 'melting', 'sampling', 'hold', 'accepted', 'rejected'];
export type DecisionType = 'accept' | 'remelt' | 'scrap';
export const ALL_DECISION_TYPE: readonly DecisionType[] = ['accept', 'remelt', 'scrap'];

export const FURNACE_TRANSITIONS: Record<string, readonly string[]> = {
  available: ['charging', 'maintenance'], charging: ['available', 'maintenance'],
  maintenance: ['available', 'locked'], locked: ['maintenance'],
};

export const HEAT_TRANSITIONS: Record<string, readonly string[]> = {
  charged: ['melting'], melting: ['sampling'], sampling: ['hold'], hold: [], accepted: [], rejected: [],
};

export const SAMPLE_TRANSITIONS: Record<string, readonly string[]> = {
  collected: ['testing'], testing: ['verified', 'rejected'], verified: [], rejected: [],
};

export const DECISION_TRANSITIONS: Record<string, readonly string[]> = {
  draft: ['accept', 'remelt', 'scrap'], accept: [], remelt: [], scrap: [],
};
