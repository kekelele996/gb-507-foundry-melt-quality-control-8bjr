import { useEffect, useState } from 'react';
import { Alert, Chip, Paper, Table, TableBody, TableCell, TableContainer, TableHead, TableRow, Typography } from '@mui/material';
import { listAudits } from '../api/audit';
import type { AuditLog } from '../types/domain';
import { formatDate } from '../utils/format';

export default function AuditPage() {
  const [logs, setLogs] = useState<AuditLog[]>([]);
  const [error, setError] = useState('');
  useEffect(() => {
    listAudits().then((result) => setLogs(result.data)).catch((reason) => setError(reason instanceof Error ? reason.message : String(reason)));
  }, []);
  return <main className="workspace">
    <header className="page-header"><div><p className="eyebrow">治理与追踪</p><h1>操作审计</h1><p>核对操作者、请求标识、实体状态变化与业务明细。</p></div></header>
    {error && <Alert severity="error" sx={{ mb: 2 }}>{error}</Alert>}
    <TableContainer component={Paper} variant="outlined">
      <Table size="small" stickyHeader>
        <TableHead><TableRow><TableCell>发生时间</TableCell><TableCell>操作者</TableCell><TableCell>动作</TableCell><TableCell>实体</TableCell><TableCell>状态变化</TableCell><TableCell>业务明细</TableCell><TableCell>Request ID</TableCell></TableRow></TableHead>
        <TableBody>{logs.map((log) => <TableRow hover key={log.id}>
          <TableCell sx={{ minWidth: 160 }}>{formatDate(log.createdAt)}</TableCell><TableCell>{log.actor}</TableCell><TableCell><Chip size="small" variant="outlined" label={log.action} /></TableCell>
          <TableCell>{log.entityType} #{log.entityId}</TableCell><TableCell><Typography variant="caption">{log.beforeState || '-'} → {log.afterState || '-'}</Typography></TableCell>
          <TableCell sx={{ minWidth: 260 }}><Typography variant="body2" className="cell-wrap">{log.detail || '-'}</Typography></TableCell><TableCell><code>{log.requestId}</code></TableCell>
        </TableRow>)}</TableBody>
      </Table>
    </TableContainer>
  </main>;
}
