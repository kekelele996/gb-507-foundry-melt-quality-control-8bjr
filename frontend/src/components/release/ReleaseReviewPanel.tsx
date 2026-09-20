import { useCallback, useEffect, useMemo, useState } from 'react';
import RefreshIcon from '@mui/icons-material/Refresh';
import {
  Alert, Box, Button, Chip, CircularProgress, Divider, MenuItem, Paper, Stack, Table, TableBody,
  TableCell, TableHead, TableRow, TextField, Tooltip, Typography,
} from '@mui/material';
import { adjudicateRelease, getReleasePanel, listPanelHeats } from '../../api/release-review';
import { useAuth } from '../../hooks/useAuth';
import type { Heat, ReleaseAdjudicationRequest, ReleasePanel as Panel, ReleaseSampleView } from '../../types/domain';
import type { HeatState, ReleaseDecision } from '../../types/status';
import { HeatStateBadge } from '../common/HeatStateBadge';
import { formatDate } from '../../utils/format';

const PAIR_STATUS_LABEL: Record<Panel['pairStatus'], { text: string; color: 'success' | 'warning' | 'error' | 'default' }> = {
  'paired-ready': { text: '已配对（2 份复核样本）', color: 'success' },
  'pairing-short': { text: '配对不完整（仅 1 份复核样本）', color: 'warning' },
  'pairing-absent': { text: '无配对（缺少复核样本）', color: 'error' },
  'already-adjudged': { text: '已完成判定', color: 'default' },
};

const DECISION_LABEL: Record<ReleaseDecision, string> = {
  accept: '合格放行',
  remelt: '返炉',
  scrap: '报废',
};

function ElementCell({ sample, element }: { sample: ReleaseSampleView; element: string }) {
  const result = sample.elementResults.find((item) => item.element === element);
  const pass = result?.pass ?? false;
  return <TableCell align="center" sx={{ color: pass ? 'text.primary' : 'error.main', fontWeight: pass ? 500 : 700 }}>
    {result ? result.value.toFixed(3) : '-'}
    {!pass && ' ✕'}
  </TableCell>;
}

export function ReleaseReviewPanel() {
  const { hasRole } = useAuth();
  const canAdjudicate = hasRole('reviewer', 'admin');
  const [heats, setHeats] = useState<Heat[]>([]);
  const [selectedCode, setSelectedCode] = useState('');
  const [panel, setPanel] = useState<Panel | null>(null);
  const [loadingHeats, setLoadingHeats] = useState(false);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const [formError, setFormError] = useState('');
  const [decision, setDecision] = useState<ReleaseDecision>('remelt');
  const [reason, setReason] = useState('');
  const [evidence, setEvidence] = useState('');

  const loadHeats = useCallback(async (preferCode?: string) => {
    setLoadingHeats(true);
    try {
      const items = await listPanelHeats();
      setHeats(items);
      const hold = items.find((heat) => heat.status === 'hold');
      setSelectedCode((current) => {
        if (preferCode && items.some((heat) => heat.code === preferCode)) return preferCode;
        if (current && items.some((heat) => heat.code === current)) return current;
        return hold?.code || items[0]?.code || '';
      });
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : String(cause));
    } finally {
      setLoadingHeats(false);
    }
  }, []);

  const loadPanel = useCallback(async (code: string) => {
    if (!code) {
      setPanel(null);
      return;
    }
    setLoading(true);
    setError('');
    try {
      setPanel(await getReleasePanel(code));
    } catch (cause) {
      setPanel(null);
      setError(cause instanceof Error ? cause.message : String(cause));
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => { void loadHeats(); }, [loadHeats]);
  useEffect(() => { void loadPanel(selectedCode); }, [loadPanel, selectedCode]);

  const refresh = useCallback(async () => {
    await loadHeats(selectedCode);
    await loadPanel(selectedCode);
  }, [loadHeats, loadPanel, selectedCode]);

  const decided = panel?.decision != null;
  const spec = useMemo(() => {
    if (!panel) return '';
    const heat = panel.heat;
    return `C ${heat.carbonMinPct.toFixed(3)}–${heat.carbonMaxPct.toFixed(3)} · Si ${heat.siliconMinPct.toFixed(3)}–${heat.siliconMaxPct.toFixed(3)} · S ≤ ${heat.sulfurMaxPct.toFixed(3)} · P ≤ ${heat.phosphorusMaxPct.toFixed(3)}`;
  }, [panel]);

  async function submit() {
    if (!panel) return;
    const payload: ReleaseAdjudicationRequest = {
      heatCode: panel.heat.code, decision, reason: reason.trim(), evidence: evidence.trim(),
    };
    setSubmitting(true);
    setFormError('');
    try {
      await adjudicateRelease(payload);
      setReason('');
      setEvidence('');
      await refresh();
    } catch (cause) {
      setFormError(cause instanceof Error ? cause.message : String(cause));
    } finally {
      setSubmitting(false);
    }
  }

  const formValid = reason.trim().length >= 3 && evidence.trim().length >= 3;

  return <Paper variant="outlined" sx={{ p: 3, mb: 3 }}>
    <Stack direction={{ xs: 'column', md: 'row' }} spacing={2} alignItems={{ md: 'center' }} justifyContent="space-between">
      <Box>
        <Typography variant="h6" component="h2">炉次放行合议</Typography>
        <Typography variant="body2" color="text.secondary">
          同一炉次两份已复核样本、碳硅硫磷均在牌号范围内、且两样本 C/Si 读数差均 ≤ 0.05% 时方可判定合格；否则只能返炉或报废并填写原因。
        </Typography>
      </Box>
      <Stack direction="row" spacing={1.5} alignItems="center">
        <TextField select size="small" sx={{ minWidth: 320 }} label="选择炉次" value={selectedCode}
          onChange={(event) => setSelectedCode(event.target.value)} disabled={loadingHeats || !heats.length}>
          {heats.map((heat) => <MenuItem key={heat.id} value={heat.code}>
            <Stack direction="row" spacing={1} alignItems="center">
              <HeatStateBadge state={heat.status as HeatState} />
              <span>{heat.code} · {heat.alloyGrade} · {heat.name}</span>
            </Stack>
          </MenuItem>)}
        </TextField>
        <Tooltip title="刷新配对状态"><Button variant="outlined" sx={{ minWidth: 44, px: 1 }} onClick={() => void refresh()}><RefreshIcon /></Button></Tooltip>
      </Stack>
    </Stack>

    {(error || formError) && <Alert severity="error" sx={{ mt: 2 }} onClose={() => { setError(''); setFormError(''); }}>{formError || error}</Alert>}

    {loading && <Box sx={{ display: 'grid', placeItems: 'center', py: 8 }}><CircularProgress /></Box>}

    {!loading && panel && <>
      <Divider sx={{ my: 2 }} />
      <Stack direction={{ xs: 'column', lg: 'row' }} spacing={3}>
        <Box sx={{ flex: 1, minWidth: 0 }}>
          <Stack direction="row" spacing={1.5} alignItems="center" flexWrap="wrap" useFlexGap>
            <Typography variant="subtitle1" fontWeight={700}>{panel.heat.code} · {panel.heat.name}</Typography>
            <HeatStateBadge state={panel.heat.status as HeatState} />
            <Chip size="small" color={PAIR_STATUS_LABEL[panel.pairStatus].color}
              variant={panel.pairStatus === 'already-adjudged' ? 'outlined' : 'filled'}
              label={PAIR_STATUS_LABEL[panel.pairStatus].text} />
          </Stack>
          <Typography variant="body2" color="text.secondary" sx={{ mt: 1 }}>
            牌号 {panel.heat.alloyGrade} · 炉台 {panel.heat.furnaceCode} · 冻结规格 {spec}
          </Typography>

          <Box sx={{ mt: 2, display: 'flex', gap: 3, flexWrap: 'wrap' }}>
            <Box><Typography variant="caption" color="text.secondary">两样本碳读数差 ΔC</Typography>
              <Typography variant="h6" sx={{ color: panel.carbonDelta <= 0.05 + 1e-9 ? 'success.main' : 'error.main' }}>
                {panel.samples.length >= 2 ? `${panel.carbonDelta.toFixed(3)} %` : '—'}
              </Typography></Box>
            <Box><Typography variant="caption" color="text.secondary">两样本硅读数差 ΔSi</Typography>
              <Typography variant="h6" sx={{ color: panel.siliconDelta <= 0.05 + 1e-9 ? 'success.main' : 'error.main' }}>
                {panel.samples.length >= 2 ? `${panel.siliconDelta.toFixed(3)} %` : '—'}
              </Typography></Box>
            <Box><Typography variant="caption" color="text.secondary">复现允差</Typography><Typography variant="h6">≤ 0.050 %</Typography></Box>
          </Box>

          <Table size="small" sx={{ mt: 2 }}>
            <TableHead><TableRow>
              <TableCell>配对样本</TableCell><TableCell>取样点 / 复核员</TableCell>
              <TableCell align="center">C %</TableCell><TableCell align="center">Si %</TableCell>
              <TableCell align="center">S %</TableCell><TableCell align="center">P %</TableCell>
              <TableCell align="center">状态</TableCell><TableCell>复核时间</TableCell>
            </TableRow></TableHead>
            <TableBody>
              {panel.samples.map((sample) => <TableRow key={sample.id} hover>
                <TableCell><strong>{sample.code}</strong><br /><Typography variant="caption" color="text.secondary">{sample.methodVersion}</Typography></TableCell>
                <TableCell>{sample.samplePoint}<br /><Typography variant="caption" color="text.secondary">{sample.analyst}</Typography></TableCell>
                <ElementCell sample={sample} element="C" />
                <ElementCell sample={sample} element="Si" />
                <ElementCell sample={sample} element="S" />
                <ElementCell sample={sample} element="P" />
                <TableCell align="center"><Chip size="small" label={sample.status} color={sample.status === 'locked' ? 'success' : sample.status === 'verified' ? 'info' : 'default'} /></TableCell>
                <TableCell>{formatDate(sample.sampledAt)}</TableCell>
              </TableRow>)}
              {!panel.samples.length && <TableRow><TableCell colSpan={8} align="center" sx={{ py: 4, color: 'text.secondary' }}>该炉次暂无已复核样本</TableCell></TableRow>}
            </TableBody>
          </Table>

          <Box sx={{ mt: 2 }}>
            <Typography variant="subtitle2" fontWeight={700} sx={{ mb: 0.5 }}>阻塞原因</Typography>
            {panel.blockers.length === 0
              ? <Alert severity="success" icon={false}>无阻塞：两份复核样本读数均满足放行条件</Alert>
              : <Stack spacing={0.5}>{panel.blockers.map((blocker) => <Alert key={blocker} severity={decided ? 'info' : 'warning'} icon={false}>{blocker}</Alert>)}</Stack>}
          </Box>
        </Box>

        <Box sx={{ width: { xs: '100%', lg: 320 }, flexShrink: 0 }}>
          <Typography variant="subtitle2" fontWeight={700}>复核员判定</Typography>
          {decided ? <Box sx={{ mt: 1.5 }}>
            <Alert severity={panel.decision?.status === 'accept' ? 'success' : 'warning'}>
              本炉次已由 {panel.decision?.reviewer} 判定为「{DECISION_LABEL[panel.decision!.status as ReleaseDecision] || panel.decision?.status}」，决定一次落盘且不可撤销。
            </Alert>
            <Typography variant="body2" sx={{ mt: 1.5 }}><strong>原因：</strong>{panel.decision?.reason}</Typography>
            {panel.decision?.conditions && <Typography variant="body2" sx={{ mt: 0.5 }}><strong>配对条件：</strong>{panel.decision.conditions}</Typography>}
            <Typography variant="caption" color="text.secondary" sx={{ display: 'block', mt: 1 }}>
              {panel.decision?.code} · {formatDate(panel.decision?.decidedAt || '')} · 证据 {panel.decision?.evidence}
            </Typography>
          </Box> : canAdjudicate ? <Stack spacing={1.5} sx={{ mt: 1.5 }}>
            <TextField select size="small" label="判定结论" value={decision} onChange={(event) => setDecision(event.target.value as ReleaseDecision)}>
              <MenuItem value="accept" disabled={!panel.canAccept}>合格放行（仅在全部条件满足时可用）</MenuItem>
              <MenuItem value="remelt">返炉（须填写原因）</MenuItem>
              <MenuItem value="scrap">报废（须填写原因）</MenuItem>
            </TextField>
            <TextField label="判定原因（必填）" value={reason} onChange={(event) => setReason(event.target.value)}
              multiline minRows={3} placeholder={decision === 'accept' ? '确认双样本复核合格的放行依据' : '说明返炉/报废原因'} fullWidth />
            <TextField label="签发证据编号（必填）" value={evidence} onChange={(event) => setEvidence(event.target.value)}
              size="small" placeholder="QMS / LIMS 证据编号" fullWidth />
            <Button variant="contained" color={decision === 'accept' ? 'success' : decision === 'scrap' ? 'error' : 'warning'}
              disabled={submitting || !formValid || (decision === 'accept' && !panel.canAccept)} onClick={() => void submit()}>
              {submitting ? '提交中…' : `提交${DECISION_LABEL[decision]}判定`}
            </Button>
            <Typography variant="caption" color="text.secondary">
              提交后炉次验收、双样本锁定、决定与审计在同一事务内一次落盘；任一步失败全部回滚，重复或并发提交只成功一次。
            </Typography>
          </Stack> : <Alert severity="info" sx={{ mt: 1.5 }}>仅复核员（reviewer/admin）可执行判定。</Alert>}
        </Box>
      </Stack>
    </>}
  </Paper>;
}
