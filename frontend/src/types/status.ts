export type HeatState = 'charged' | 'melting' | 'sampling' | 'hold' | 'accepted' | 'rejected';
export const ALL_HEAT_STATE: readonly HeatState[] = ['charged', 'melting', 'sampling', 'hold', 'accepted', 'rejected'];
export type DecisionType = 'accept' | 'remelt' | 'scrap';
export const ALL_DECISION_TYPE: readonly DecisionType[] = ['accept', 'remelt', 'scrap'];

export type ReleaseReviewState = 'open' | 'accepted' | 'remelted' | 'scrapped';
export const ALL_RELEASE_REVIEW_STATE: readonly ReleaseReviewState[] = ['open', 'accepted', 'remelted', 'scrapped'];

// PairingState is the read-model state shown on the quality board.
export type PairingState = 'incomplete' | 'blocked' | 'ready' | 'locked-in' | 'closed';

export const FURNACE_TRANSITIONS: Record<string, readonly string[]> = {
  available: ['charging', 'maintenance'], charging: ['available', 'maintenance'],
  maintenance: ['available', 'locked'], locked: ['maintenance'],
};

export const HEAT_TRANSITIONS: Record<string, readonly string[]> = {
  charged: ['melting'], melting: ['sampling'], sampling: ['hold'], hold: [], accepted: [], rejected: [],
};

export const SAMPLE_TRANSITIONS: Record<string, readonly string[]> = {
  collected: ['testing'], testing: ['verified', 'rejected'], verified: ['locked'], locked: [], rejected: [],
};

export const DECISION_TRANSITIONS: Record<string, readonly string[]> = {
  draft: ['accept', 'remelt', 'scrap'], accept: [], remelt: [], scrap: [],
};

// A joint review leaves "open" exactly once, directly to a terminal verdict.
export const RELEASE_REVIEW_TRANSITIONS: Record<string, readonly ReleaseReviewState[]> = {
  open: ['accepted', 'remelted', 'scrapped'], accepted: [], remelted: [], scrapped: [],
};
