import {
  createContext,
  useContext,
  useEffect,
  useState,
  type ReactNode,
} from "react";
import { api, type Account } from "../api/client";

type AuthValue = {
  account: Account | null;
  loading: boolean;
  login: (username: string, password: string) => Promise<void>;
  logout: () => Promise<void>;
};
const AuthContext = createContext<AuthValue | null>(null);

export function AuthProvider({ children }: { children: ReactNode }) {
  const [account, setAccount] = useState<Account | null>(null);
  const [loading, setLoading] = useState(true);
  useEffect(() => {
    api<Account>("/auth/me")
      .then(setAccount)
      .catch(() => setAccount(null))
      .finally(() => setLoading(false));
  }, []);
  async function login(username: string, password: string) {
    setAccount(
      await api<Account>("/auth/login", {
        method: "POST",
        body: JSON.stringify({ username, password }),
      }),
    );
  }
  async function logout() {
    try {
      await api<void>("/auth/logout", { method: "POST" });
    } finally {
      setAccount(null);
    }
  }
  return (
    <AuthContext.Provider value={{ account, loading, login, logout }}>
      {children}
    </AuthContext.Provider>
  );
}

// AuthProvider and its hook intentionally share the same private context.
// eslint-disable-next-line react-refresh/only-export-components
export function useAuth() {
  const value = useContext(AuthContext);
  if (!value) throw new Error("useAuth must be used within AuthProvider");
  return value;
}
