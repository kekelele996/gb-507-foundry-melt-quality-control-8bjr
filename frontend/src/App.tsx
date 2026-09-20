import AssessmentOutlinedIcon from '@mui/icons-material/AssessmentOutlined';
import BiotechOutlinedIcon from '@mui/icons-material/BiotechOutlined';
import FactoryOutlinedIcon from '@mui/icons-material/FactoryOutlined';
import GavelOutlinedIcon from '@mui/icons-material/GavelOutlined';
import LogoutIcon from '@mui/icons-material/Logout';
import WhatshotOutlinedIcon from '@mui/icons-material/WhatshotOutlined';
import { IconButton, Tooltip } from '@mui/material';
import { NavLink, Outlet, useNavigate } from 'react-router-dom';
import { useAuth } from './hooks/useAuth';

const navigation = [
  { to: '/furnaces', label: '炉台能力', icon: <FactoryOutlinedIcon />, roles: ['viewer', 'operator', 'reviewer', 'admin'] },
  { to: '/heats', label: '炉次工作台', icon: <WhatshotOutlinedIcon />, roles: ['viewer', 'operator', 'reviewer', 'admin'] },
  { to: '/samples', label: '化验结果', icon: <BiotechOutlinedIcon />, roles: ['viewer', 'operator', 'reviewer', 'admin'] },
  { to: '/decisions', label: '质量判定', icon: <GavelOutlinedIcon />, roles: ['viewer', 'operator', 'reviewer', 'admin'] },
  { to: '/audit', label: '审计记录', icon: <AssessmentOutlinedIcon />, roles: ['reviewer', 'admin'] },
];

export default function App() {
  const { session, logout } = useAuth();
  const navigate = useNavigate();
  const signOut = () => { logout(); navigate('/login', { replace: true }); };
  return <div className="app-shell">
    <aside>
      <div className="brand"><span>FOUNDRY QMS</span><strong>熔炼质量控制台</strong><small>成分与质量判定</small></div>
      <nav>{navigation.filter((item) => session && item.roles.includes(session.role)).map((item) => <NavLink key={item.to} to={item.to}>{item.icon}<span>{item.label}</span></NavLink>)}</nav>
      <div className="user-panel">
        <div><span>{session?.displayName}</span><small>{session?.username} · {session?.role}</small></div>
        <Tooltip title="退出登录"><IconButton size="small" onClick={signOut} aria-label="退出登录"><LogoutIcon fontSize="small" /></IconButton></Tooltip>
      </div>
    </aside>
    <section className="content">
      <header className="topbar"><span>生产质量中心</span><span className="live-dot">会话已认证</span></header>
      <Outlet />
    </section>
  </div>;
}
