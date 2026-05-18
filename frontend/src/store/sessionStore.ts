import { useEffect } from "react";
import { create } from "zustand";
import { authApi } from "../lib/api";
import type { AuthSession, CurrentUserPayload, User } from "../types/models";

const SESSION_STORAGE_KEY = "aegisops-mvp-session";

type SessionStore = {
  token: string | null;
  user: User | null;
  permissions: string[];
  initialized: boolean;
  bootstrapped: boolean;
  setSession: (payload: AuthSession) => void;
  clearSession: () => void;
  setInitialized: (initialized: boolean) => void;
  setBootstrapped: (bootstrapped: boolean) => void;
};

function readStoredToken() {
  if (typeof window === "undefined") {
    return null;
  }
  return window.localStorage.getItem(SESSION_STORAGE_KEY);
}

function persistToken(token: string | null) {
  if (typeof window === "undefined") {
    return;
  }
  if (!token) {
    window.localStorage.removeItem(SESSION_STORAGE_KEY);
    return;
  }
  window.localStorage.setItem(SESSION_STORAGE_KEY, token);
}

export const useSessionStore = create<SessionStore>((set) => ({
  token: readStoredToken(),
  user: null,
  permissions: [],
  initialized: false,
  bootstrapped: false,
  setSession: ({ token, user, permissions }) => {
    persistToken(token);
    set({ token, user, permissions });
  },
  clearSession: () => {
    persistToken(null);
    set({ token: null, user: null, permissions: [] });
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
                user: result.user,
                permissions: result.permissions,
              });
            }
          } catch {
            if (mounted) {
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
