import GavelIcon from '@mui/icons-material/Gavel';
import BlockIcon from '@mui/icons-material/Block';
import CheckCircleIcon from '@mui/icons-material/CheckCircle';
import LockIcon from '@mui/icons-material/Lock';
import {
  Alert, Box, Button, Chip, IconButton, Paper, Table, TableBody, TableCell, TableContainer,
  TableHead, TableRow, Tooltip, Typography,
} from '@mui/material';
import { useAuth } from '../../hooks/useAuth';
import type { ReleasePairingView } from '../../types/domain';
import { HeatStateBadge } from '../common/HeatStateBadge';
import { StatusBadge } from '../common/StatusBadge';
import { formatDate } from '../../utils/format';

interface PairingBoardProps {
  rows: ReleasePairingView[];
  loading: boolean;
  onOpenReview: (heatCode: string) => void;
  onJudge: (row: ReleasePairingView, target: 'accepted' | 'remelted' | 'scrapped') => void;
}

const STATE_META: Record<string, { label: string; color: 'success' | 'warning' | 'error' | 'default' }> = {
  ready: { label: '可放行', color: 'success' },
  blocked: { label: '被阻塞', color: 'error' },
  incomplete: { label: '配对不足', color: 'warning' },
  'locked-in': { label: '已锁定验收', color: 'success' },
  closed: { label: '已终判', color: 'default' },
};

function DeltaChip({ ok, value, hasPair, element }: { ok: boolean; value: number; hasPair: boolean; element: string }) {
  if (!hasPair) return <Chip size="small" variant="outlined" disabled label={`Δ${element} -`} />;
  return <Chip size="small" color={ok ? 'success' : 'error'} variant={ok ? 'outlined' : 'filled'}
    label={`Δ${element} ${Math.abs(value).toFixed(3)}`} />;
}

function ReadingCell({ value, inRange, rangeLabel }: { value: number | null; inRange?: boolean; rangeLabel?: string }) {
  if (value === null || value === undefined) {
    return <Typography variant="body2" color="text.disabled">缺样本</Typography>;
  }
  return <Tooltip title={rangeLabel || ''} placement="top">
    <Typography variant="body2" fontWeight={600} color={inRange === false ? 'error.main' : 'text.primary'}>
      {value.toFixed(3)}
    </Typography>
  </Tooltip>;
}

export function ReleasePairingBoard({ rows, loading, onOpenReview, onJudge }: PairingBoardProps) {
  const { hasRole } = useAuth();
  const canReview = hasRole('reviewer', 'admin');

  return <section className="pairing-board">
    <Box className="pairing-board-header">
      <div>
        <Typography variant="h6" component="h2">炉次放行合议配对看板</Typography>
        <Typography variant="body2" color="text.secondary">
          同一炉次两份已复核样本配对；碳、硅读数差值各不超过 0.050 且碳硅硫磷均在牌号范围内方可放行，否则只能返炉或报废并填写原因。
        </Typography>
      </div>
      <Chip size="small" label={`待判炉次 ${rows.filter((r) => r.heatStatus === 'hold').length}`} color="warning" />
    </Box>
    {loading && <Alert severity="info" sx={{ mb: 1 }}>正在加载配对数据…</Alert>}
    <TableContainer component={Paper} variant="outlined">
      <Table size="small" className="pairing-table">
        <TableHead><TableRow>
          <TableCell>炉次 / 牌号</TableCell>
          <TableCell>样本甲（先取）</TableCell>
          <TableCell>样本乙（后取）</TableCell>
          <TableCell>ΔC / ΔSi</TableCell>
          <TableCell sx={{ minWidth: 240 }}>阻塞原因</TableCell>
          <TableCell>合议 / 决定</TableCell>
          <TableCell align="right">判定操作</TableCell>
        </TableRow></TableHead>
        <TableBody>
          {rows.map((row) => {
            const meta = STATE_META[row.pairingState] || STATE_META.incomplete;
            const inHold = row.heatStatus === 'hold';
            const hasOpenReview = row.reviewStatus === 'open';
            const showActions = canReview && inHold;
            const cInSpec = (v: number | null | undefined, lo: number, hi: number) =>
              v === null || v === undefined ? undefined : v >= lo && v <= hi;
            const sInSpec = (v: number | null | undefined, max: number) =>
              v === null || v === undefined ? undefined : v <= max;
            return <TableRow key={row.heatCode} hover className={row.pairingState === 'ready' ? 'pairing-ready' : row.pairingState === 'blocked' ? 'pairing-blocked' : ''}>
              <TableCell>
                <Typography variant="body2" fontWeight={700}>{row.heatCode}</Typography>
                <Typography variant="caption" color="text.secondary">{row.heatName} · {row.alloyGrade}</Typography>
                <Box sx={{ mt: 0.5 }}><HeatStateBadge state={row.heatStatus as never} /></Box>
              </TableCell>
              <TableCell>
                <SampleReading row={row} which="first" cInSpec={cInSpec} sInSpec={sInSpec} />
              </TableCell>
              <TableCell>
                <SampleReading row={row} which="second" cInSpec={cInSpec} sInSpec={sInSpec} />
              </TableCell>
              <TableCell>
                <Box className="delta-stack">
                  <DeltaChip ok={row.carbonDeltaOk} value={row.carbonDeltaPct} hasPair={Boolean(row.firstSample && row.secondSample)} element="C" />
                  <DeltaChip ok={row.siliconDeltaOk} value={row.siliconDeltaPct} hasPair={Boolean(row.firstSample && row.secondSample)} element="Si" />
                </Box>
                <Typography variant="caption" color="text.secondary">容差 0.050</Typography>
              </TableCell>
              <TableCell>
                {row.blockers.length > 0
                  ? <Box className="blocker-list">
                    {row.blockers.slice(0, 3).map((reason) => <Alert key={reason} severity="error" icon={<BlockIcon fontSize="inherit" />} sx={{ py: 0, mb: 0.5 }}>
                      <Typography variant="caption">{reason}</Typography>
                    </Alert>)}
                  </Box>
                  : <Typography variant="caption" color={inHold ? 'success.main' : 'text.secondary'}>
                    {inHold ? '无阻塞项，满足放行条件' : '该炉次已终判'}
                  </Typography>}
              </TableCell>
              <TableCell>
                <Box className="review-state-cell">
                  <Chip size="small" color={meta.color} label={meta.label} />
                  {row.reviewCode
                    ? <Typography variant="caption">{row.reviewCode} · <StatusBadge status={row.reviewStatus} /></Typography>
                    : <Typography variant="caption" color="text.secondary">尚未发起合议</Typography>}
                  {row.decisionCode && <Typography variant="caption" color="text.secondary">决定 {row.decisionCode} · {row.decisionStatus}</Typography>}
                  {row.reviewer && <Typography variant="caption" color="text.secondary">复核员 {row.reviewer}</Typography>}
                </Box>
              </TableCell>
              <TableCell align="right">
                {showActions && !hasOpenReview && <Tooltip title="发起放行合议"><span>
                  <Button size="small" variant="outlined" startIcon={<GavelIcon />} onClick={() => onOpenReview(row.heatCode)}>发起合议</Button>
                </span></Tooltip>}
                {showActions && hasOpenReview && <Box className="verdict-buttons">
                  <Tooltip title={row.pairingState === 'ready' ? '两份样本一致且全部在牌号范围内' : '存在阻塞项，不能放行'}>
                    <span>
                      <IconButton size="small" color="success" disabled={row.pairingState !== 'ready'}
                        onClick={() => onJudge(row, 'accepted')}><CheckCircleIcon fontSize="small" /></IconButton>
                    </span>
                  </Tooltip>
                  <Tooltip title="返炉（必须填写原因）"><span>
                    <Button size="small" color="warning" variant="outlined"
                      onClick={() => onJudge(row, 'remelted')}>返炉</Button>
                  </span></Tooltip>
                  <Tooltip title="报废（必须填写原因）"><span>
                    <Button size="small" color="error" variant="outlined"
                      onClick={() => onJudge(row, 'scrapped')}>报废</Button>
                  </span></Tooltip>
                </Box>}
                {!inHold && <Tooltip title="炉次已终判"><span><LockIcon fontSize="small" color="disabled" /></span></Tooltip>}
              </TableCell>
            </TableRow>;
          })}
          {!rows.length && !loading && <TableRow><TableCell colSpan={7} align="center" sx={{ py: 6, color: 'text.secondary' }}>
            暂无待判炉次
          </TableCell></TableRow>}
        </TableBody>
      </Table>
    </TableContainer>
  </section>;
}

function SampleReading({ row, which, cInSpec, sInSpec }: {
  row: ReleasePairingView;
  which: 'first' | 'second';
  cInSpec: (v: number | null | undefined, lo: number, hi: number) => boolean | undefined;
  sInSpec: (v: number | null | undefined, max: number) => boolean | undefined;
}) {
  const sample = which === 'first' ? row.firstSample : row.secondSample;
  if (!sample) {
    return <Box>
      <Typography variant="body2" color="text.disabled">（缺第二份已复核样本）</Typography>
      <Typography variant="caption" color="text.secondary">已复核 {row.verifiedCount} 份</Typography>
    </Box>;
  }
  const cRange = `C 牌号范围 ${row.carbonRange[0].toFixed(2)}-${row.carbonRange[1].toFixed(2)}%`;
  const siRange = `Si 牌号范围 ${row.siliconRange[0].toFixed(2)}-${row.siliconRange[1].toFixed(2)}%`;
  const carbonIn = cInSpec(sample.carbonPct, row.carbonRange[0], row.carbonRange[1]);
  const siliconIn = cInSpec(sample.siliconPct, row.siliconRange[0], row.siliconRange[1]);
  const sulfurIn = sInSpec(sample.sulfurPct, row.sulfurMaxPct);
  const phosphorusIn = sInSpec(sample.phosphorusPct, row.phosphorusMaxPct);
  return <Box className="sample-reading">
    <Typography variant="body2" fontWeight={700}>{sample.code} <StatusBadge status={sample.status} /></Typography>
    <Typography variant="caption" color="text.secondary">{formatDate(sample.sampledAt)}</Typography>
    <Box className="reading-grid">
      <span>C <ReadingCell value={sample.carbonPct} inRange={carbonIn} rangeLabel={cRange} /></span>
      <span>Si <ReadingCell value={sample.siliconPct} inRange={siliconIn} rangeLabel={siRange} /></span>
      <span>S <ReadingCell value={sample.sulfurPct} inRange={sulfurIn} rangeLabel={`S 上限 ${row.sulfurMaxPct.toFixed(3)}%`} /></span>
      <span>P <ReadingCell value={sample.phosphorusPct} inRange={phosphorusIn} rangeLabel={`P 上限 ${row.phosphorusMaxPct.toFixed(3)}%`} /></span>
    </Box>
  </Box>;
}
