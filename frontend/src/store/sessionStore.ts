import { useEffect } from "react";
import { create } from "zustand";

const SESSION_STORAGE_KEY = "aegisops-session";

type SessionState = {
  bootstrapped: boolean;
  token: string | null;
  bootstrapSession: () => void;
  setSession: (token: string | null) => void;
  clearSession: () => void;
};

function readStoredToken() {
  if (typeof window === "undefined") {
    return null;
  }
  return window.localStorage.getItem(SESSION_STORAGE_KEY);
}

function writeStoredToken(token: string | null) {
  if (typeof window === "undefined") {
    return;
  }
  if (token) {
    window.localStorage.setItem(SESSION_STORAGE_KEY, token);
    return;
  }
  window.localStorage.removeItem(SESSION_STORAGE_KEY);
}

export const useSessionStore = create<SessionState>((set) => ({
  bootstrapped: false,
  token: null,
  bootstrapSession: () => {
    set({ token: readStoredToken(), bootstrapped: true });
  },
  setSession: (token) => {
    writeStoredToken(token);
    set({ token, bootstrapped: true });
  },
  clearSession: () => {
    writeStoredToken(null);
    set({ token: null, bootstrapped: true });
  },
}));

export function useBootstrapSession() {
  const bootstrapSession = useSessionStore((state) => state.bootstrapSession);

  useEffect(() => {
    bootstrapSession();
  }, [bootstrapSession]);
}
