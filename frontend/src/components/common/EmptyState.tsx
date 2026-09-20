
import type { BaseRecord } from '../../types/domain';
import { StatusBadge } from './StatusBadge';
export function EmptyState({ records }: { records: BaseRecord[] }) {
  if (!records.length) return <div className="empty">暂无可展示的业务证据</div>;
  return <div className="evidence-strip">{records.slice(0, 4).map((item) => <article key={item.id}><strong>{item.code}</strong><span>{item.name}</span><StatusBadge status={item.status} /></article>)}</div>;
}
