import { useEffect, useMemo, useState, type ReactNode } from 'react';
import AddIcon from '@mui/icons-material/Add';
import DeleteOutlineIcon from '@mui/icons-material/DeleteOutline';
import EditOutlinedIcon from '@mui/icons-material/EditOutlined';
import RefreshIcon from '@mui/icons-material/Refresh';
import SearchIcon from '@mui/icons-material/Search';
import SyncAltIcon from '@mui/icons-material/SyncAlt';
import {
  Alert, Box, Button, CircularProgress, Dialog, DialogActions, DialogContent, DialogTitle,
  IconButton, InputAdornment, MenuItem, Paper, Table, TableBody, TableCell, TableContainer,
  TableHead, TablePagination, TableRow, TextField, Tooltip, Typography,
} from '@mui/material';
import type { BaseRecord, UserRole } from '../types/domain';
import type { EntityStore } from '../stores/factory';
import { useAuth } from '../hooks/useAuth';
import { usePagination } from '../hooks/usePagination';
import { MetricCard } from './common/MetricCard';
import { StatusBadge } from './common/StatusBadge';

export interface FieldDefinition {
  key: string;
  label: string;
  type?: 'text' | 'number' | 'datetime-local' | 'multiline';
  required?: boolean;
  defaultValue?: string | number;
  min?: number;
  max?: number;
  step?: number;
}

export interface ColumnDefinition<T extends BaseRecord> {
  key: string;
  label: string;
  render: (item: T) => ReactNode;
  minWidth?: number;
}

interface EntityPageProps<T extends BaseRecord> {
  path: string;
  label: string;
  description: string;
  useStore: EntityStore<T>;
  fields: readonly FieldDefinition[];
  columns: readonly ColumnDefinition<T>[];
  transitions: Record<string, readonly string[]>;
  createRoles: readonly UserRole[];
  updateRoles: readonly UserRole[];
  transitionRoles: readonly UserRole[];
  editableStatuses?: readonly string[];
  deletableStatuses?: readonly string[];
  statusRender?: (item: T) => ReactNode;
  footer?: ReactNode;
}

type Draft = Record<string, string | number>;

function inputDate(value?: string): string {
  const date = value ? new Date(value) : new Date();
  const local = new Date(date.getTime() - date.getTimezoneOffset() * 60_000);
  return local.toISOString().slice(0, 16);
}

function makeDraft(fields: readonly FieldDefinition[], item?: BaseRecord): Draft {
  return Object.fromEntries(fields.map((field) => {
    const value = item ? (item as unknown as Record<string, unknown>)[field.key] : field.defaultValue;
    if (field.type === 'datetime-local') return [field.key, inputDate(typeof value === 'string' ? value : undefined)];
    return [field.key, typeof value === 'number' || typeof value === 'string' ? value : ''];
  }));
}

function preparePayload(fields: readonly FieldDefinition[], draft: Draft): Record<string, unknown> {
  return Object.fromEntries(fields.map((field) => {
    const value = draft[field.key];
    if (field.type === 'number') return [field.key, Number(value)];
    if (field.type === 'datetime-local') return [field.key, new Date(String(value)).toISOString()];
    return [field.key, String(value).trim()];
  }));
}

export function EntityPage<T extends BaseRecord>({
  path, label, description, useStore, fields, columns, transitions, createRoles, updateRoles, transitionRoles,
  editableStatuses, deletableStatuses, statusRender, footer,
}: EntityPageProps<T>) {
  const { items, meta, loading, error, load, createRecord, updateRecord, transition, deleteRecord, clearError } = useStore();
  const { session, hasRole } = useAuth();
  const pagination = usePagination(meta.total, 20);
  const [searchInput, setSearchInput] = useState('');
  const [query, setQuery] = useState('');
  const [editing, setEditing] = useState<T | null | undefined>(undefined);
  const [draft, setDraft] = useState<Draft>(() => makeDraft(fields));
  const [transitioning, setTransitioning] = useState<T | null>(null);
  const [target, setTarget] = useState('');
  const [reason, setReason] = useState('');
  const [deleting, setDeleting] = useState<T | null>(null);

  useEffect(() => {
    void load(path, query, pagination.page, pagination.pageSize);
  }, [load, pagination.page, pagination.pageSize, path, query]);

  const statusKinds = useMemo(() => new Set(items.map((item) => item.status)).size, [items]);
  const pending = useMemo(() => items.filter((item) => (transitions[item.status] || []).length > 0).length, [items, transitions]);
  const canCreate = hasRole(...createRoles);
  const canUpdate = hasRole(...updateRoles);
  const canTransition = hasRole(...transitionRoles);
  const isAdmin = session?.role === 'admin';

  function openCreate() {
    clearError();
    setDraft(makeDraft(fields));
    setEditing(null);
  }

  function openEdit(item: T) {
    clearError();
    setDraft(makeDraft(fields, item));
    setEditing(item);
  }

  function openTransition(item: T) {
    const targets = transitions[item.status] || [];
    setTransitioning(item);
    setTarget(targets[0] || '');
    setReason('');
  }

  async function save() {
    const payload = preparePayload(fields, draft);
    if (editing) await updateRecord(path, editing, payload);
    else await createRecord(path, payload);
    setEditing(undefined);
  }

  async function applyTransition() {
    if (!transitioning || !target || reason.trim().length < 3) return;
    await transition(path, transitioning, target, reason.trim());
    setTransitioning(null);
  }

  async function remove() {
    if (!deleting) return;
    await deleteRecord(path, deleting);
    setDeleting(null);
  }

  return <main className="workspace">
    <header className="page-header">
      <div><p className="eyebrow">生产质量控制</p><h1>{label}</h1><p>{description}</p></div>
      {canCreate && <Button variant="contained" startIcon={<AddIcon />} onClick={openCreate}>新增{label}</Button>}
    </header>

    <section className="metrics" aria-label={`${label}概览`}>
      <MetricCard label="记录总数" value={meta.total} detail="当前查询范围" />
      <MetricCard label="待推进" value={pending} detail="仍有合法下一状态" />
      <MetricCard label="状态分布" value={statusKinds} detail="当前页已覆盖状态" />
    </section>

    {error && <Alert severity="error" onClose={clearError} sx={{ mb: 2 }}>{error}</Alert>}

    <Paper variant="outlined" className="work-surface">
      <Box className="toolbar">
        <TextField
          size="small" value={searchInput} onChange={(event) => setSearchInput(event.target.value)}
          onKeyDown={(event) => { if (event.key === 'Enter') { pagination.setPage(1); setQuery(searchInput.trim()); } }}
          placeholder={`搜索${label}编码或名称`} aria-label={`搜索${label}`}
          InputProps={{ startAdornment: <InputAdornment position="start"><SearchIcon fontSize="small" /></InputAdornment> }}
        />
        <Button variant="outlined" onClick={() => { pagination.setPage(1); setQuery(searchInput.trim()); }}>查询</Button>
        <Tooltip title="重置查询"><IconButton onClick={() => { setSearchInput(''); setQuery(''); pagination.setPage(1); }}><RefreshIcon /></IconButton></Tooltip>
      </Box>
      <TableContainer className="domain-table">
        <Table stickyHeader size="small">
          <TableHead><TableRow>
            <TableCell sx={{ minWidth: 130 }}>编码 / 名称</TableCell>
            <TableCell sx={{ minWidth: 110 }}>状态</TableCell>
            {columns.map((column) => <TableCell key={column.key} sx={{ minWidth: column.minWidth }}>{column.label}</TableCell>)}
            <TableCell align="right" sx={{ minWidth: 140 }}>操作</TableCell>
          </TableRow></TableHead>
          <TableBody>
            {items.map((item) => {
              const targets = transitions[item.status] || [];
              return <TableRow hover key={item.id}>
                <TableCell><Typography variant="body2" fontWeight={700}>{item.code}</Typography><Typography variant="caption" color="text.secondary">{item.name}</Typography></TableCell>
                <TableCell>{statusRender ? statusRender(item) : <StatusBadge status={item.status} />}</TableCell>
                {columns.map((column) => <TableCell key={column.key}>{column.render(item)}</TableCell>)}
                <TableCell align="right">
                  {canUpdate && (!editableStatuses || editableStatuses.includes(item.status)) && <Tooltip title="编辑"><span><IconButton size="small" onClick={() => openEdit(item)} disabled={loading}><EditOutlinedIcon fontSize="small" /></IconButton></span></Tooltip>}
                  {canTransition && targets.length > 0 && <Tooltip title="状态迁移"><span><IconButton size="small" color="primary" onClick={() => openTransition(item)} disabled={loading}><SyncAltIcon fontSize="small" /></IconButton></span></Tooltip>}
                  {isAdmin && (!deletableStatuses || deletableStatuses.includes(item.status)) && <Tooltip title="删除"><span><IconButton size="small" color="error" onClick={() => setDeleting(item)} disabled={loading}><DeleteOutlineIcon fontSize="small" /></IconButton></span></Tooltip>}
                </TableCell>
              </TableRow>;
            })}
            {!items.length && !loading && <TableRow><TableCell colSpan={columns.length + 3} align="center" sx={{ py: 8, color: 'text.secondary' }}>暂无记录</TableCell></TableRow>}
          </TableBody>
        </Table>
        {loading && <Box className="table-loading"><CircularProgress size={28} /></Box>}
      </TableContainer>
      <TablePagination
        component="div" count={meta.total} page={pagination.page - 1} rowsPerPage={pagination.pageSize}
        onPageChange={(_, page) => pagination.setPage(page + 1)}
        onRowsPerPageChange={(event) => { pagination.setPageSize(Number(event.target.value)); pagination.setPage(1); }}
        rowsPerPageOptions={[10, 20, 50]} labelRowsPerPage="每页" labelDisplayedRows={({ from, to, count }) => `${from}-${to} / ${count}`}
      />
    </Paper>

    {footer}

    <Dialog open={editing !== undefined} onClose={() => setEditing(undefined)} maxWidth="md" fullWidth>
      <DialogTitle>{editing ? `编辑${label}` : `新增${label}`}</DialogTitle>
      <DialogContent dividers><Box className="form-grid">
        {fields.map((field) => <TextField
          key={field.key} label={field.label} required={field.required} fullWidth
          type={field.type === 'multiline' ? 'text' : field.type || 'text'}
          multiline={field.type === 'multiline'} minRows={field.type === 'multiline' ? 2 : undefined}
          value={draft[field.key] ?? ''} disabled={Boolean(editing && field.key === 'code')}
          inputProps={{ min: field.min, max: field.max, step: field.step }}
          onChange={(event) => setDraft((current) => ({ ...current, [field.key]: event.target.value }))}
        />)}
      </Box></DialogContent>
      <DialogActions><Button onClick={() => setEditing(undefined)}>取消</Button><Button variant="contained" onClick={() => void save()} disabled={loading}>保存</Button></DialogActions>
    </Dialog>

    <Dialog open={Boolean(transitioning)} onClose={() => setTransitioning(null)} fullWidth maxWidth="xs">
      <DialogTitle>状态迁移 · {transitioning?.code}</DialogTitle>
      <DialogContent dividers className="dialog-stack">
        <TextField select label="目标状态" value={target} onChange={(event) => setTarget(event.target.value)} fullWidth>
          {(transitioning ? transitions[transitioning.status] || [] : []).map((value) => <MenuItem key={value} value={value}>{value}</MenuItem>)}
        </TextField>
        <TextField label="迁移原因" value={reason} onChange={(event) => setReason(event.target.value)} multiline minRows={3} required fullWidth />
      </DialogContent>
      <DialogActions><Button onClick={() => setTransitioning(null)}>取消</Button><Button variant="contained" onClick={() => void applyTransition()} disabled={loading || reason.trim().length < 3}>确认迁移</Button></DialogActions>
    </Dialog>

    <Dialog open={Boolean(deleting)} onClose={() => setDeleting(null)} maxWidth="xs" fullWidth>
      <DialogTitle>删除 {deleting?.code}</DialogTitle>
      <DialogContent dividers><Typography>该记录将进入软删除状态，审计日志会保留。</Typography></DialogContent>
      <DialogActions><Button onClick={() => setDeleting(null)}>取消</Button><Button color="error" variant="contained" onClick={() => void remove()} disabled={loading}>删除</Button></DialogActions>
    </Dialog>
  </main>;
}
