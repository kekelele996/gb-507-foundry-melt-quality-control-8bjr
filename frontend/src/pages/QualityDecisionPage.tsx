import { useCallback, useEffect, useState } from 'react';
import RefreshIcon from '@mui/icons-material/Refresh';
import { Alert, Box, Button, IconButton, Tooltip, Typography } from '@mui/material';
import { ReleasePairingBoard } from '../components/release/ReleasePairingBoard';
import { ReleaseJudgeDialog, type VerdictTarget } from '../components/release/ReleaseJudgeDialog';
import { ReleaseReviewHistory } from '../components/release/ReleaseReviewHistory';
import { useReleaseReviewStore } from '../stores/release-review';
import type { ReleasePairingView } from '../types/domain';

export default function QualityDecisionPage() {
  const { board, reviews, loading, error, loadBoard, loadReviews, openReview, judge, clearError } = useReleaseReviewStore();
  const [dialogRow, setDialogRow] = useState<ReleasePairingView | null>(null);
  const [dialogTarget, setDialogTarget] = useState<VerdictTarget>('accepted');
  const [actionError, setActionError] = useState('');

  const refresh = useCallback(() => {
    void loadBoard();
    void loadReviews(1, 50);
  }, [loadBoard, loadReviews]);

  useEffect(() => { refresh(); }, [refresh]);

  async function handleOpenReview(heatCode: string) {
    setActionError('');
    try {
      await openReview(heatCode, `炉次 ${heatCode} 放行合议`);
      refresh();
    } catch (reason) {
      setActionError(reason instanceof Error ? reason.message : String(reason));
    }
  }

  function handleJudge(row: ReleasePairingView, target: VerdictTarget) {
    setActionError('');
    setDialogRow(row);
    setDialogTarget(target);
  }

  async function confirmJudge(reason: string) {
    if (!dialogRow || !dialogRow.reviewId) return;
    setActionError('');
    try {
      await judge(dialogRow.reviewId, dialogTarget, dialogRow.reviewVersion, reason);
      setDialogRow(null);
      refresh();
    } catch (reason) {
      setActionError(reason instanceof Error ? reason.message : String(reason));
    }
  }

  return <main className="workspace release-workspace">
    <header className="page-header">
      <div>
        <p className="eyebrow">生产质量控制</p>
        <h1>质量判定 · 炉次放行合议</h1>
        <p>同一炉次须有两份已复核样本，碳硅硫磷都在牌号范围内、且两份样本碳硅读数差值各不超过 0.050，复核员才能判定合格；否则只能返炉或报废并填写原因。</p>
      </div>
      <Tooltip title="刷新配对与判定状态"><span>
        <Button variant="outlined" startIcon={<RefreshIcon />} onClick={refresh} disabled={loading}>刷新</Button>
      </span></Tooltip>
    </header>

    {(error || actionError) && <Alert severity="error" onClose={() => { clearError(); setActionError(''); }} sx={{ mb: 2 }}>
      {actionError || error}
    </Alert>}

    <ReleasePairingBoard rows={board} loading={loading} onOpenReview={handleOpenReview} onJudge={handleJudge} />

    <Box sx={{ height: 24 }} />

    <ReleaseReviewHistory reviews={reviews} />

    <ReleaseJudgeDialog
      row={dialogRow}
      target={dialogTarget}
      submitting={loading}
      onClose={() => setDialogRow(null)}
      onConfirm={confirmJudge}
    />
  </main>;
}
