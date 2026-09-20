import { useEffect, useState } from 'react';
import { Typography } from '@mui/material';
import { request } from '../api/client';
import { EntityPage, type ColumnDefinition } from '../components/EntityPage';
import { ReleaseReviewPanel } from '../components/release/ReleaseReviewPanel';
import { ChemistryTable } from '../components/common/ChemistryTable';
import { HeatStateBadge } from '../components/common/HeatStateBadge';
import { useQualityDecisionStore } from '../stores/quality-decision';
import type { ChemicalSample, QualityDecision } from '../types/domain';
import { DECISION_TRANSITIONS, type HeatState } from '../types/status';
import { formatDate } from '../utils/format';

const columns: readonly ColumnDefinition<QualityDecision>[] = [
  { key: 'relation', label: '炉次 / 配对样本', minWidth: 170, render: (item) => <>
    <Typography variant="body2" fontWeight={600}>{item.heatCode}</Typography>
    <Typography variant="caption" color="text.secondary">{item.sampleCode}{item.pairedSampleCode ? ` + ${item.pairedSampleCode}` : ''}</Typography>
  </> },
  { key: 'reviewer', label: '复核人 / 时间', minWidth: 170, render: (item) => <>
    <Typography variant="body2">{item.reviewer}</Typography>
    <Typography variant="caption" color="text.secondary">{formatDate(item.decidedAt)}</Typography>
  </> },
  { key: 'reason', label: '判定依据 / 原因', minWidth: 260, render: (item) => <Typography variant="body2" className="cell-wrap">{item.reason}</Typography> },
  { key: 'evidence', label: '签发证据', minWidth: 160, render: (item) => <Typography variant="caption">{item.evidence}</Typography> },
];

function derivedHeatState(status: string): HeatState {
  if (status === 'accept') return 'accepted';
  if (status === 'remelt' || status === 'scrap') return 'rejected';
  return 'hold';
}

export default function QualityDecisionPage() {
  const [samples, setSamples] = useState<ChemicalSample[]>([]);
  useEffect(() => {
    request<ChemicalSample[]>('/samples?page=1&pageSize=20').then((result) => setSamples(result.data)).catch(() => setSamples([]));
  }, []);
  return <>
    <main className="workspace" style={{ paddingBottom: 0 }}>
      <header className="page-header">
        <div>
          <p className="eyebrow">生产质量控制</p>
          <h1>质量判定</h1>
          <p>炉次放行合议：双样本配对、四元素牌号范围与 C/Si 复现允差全部满足才可合格放行，否则返炉或报废并登记原因。</p>
        </div>
      </header>
      <ReleaseReviewPanel />
    </main>
    <div className="workspace" style={{ paddingTop: 0 }}>
      <EntityPage
        path="decisions" label="质量决定台账" description="已落盘的放行决定（含历史草稿），合格/返炉/报废均在炉次放行合议中提交。"
        useStore={useQualityDecisionStore} fields={[]} columns={columns} transitions={DECISION_TRANSITIONS}
        createRoles={[]} updateRoles={[]} transitionRoles={['reviewer', 'admin']}
        editableStatuses={['draft']} deletableStatuses={['draft']}
        statusRender={(item) => <span className="decision-status"><span>{item.status}</span><HeatStateBadge state={derivedHeatState(item.status)} /></span>}
        footer={<ChemistryTable records={samples} title="最近化验明细（含已锁定放行证据）" />}
      />
    </div>
  </>;
}
