import { Paper, Table, TableBody, TableCell, TableContainer, TableHead, TableRow, Typography } from '@mui/material';
import type { ChemicalSample } from '../../types/domain';
import { StatusBadge } from './StatusBadge';

export function ChemistryTable({ records, title = '五元素化验明细' }: { records: ChemicalSample[]; title?: string }) {
  return <section className="chemistry-section">
    <Typography variant="h6" component="h2">{title}</Typography>
    <TableContainer component={Paper} variant="outlined">
      <Table size="small">
        <TableHead><TableRow><TableCell>样本</TableCell><TableCell>炉次</TableCell><TableCell>C %</TableCell><TableCell>Si %</TableCell><TableCell>Mn %</TableCell><TableCell>S %</TableCell><TableCell>P %</TableCell><TableCell>结果</TableCell></TableRow></TableHead>
        <TableBody>
          {records.slice(0, 8).map((item) => <TableRow key={item.id}>
            <TableCell><strong>{item.code}</strong><small>{item.methodVersion}</small></TableCell><TableCell>{item.heatCode}</TableCell>
            <TableCell>{item.carbonPct.toFixed(3)}</TableCell><TableCell>{item.siliconPct.toFixed(3)}</TableCell><TableCell>{item.manganesePct.toFixed(3)}</TableCell>
            <TableCell>{item.sulfurPct.toFixed(3)}</TableCell><TableCell>{item.phosphorusPct.toFixed(3)}</TableCell><TableCell><StatusBadge status={item.status} /></TableCell>
          </TableRow>)}
          {!records.length && <TableRow><TableCell colSpan={8} align="center" sx={{ py: 4, color: 'text.secondary' }}>暂无化验结果</TableCell></TableRow>}
        </TableBody>
      </Table>
    </TableContainer>
  </section>;
}
