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

// locked 是炉次放行合议合格时由后端原子写入的派生终态（类似炉次的
// accepted/rejected），不提供人工迁移入口，因此 verified 没有到 locked 的边。
export const SAMPLE_TRANSITIONS: Record<string, readonly string[]> = {
  collected: ['testing'], testing: ['verified', 'rejected'], verified: [], locked: [], rejected: [],
};

// 炉次放行合议：配对样本需为两份已复核（verified）样本，合格放行后双样本被
// 原子锁定为 locked 证据，不可再编辑或参与其它配对。
export type ReleasePairStatus = 'paired-ready' | 'pairing-short' | 'pairing-absent' | 'already-adjudged';
export const RELEASE_DECISIONS = ['accept', 'remelt', 'scrap'] as const;
export type ReleaseDecision = typeof RELEASE_DECISIONS[number];

export const DECISION_TRANSITIONS: Record<string, readonly string[]> = {
  draft: ['accept', 'remelt', 'scrap'], accept: [], remelt: [], scrap: [],
};
