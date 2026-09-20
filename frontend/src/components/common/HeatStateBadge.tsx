import type { HeatState } from '../../types/status';
import { StatusBadge } from './StatusBadge';

export function HeatStateBadge({ state }: { state: HeatState }) {
  return <StatusBadge status={state} />;
}
