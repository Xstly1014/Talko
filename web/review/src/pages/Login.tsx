import { useState, useContext } from "react";
import { ShieldCheck, Lock } from "lucide-react";
import { post } from "../api";
import { StaffContext } from "../state";
import { Feedback } from "../components/ui";
export function Login() {
  const [email, setEmail] = useState(""),
    [password, setPassword] = useState(""),
    [error, fail] = useState(""),
    [busy, setBusy] = useState(false);
  const { refresh } = useContext(StaffContext);
  return (
    <div className="login">
      <div className="login-intro">
        <div className="brand">
          ✳ Talko <span>TRUST & SAFETY</span>
        </div>
        <ShieldCheck size={90} strokeWidth={1} />
        <h1>
          每一次判断，
          <br />
          都让社区更值得信任。
        </h1>
        <p>内容审核 · 机会审核 · 举报处置 · 申诉复核</p>
        <small>独立人员账号与社区用户会话隔离。</small>
      </div>
      <form
        className="panel login-form"
        onSubmit={async (e) => {
          e.preventDefault();
          setBusy(true);
          fail("");
          try {
            await post("/auth/login", { email, password });
            await refresh();
          } catch (e) {
            fail((e as Error).message);
          } finally {
            setBusy(false);
          }
        }}
      >
        <span className="eyebrow">REVIEW WORKSPACE</span>
        <h2>登录审核工作台</h2>
        <p className="muted">使用管理员开通的人员账号。</p>
        <label>
          工作邮箱
          <input
            type="email"
            required
            autoComplete="username"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
          />
        </label>
        <label>
          密码
          <input
            type="password"
            required
            autoComplete="current-password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
          />
        </label>
        <Feedback error={error} />
        <button className="primary full" disabled={busy}>
          <Lock size={16} />
          {busy ? "正在登录…" : "安全登录"}
        </button>
        <p className="fine">没有账号？请联系平台管理员开通，不提供公开注册。</p>
      </form>
    </div>
  );
}
