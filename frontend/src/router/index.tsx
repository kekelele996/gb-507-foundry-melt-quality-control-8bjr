import { Navigate, Outlet, createBrowserRouter, useLocation } from 'react-router-dom';
import App from '../App';
import { useAuth } from '../hooks/useAuth';
import type { UserRole } from '../types/domain';
import LoginPage from '../pages/LoginPage';
import FurnacePage from '../pages/FurnacePage';
import HeatPage from '../pages/HeatPage';
import ChemicalSamplePage from '../pages/ChemicalSamplePage';
import QualityDecisionPage from '../pages/QualityDecisionPage';
import AuditPage from '../pages/AuditPage';

function RequireSession() {
  const { session } = useAuth();
  const location = useLocation();
  return session ? <Outlet /> : <Navigate to="/login" replace state={{ from: location.pathname }} />;
}

function RequireRole({ roles }: { roles: readonly UserRole[] }) {
  const { session } = useAuth();
  return session && roles.includes(session.role) ? <Outlet /> : <Navigate to="/furnaces" replace />;
}

export const router = createBrowserRouter([
  { path: '/login', element: <LoginPage /> },
  {
    element: <RequireSession />,
    children: [{
      path: '/', element: <App />, children: [
        { index: true, element: <Navigate to="/furnaces" replace /> },
        { path: 'furnaces', element: <FurnacePage /> },
        { path: 'heats', element: <HeatPage /> },
        { path: 'samples', element: <ChemicalSamplePage /> },
        { path: 'decisions', element: <QualityDecisionPage /> },
        { element: <RequireRole roles={['reviewer', 'admin']} />, children: [{ path: 'audit', element: <AuditPage /> }] },
      ],
    }],
  },
  { path: '*', element: <Navigate to="/" replace /> },
], { future: { v7_fetcherPersist: true, v7_normalizeFormMethod: true, v7_partialHydration: true, v7_relativeSplatPath: true, v7_skipActionErrorRevalidation: true } });
