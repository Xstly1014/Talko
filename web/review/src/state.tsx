import { createContext, useState, useEffect } from "react";
import { api, type Staff } from "./api";
export const StaffContext = createContext<{
  staff: Staff | null;
  refresh: () => Promise<void>;
}>({ staff: null, refresh: async () => {} });
export function useData<T>(path: string) {
  const [data, setData] = useState<T | null>(null),
    [error, setError] = useState(""),
    [loading, setLoading] = useState(true),
    [n, setN] = useState(0);
  useEffect(() => {
    let valid = true;
    setLoading(true);
    setError("");
    api<T>(path)
      .then((d) => {
        if (valid) setData(d);
      })
      .catch((e) => {
        if (valid) setError(e.message);
      })
      .finally(() => {
        if (valid) setLoading(false);
      });
    return () => {
      valid = false;
    };
  }, [path, n]);
  return { data, error, loading, reload: () => setN((x) => x + 1) };
}
