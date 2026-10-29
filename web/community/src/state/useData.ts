import { useEffect, useState } from "react";

import { api } from "../api";
export function useData<T>(path: string) {
  const [data, set] = useState<T | null>(null),
    [error, fail] = useState(""),
    [loading, busy] = useState(true),
    [version, reload] = useState(0);
  useEffect(() => {
    let alive = true;
    busy(true);
    fail("");
    api<T>(path)
      .then((d) => {
        if (alive) set(d);
      })
      .catch((e) => {
        if (alive) fail(e.message);
      })
      .finally(() => {
        if (alive) busy(false);
      });
    return () => {
      alive = false;
    };
  }, [path, version]);
  return { data, error, loading, reload: () => reload((v) => v + 1) };
}
