import { Typography } from '@mui/material';
import { EntityPage, type ColumnDefinition, type FieldDefinition } from '../components/EntityPage';
import type { Furnace } from '../types/domain';
import { FURNACE_TRANSITIONS } from '../types/status';
import { formatDate } from '../utils/format';
import { useFurnaceStore } from '../stores/furnace';

const fields: readonly FieldDefinition[] = [
  { key: 'code', label: '炉台编码', required: true }, { key: 'name', label: '炉台名称', required: true },
  { key: 'plantArea', label: '厂区 / 车间', required: true }, { key: 'furnaceType', label: '炉型', required: true },
  { key: 'capacityTonnes', label: '容量（吨）', type: 'number', required: true, min: 0.1, max: 500, step: 0.1, defaultValue: 10 },
  { key: 'maxTemperatureC', label: '最高温度（°C）', type: 'number', required: true, min: 500, max: 2200, step: 1, defaultValue: 1600 },
  { key: 'supportedAlloys', label: '支持牌号（逗号分隔）', required: true }, { key: 'operator', label: '责任班组', required: true },
  { key: 'lastInspectionAt', label: '最近检验时间', type: 'datetime-local', required: true },
  { key: 'evidence', label: '检验证据编号', required: true }, { key: 'description', label: '备注', type: 'multiline' },
];

const columns: readonly ColumnDefinition<Furnace>[] = [
  { key: 'location', label: '位置 / 炉型', minWidth: 150, render: (item) => <><Typography variant="body2">{item.plantArea}</Typography><Typography variant="caption" color="text.secondary">{item.furnaceType}</Typography></> },
  { key: 'capacity', label: '能力', minWidth: 120, render: (item) => <><Typography variant="body2">{item.capacityTonnes.toFixed(1)} t</Typography><Typography variant="caption" color="text.secondary">≤ {item.maxTemperatureC.toFixed(0)} °C</Typography></> },
  { key: 'alloys', label: '支持牌号', minWidth: 160, render: (item) => item.supportedAlloys },
  { key: 'operator', label: '责任班组', minWidth: 120, render: (item) => item.operator },
  { key: 'inspection', label: '最近检验', minWidth: 170, render: (item) => <><Typography variant="body2">{formatDate(item.lastInspectionAt)}</Typography><Typography variant="caption" color="text.secondary">{item.evidence || '未登记证据'}</Typography></> },
];

export default function FurnacePage() {
  return <EntityPage
    path="furnaces" label="炉台能力" description="维护炉台容量、温度上限、适用牌号与检验状态。"
    useStore={useFurnaceStore} fields={fields} columns={columns} transitions={FURNACE_TRANSITIONS}
    createRoles={['reviewer', 'admin']} updateRoles={['reviewer', 'admin']} transitionRoles={['operator', 'reviewer', 'admin']}
    editableStatuses={['available', 'charging', 'maintenance']} deletableStatuses={['maintenance', 'locked']}
  />;
}
