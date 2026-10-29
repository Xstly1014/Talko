export type Staff = {
  id: string;
  email: string;
  role: "admin" | "reviewer" | "risk";
};
export async function api<T = any>(
  path: string,
  init: RequestInit = {},
): Promise<T> {
  const headers = new Headers(init.headers);
  if (init.body) headers.set("Content-Type", "application/json");
  const r = await fetch("/review-api/v1" + path, {
    ...init,
    headers,
    credentials: "same-origin",
  });
  const data = await r.json().catch(() => ({ message: "服务响应异常" }));
  if (r.status === 401 && path !== "/auth/login" && path !== "/me")
    window.dispatchEvent(new Event("review:unauthorized"));
  if (!r.ok) throw new Error(data.message || "请求失败");
  return data;
}
export const post = (path: string, body: unknown = {}) =>
  api(path, { method: "POST", body: JSON.stringify(body) });
