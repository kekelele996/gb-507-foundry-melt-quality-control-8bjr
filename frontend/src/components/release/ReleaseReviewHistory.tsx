import {
  Paper, Table, TableBody, TableCell, TableContainer, TableHead, TableRow, Typography,
} from '@mui/material';
import type { HeatReleaseReview } from '../../types/domain';
import { StatusBadge } from '../common/StatusBadge';
import { formatDate } from '../../utils/format';

export function ReleaseReviewHistory({ reviews }: { reviews: HeatReleaseReview[] }) {
  return <section className="review-history">
    <Typography variant="h6" component="h2">合议判定记录</Typography>
    <TableContainer component={Paper} variant="outlined">
      <Table size="small">
        <TableHead><TableRow>
          <TableCell>合议编码</TableCell>
          <TableCell>炉次</TableCell>
          <TableCell>配对样本</TableCell>
          <TableCell>ΔC / ΔSi</TableCell>
          <TableCell>判定</TableCell>
          <TableCell>复核员 / 时间</TableCell>
          <TableCell>原因</TableCell>
        </TableRow></TableHead>
        <TableBody>
          {reviews.map((review) => <TableRow key={review.id} hover>
            <TableCell><Typography variant="body2" fontWeight={700}>{review.code}</Typography>
              <Typography variant="caption" color="text.secondary">{review.name}</Typography></TableCell>
            <TableCell>{review.heatCode}</TableCell>
            <TableCell>
              <Typography variant="body2">{review.firstSampleCode || '-'}</Typography>
              <Typography variant="caption" color="text.secondary">{review.secondSampleCode || '缺第二份'}</Typography>
            </TableCell>
            <TableCell>
              <Typography variant="body2">ΔC {Math.abs(review.carbonDeltaPct).toFixed(3)}</Typography>
              <Typography variant="caption" color="text.secondary">ΔSi {Math.abs(review.siliconDeltaPct).toFixed(3)}</Typography>
            </TableCell>
            <TableCell><StatusBadge status={review.status} /></TableCell>
            <TableCell>
              <Typography variant="body2">{review.reviewer}</Typography>
              <Typography variant="caption" color="text.secondary">{review.decidedAt ? formatDate(review.decidedAt) : '待判定'}</Typography>
            </TableCell>
            <TableCell sx={{ maxWidth: 280 }}>
              <Typography variant="body2" className="cell-wrap">{review.verdictReason || review.pairingBlockers || '—'}</Typography>
            </TableCell>
          </TableRow>)}
          {!reviews.length && <TableRow><TableCell colSpan={7} align="center" sx={{ py: 4, color: 'text.secondary' }}>暂无合议记录</TableCell></TableRow>}
        </TableBody>
      </Table>
    </TableContainer>
  </section>;
}
