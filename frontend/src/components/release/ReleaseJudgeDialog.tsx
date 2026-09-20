import { useEffect, useState } from 'react';
import {
  Button, Dialog, DialogActions, DialogContent, DialogTitle, TextField, Typography,
} from '@mui/material';
import type { ReleasePairingView } from '../../types/domain';

export type VerdictTarget = 'accepted' | 'remelted' | 'scrapped';

interface JudgeDialogProps {
  row: ReleasePairingView | null;
  target: VerdictTarget;
  submitting: boolean;
  onClose: () => void;
  onConfirm: (reason: string) => Promise<void>;
}

const TITLE: Record<VerdictTarget, string> = {
  accepted: '判定合格 · 放行炉次',
  remelted: '判定返炉 · 填写原因',
  scrapped: '判定报废 · 填写原因',
};

const REQUIRE_REASON: Record<VerdictTarget, boolean> = {
  accepted: false,
  remelted: true,
  scrapped: true,
};

export function ReleaseJudgeDialog({ row, target, submitting, onClose, onConfirm }: JudgeDialogProps) {
  const [reason, setReason] = useState('');
  const [busy, setBusy] = useState(false);
  useEffect(() => {
    setReason('');
    setBusy(false);
  }, [row, target]);

  if (!row) return null;
  const needReason = REQUIRE_REASON[target];
  const reasonValid = !needReason || reason.trim().length >= 3;

  async function confirm() {
    if (!reasonValid) return;
    setBusy(true);
    try {
      await onConfirm(reason.trim());
    } finally {
      setBusy(false);
    }
  }

  return <Dialog open={Boolean(row)} onClose={submitting ? undefined : onClose} fullWidth maxWidth="sm">
    <DialogTitle>{TITLE[target]} · {row.heatCode}</DialogTitle>
    <DialogContent dividers className="dialog-stack">
      <Typography variant="body2">
        合议 {row.reviewCode || '（新建后生成）'}，配对样本 {row.firstSample?.code || '-'} / {row.secondSample?.code || '-'}，
        ΔC={Math.abs(row.carbonDeltaPct).toFixed(3)}，ΔSi={Math.abs(row.siliconDeltaPct).toFixed(3)}。
      </Typography>
      {target === 'accepted' && <Typography variant="body2" color="success.main">
        提交后将一次落盘：炉次验收、两份样本锁定、质量决定与全部审计；任一步失败都会整体回滚。
      </Typography>}
      {(target === 'remelted' || target === 'scrapped') && <Typography variant="body2" color="warning.main">
        炉次将进入 rejected 终态，质量决定记为{target === 'remelted' ? '返炉（remelt）' : '报废（scrap）'}，样本不锁定。原因至少 3 个字。
      </Typography>}
      <TextField
        label={needReason ? '返炉 / 报废原因（必填）' : '放行备注（可选）'}
        value={reason} onChange={(event) => setReason(event.target.value)}
        multiline minRows={3} required={needReason} fullWidth autoFocus
        helperText={needReason && !reasonValid ? '请填写至少 3 个字的原因' : '该原因会写入质量决定与审计日志'}
      />
    </DialogContent>
    <DialogActions>
      <Button onClick={onClose} disabled={submitting || busy}>取消</Button>
      <Button variant="contained" color={target === 'accepted' ? 'success' : target === 'remelted' ? 'warning' : 'error'}
        onClick={() => void confirm()} disabled={submitting || busy || !reasonValid}>
        {busy || submitting ? '提交中…' : '确认判定'}
      </Button>
    </DialogActions>
  </Dialog>;
}
