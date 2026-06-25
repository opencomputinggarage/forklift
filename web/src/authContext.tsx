import { createContext, useContext } from "react";
import { Me } from "./api";

interface AuthContextValue {
  me: Me;
}

const AuthContext = createContext<AuthContextValue | null>(null);

export const AuthProvider = AuthContext.Provider;

export function useAuth() {
  const value = useContext(AuthContext);
  if (!value) throw new Error("useAuth must be used within AuthProvider");
  return value;
}
