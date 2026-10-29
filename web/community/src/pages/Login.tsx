import { useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { ShieldCheck } from "lucide-react";
import { send } from "../api";
import { useAuth } from "../state/auth";

import { Feedback } from "../components/ui";

export function Login() {
  const { refresh } = useAuth(),
    nav = useNavigate();
  const [register, setRegister] = useState(false),
    [email, setEmail] = useState(""),
    [password, setPassword] = useState(""),
    [code, setCode] = useState(""),
    [nickname, setNickname] = useState(""),
    [error, fail] = useState(""),
    [message, msg] = useState(""),
    [busy, setBusy] = useState(false);
  return (
    <div className="login-layout">
      <div>
        <span className="eyebrow">HELLO, TALKO</span>
        <h1>
          找到和你一起
          <br />
          成长的人。
        </h1>
        <p>面向校园，也欢迎每一位认真生活的人。</p>
        <ShieldCheck size={90} strokeWidth={1} />
      </div>
      <form
        className="panel"
        onSubmit={async (e) => {
          e.preventDefault();
          setBusy(true);
          fail("");
          try {
            await send(
              "/auth/" + (register ? "register" : "login"),
              register
                ? { email, password, code, nickname }
                : { email, password },
            );
            await refresh();
            nav("/");
          } catch (e) {
            fail((e as Error).message);
          } finally {
            setBusy(false);
          }
        }}
      >
        <h2>{register ? "创建账号" : "欢迎回来"}</h2>
        <p className="muted">使用普通邮箱，无需学校身份认证。</p>
        <label>
          邮箱
          <input
            type="email"
            required
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            autoComplete="email"
          />
        </label>
        {register && (
          <>
            <label>
              昵称
              <input
                required
                maxLength={30}
                value={nickname}
                onChange={(e) => setNickname(e.target.value)}
              />
            </label>
            <label>
              邮箱验证码
              <div className="inline">
                <input
                  required
                  value={code}
                  onChange={(e) => setCode(e.target.value)}
                  inputMode="numeric"
                />
                <button
                  type="button"
                  disabled={busy}
                  onClick={async () => {
                    fail("");
                    try {
                      await send("/auth/email-code", { email });
                      msg("验证码已发送，请查看邮箱。");
                    } catch (e) {
                      fail((e as Error).message);
                    }
                  }}
                >
                  发送验证码
                </button>
              </div>
            </label>
          </>
        )}
        <label>
          密码
          <input
            type="password"
            required
            minLength={12}
            maxLength={72}
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            autoComplete={register ? "new-password" : "current-password"}
          />
        </label>
        <Feedback error={error} />
        {message && <p className="success">{message}</p>}
        <button className="button primary full" disabled={busy}>
          {busy ? "处理中…" : register ? "验证并注册" : "登录"}
        </button>
        <button
          className="text full"
          type="button"
          onClick={() => {
            setRegister(!register);
            fail("");
          }}
        >
          {register ? "已有账号？登录" : "没有账号？邮箱注册"}
        </button>
        <p className="fine">
          注册前请阅读<Link to="/help">社区规则与隐私说明</Link>。
        </p>
      </form>
    </div>
  );
}
