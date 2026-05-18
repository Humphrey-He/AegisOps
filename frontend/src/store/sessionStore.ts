import { useEffect } from "react";
import { create } from "zustand";
import { authApi } from "../lib/api";
import { ApiError } from "../types/api";
import type { AuthSession, CurrentUserPayload, User } from "../types/models";

const SESSION_STORAGE_KEY = "aegisops-mvp-session";

type StoredSessionSnapshot = {
  token: string;
  refreshToken?: string;
  user?: User | null;
  permissions?: string[];
};

type SessionStore = {
  token: string | null;
  refreshToken?: string;
  user: User | null;
  permissions: string[];
  initialized: boolean;
  bootstrapped: boolean;
  setSession: (payload: AuthSession) => void;
  clearSession: () => void;
  setInitialized: (initialized: boolean) => void;
  setBootstrapped: (bootstrapped: boolean) => void;
};

function readStoredSession(): StoredSessionSnapshot | null {
  if (typeof window === "undefined") {
    return null;
  }
  const raw = window.localStorage.getItem(SESSION_STORAGE_KEY);
  if (!raw) {
    return null;
  }
  if (!raw.trim().startsWith("{")) {
    return { token: raw };
  }
  try {
    const parsed = JSON.parse(raw) as StoredSessionSnapshot;
    if (!parsed?.token) {
      return null;
    }
    return {
      token: parsed.token,
      refreshToken: parsed.refreshToken,
      user: parsed.user ?? null,
      permissions: parsed.permissions ?? [],
    };
  } catch {
    return null;
  }
}

function persistSession(session: StoredSessionSnapshot | null) {
  if (typeof window === "undefined") {
    return;
  }
  if (!session?.token) {
    window.localStorage.removeItem(SESSION_STORAGE_KEY);
    return;
  }
  window.localStorage.setItem(SESSION_STORAGE_KEY, JSON.stringify(session));
}

const storedSession = readStoredSession();

export const useSessionStore = create<SessionStore>((set) => ({
  token: storedSession?.token ?? null,
  refreshToken: storedSession?.refreshToken,
  user: storedSession?.user ?? null,
  permissions: storedSession?.permissions ?? [],
  initialized: false,
  bootstrapped: false,
  setSession: ({ token, refreshToken, user, permissions }) => {
    persistSession({ token, refreshToken, user, permissions });
    set({ token, refreshToken, user, permissions });
  },
  clearSession: () => {
    persistSession(null);
    set({ token: null, refreshToken: undefined, user: null, permissions: [] });
  },
  setInitialized: (initialized) => set({ initialized }),
  setBootstrapped: (bootstrapped) => set({ bootstrapped }),
}));

export function useBootstrapSession() {
  const setInitialized = useSessionStore((state) => state.setInitialized);
  const setBootstrapped = useSessionStore((state) => state.setBootstrapped);
  const setSession = useSessionStore((state) => state.setSession);
  const clearSession = useSessionStore((state) => state.clearSession);

  useEffect(() => {
    let mounted = true;

    async function bootstrap() {
      try {
        const setupStatus = await authApi.getSetupStatus();
        if (mounted) {
          setInitialized(setupStatus.initialized);
        }
        const token = useSessionStore.getState().token;
        if (setupStatus.initialized && token) {
          try {
            const result = await authApi.me();
            if (mounted) {
              setSession({
                token,
                refreshToken: useSessionStore.getState().refreshToken,
                user: result.user,
                permissions: result.permissions,
              });
            }
          } catch (error) {
            if (mounted && error instanceof ApiError && (error.status === 401 || error.status === 403)) {
              clearSession();
            }
          }
        }
      } finally {
        if (mounted) {
          setBootstrapped(true);
        }
      }
    }

    void bootstrap();
    return () => {
      mounted = false;
    };
  }, [clearSession, setBootstrapped, setInitialized, setSession]);
}
