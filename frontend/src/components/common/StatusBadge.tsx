
import { statusTone } from '../../utils/format';
import Chip from '@mui/material/Chip';
export function StatusBadge({ status }: { status: string }) {
  const tone = statusTone(status);
  const color = tone === 'success' ? 'success' : tone === 'warning' ? 'warning' : tone === 'danger' ? 'error' : 'default';
  return <Chip size="small" color={color} variant={tone === 'neutral' ? 'outlined' : 'filled'} label={status.replaceAll('_', ' ')} />;
}
