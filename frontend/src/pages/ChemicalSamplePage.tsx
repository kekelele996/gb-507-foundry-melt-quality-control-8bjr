import { Typography } from '@mui/material';
import { EntityPage, type ColumnDefinition, type FieldDefinition } from '../components/EntityPage';
import { ChemistryTable } from '../components/common/ChemistryTable';
import { useChemicalSampleStore } from '../stores/chemical-sample';
import type { ChemicalSample } from '../types/domain';
import { SAMPLE_TRANSITIONS } from '../types/status';
import { formatDate } from '../utils/format';

const fields: readonly FieldDefinition[] = [
  { key: 'code', label: '样本编码', required: true }, { key: 'name', label: '样本名称', required: true },
  { key: 'heatCode', label: '炉次编码', required: true }, { key: 'samplePoint', label: '取样点', required: true },
  { key: 'methodVersion', label: '方法版本', required: true },
  { key: 'carbonPct', label: 'C（%）', type: 'number', required: true, min: 0, max: 6, step: 0.001, defaultValue: 3.3 },
  { key: 'siliconPct', label: 'Si（%）', type: 'number', required: true, min: 0, max: 6, step: 0.001, defaultValue: 2.0 },
  { key: 'manganesePct', label: 'Mn（%）', type: 'number', required: true, min: 0, max: 5, step: 0.001, defaultValue: 0.7 },
  { key: 'sulfurPct', label: 'S（%）', type: 'number', required: true, min: 0, max: 1, step: 0.001, defaultValue: 0.04 },
  { key: 'phosphorusPct', label: 'P（%）', type: 'number', required: true, min: 0, max: 1, step: 0.001, defaultValue: 0.07 },
  { key: 'sampledAt', label: '取样时间', type: 'datetime-local', required: true }, { key: 'evidence', label: 'LIMS 证据编号', required: true },
  { key: 'description', label: '备注', type: 'multiline' },
];

const columns: readonly ColumnDefinition<ChemicalSample>[] = [
  { key: 'relation', label: '炉次 / 取样点', minWidth: 140, render: (item) => <><Typography variant="body2" fontWeight={600}>{item.heatCode}</Typography><Typography variant="caption" color="text.secondary">{item.samplePoint}</Typography></> },
  { key: 'method', label: '方法 / 分析员', minWidth: 140, render: (item) => <><Typography variant="body2">{item.methodVersion}</Typography><Typography variant="caption" color="text.secondary">{item.analyst}</Typography></> },
  { key: 'chemistry', label: 'C / Si / Mn / S / P（%）', minWidth: 230, render: (item) => <Typography variant="caption">{item.carbonPct.toFixed(3)} / {item.siliconPct.toFixed(3)} / {item.manganesePct.toFixed(3)} / {item.sulfurPct.toFixed(3)} / {item.phosphorusPct.toFixed(3)}</Typography> },
  { key: 'sampled', label: '取样时间', minWidth: 160, render: (item) => formatDate(item.sampledAt) },
];

export default function ChemicalSamplePage() {
  const samples = useChemicalSampleStore((state) => state.items);
  return <EntityPage
    path="samples" label="化验结果" description="登记五元素光谱结果，复核方法版本、证据与终态。"
    useStore={useChemicalSampleStore} fields={fields} columns={columns} transitions={SAMPLE_TRANSITIONS}
    createRoles={['operator', 'reviewer', 'admin']} updateRoles={['operator', 'reviewer', 'admin']} transitionRoles={['reviewer', 'admin']}
    editableStatuses={['collected', 'testing']} deletableStatuses={['collected']}
    footer={<ChemistryTable records={samples} />}
  />;
}
