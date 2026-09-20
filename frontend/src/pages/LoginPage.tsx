import { useState, type FormEvent } from 'react';
import LockOutlinedIcon from '@mui/icons-material/LockOutlined';
import { Alert, Button, CircularProgress, TextField } from '@mui/material';
import { Navigate, useLocation, useNavigate } from 'react-router-dom';
import { useAuth } from '../hooks/useAuth';

export default function LoginPage() {
  const { session, loading, signIn } = useAuth();
  const navigate = useNavigate();
  const location = useLocation();
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  if (session) return <Navigate to="/furnaces" replace />;

  async function submit(event: FormEvent) {
    event.preventDefault();
    setError('');
    try {
      await signIn(username, password);
      const target = (location.state as { from?: string } | null)?.from || '/furnaces';
      navigate(target, { replace: true });
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : '登录失败');
    }
  }

  return <main className="login-page">
    <section className="login-visual" aria-hidden="true"><div className="heat-readout"><span>HEAT 260822</span><strong>1,512 °C</strong><small>OES LINE · ACTIVE</small></div></section>
    <section className="login-panel">
      <form onSubmit={(event) => void submit(event)}>
        <div className="login-mark"><LockOutlinedIcon /><span>FOUNDRY QMS</span></div>
        <h1>熔炼质量控制台</h1>
        <p>生产质量身份认证</p>
        {error && <Alert severity="error">{error}</Alert>}
        <TextField label="账号" autoComplete="username" value={username} onChange={(event) => setUsername(event.target.value)} required fullWidth autoFocus />
        <TextField label="密码" type="password" autoComplete="current-password" value={password} onChange={(event) => setPassword(event.target.value)} required fullWidth />
        <Button type="submit" variant="contained" size="large" disabled={loading || !username.trim() || !password} fullWidth>
          {loading ? <CircularProgress size={22} color="inherit" /> : '登录'}
        </Button>
      </form>
    </section>
  </main>;
}
