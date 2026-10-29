import { createContext, useContext } from "react";

import { type User } from "../api";
export const Auth = createContext<{
  user: User | null;
  refresh: () => Promise<void>;
}>({ user: null, refresh: async () => {} });
export function useAuth() {
  return useContext(Auth);
}
