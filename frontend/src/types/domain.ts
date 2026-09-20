export type UserRole = 'viewer' | 'operator' | 'reviewer' | 'admin';

export interface BaseRecord {
  id: number;
  code: string;
  name: string;
  status: string;
  version: number;
  description: string;
  createdAt: string;
  updatedAt: string;
}

// Generic API helpers use the shared base contract when a page does not need
// the entity-specific fields. Keeping the alias here makes those helpers
// type-safe without duplicating a second, divergent record shape.
export type DomainRecord = BaseRecord;

export interface Furnace extends BaseRecord {
  plantArea: string;
  furnaceType: string;
  capacityTonnes: number;
  supportedAlloys: string;
  maxTemperatureC: number;
  operator: string;
  lastInspectionAt: string;
  evidence: string;
}

export interface Heat extends BaseRecord {
  furnaceCode: string;
  alloyGrade: string;
  owner: string;
  chargeWeightKg: number;
  targetTemperatureC: number;
  carbonMinPct: number;
  carbonMaxPct: number;
  siliconMinPct: number;
  siliconMaxPct: number;
  sulfurMaxPct: number;
  phosphorusMaxPct: number;
  startedAt: string;
  evidence: string;
}

export interface ChemicalSample extends BaseRecord {
  heatCode: string;
  samplePoint: string;
  methodVersion: string;
  analyst: string;
  carbonPct: number;
  siliconPct: number;
  manganesePct: number;
  sulfurPct: number;
  phosphorusPct: number;
  sampledAt: string;
  evidence: string;
}

export interface QualityDecision extends BaseRecord {
  heatCode: string;
  sampleCode: string;
  reviewer: string;
  reason: string;
  conditions: string;
  decidedAt: string;
  evidence: string;
}

export interface HeatReleaseReview extends BaseRecord {
  heatCode: string;
  firstSampleId: number;
  secondSampleId: number;
  firstSampleCode: string;
  secondSampleCode: string;
  alloyGrade: string;
  reviewer: string;
  verdictReason: string;
  decidedAt: string;
  firstCarbonPct: number;
  secondCarbonPct: number;
  firstSiliconPct: number;
  secondSiliconPct: number;
  carbonDeltaPct: number;
  siliconDeltaPct: number;
  pairingBlockers: string;
}

export interface ReleaseSampleView {
  id: number;
  code: string;
  status: string;
  carbonPct: number;
  siliconPct: number;
  sulfurPct: number;
  phosphorusPct: number;
  manganesePct: number;
  sampledAt: string;
}

export interface ReleasePairingView {
  heatCode: string;
  heatName: string;
  heatStatus: string;
  heatVersion: number;
  alloyGrade: string;
  carbonRange: [number, number];
  siliconRange: [number, number];
  sulfurMaxPct: number;
  phosphorusMaxPct: number;
  pairingState: 'incomplete' | 'blocked' | 'ready' | 'locked-in' | 'closed';
  verifiedCount: number;
  firstSample: ReleaseSampleView | null;
  secondSample: ReleaseSampleView | null;
  carbonDeltaPct: number;
  siliconDeltaPct: number;
  carbonDeltaOk: boolean;
  siliconDeltaOk: boolean;
  blockers: string[];
  reviewId: number;
  reviewCode: string;
  reviewStatus: string;
  reviewVersion: number;
  reviewer: string;
  decisionCode: string;
  decisionStatus: string;
}

export interface PageMeta {
  page: number;
  pageSize: number;
  total: number;
}

export interface ApiEnvelope<T> {
  data: T;
  error?: string;
  message?: string;
  meta?: PageMeta;
}

export interface UserSession {
  token: string;
  username: string;
  displayName: string;
  role: UserRole;
  expiresIn: number;
}

export interface AuditLog {
  id: number;
  requestId: string;
  actor: string;
  action: string;
  entityType: string;
  entityId: number;
  beforeState: string;
  afterState: string;
  detail: string;
  createdAt: string;
}
