import { createContext, useCallback, useContext, useEffect, useMemo, useState, type ReactNode } from 'react';
import { login } from '../api/auth';
import { clearSession, readSession, saveSession } from '../api/client';
import type { UserRole, UserSession } from '../types/domain';

interface AuthContextValue {
  session: UserSession | null;
  loading: boolean;
  signIn: (username: string, password: string) => Promise<void>;
  logout: () => void;
  hasRole: (...roles: UserRole[]) => boolean;
}

const AuthContext = createContext<AuthContextValue | null>(null);

export function AuthProvider({ children }: { children: ReactNode }) {
  const [session, setSession] = useState<UserSession | null>(() => readSession());
  const [loading, setLoading] = useState(false);

  const signIn = useCallback(async (username: string, password: string) => {
    setLoading(true);
    try {
      const next = await login(username, password);
      saveSession(next);
      setSession(next);
    } finally {
      setLoading(false);
    }
  }, []);

  const logout = useCallback(() => {
    clearSession();
    setSession(null);
  }, []);

  useEffect(() => {
    window.addEventListener('foundry:session-expired', logout);
    return () => window.removeEventListener('foundry:session-expired', logout);
  }, [logout]);

  const value = useMemo<AuthContextValue>(() => ({
    session, loading, signIn, logout,
    hasRole: (...roles) => Boolean(session && roles.includes(session.role)),
  }), [loading, logout, session, signIn]);

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth(): AuthContextValue {
  const value = useContext(AuthContext);
  if (!value) throw new Error('useAuth must be used within AuthProvider');
  return value;
}
