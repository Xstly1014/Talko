import { useState, useEffect, useContext } from "react";
import { BrowserRouter, Routes, Route, Link, NavLink } from "react-router-dom";
import {
  LayoutDashboard,
  ListChecks,
  ShieldAlert,
  MessagesSquare,
  FileClock,
  LogOut,
  Lock,
} from "lucide-react";
import { api, post, type Staff } from "./api";
import { StaffContext } from "./state";
import { Login } from "./pages/Login";
import { Dashboard } from "./pages/Dashboard";
import { Tasks } from "./pages/Tasks";
import { TaskDetail } from "./pages/TaskDetail";
import { Cases } from "./pages/Cases";
import { Audit } from "./pages/Audit";
export function App() {
  const [staff, setStaff] = useState<Staff | null>(null),
    [loading, setLoading] = useState(true);
  const refresh = async () => {
    try {
      setStaff(await api("/me"));
    } catch {
      setStaff(null);
    } finally {
      setLoading(false);
    }
  };
  useEffect(() => {
    void refresh();
    const expire = () => setStaff(null);
    window.addEventListener("review:unauthorized", expire);
    return () => window.removeEventListener("review:unauthorized", expire);
  }, []);
  if (loading) return <div className="empty">正在验证审核员身份…</div>;
  return (
    <StaffContext.Provider value={{ staff, refresh }}>
      <BrowserRouter>{staff ? <Shell /> : <Login />}</BrowserRouter>
    </StaffContext.Provider>
  );
}
function Shell() {
  const { staff, refresh } = useContext(StaffContext);
  const canRisk = staff?.role === "admin" || staff?.role === "risk";
  return (
    <>
      <aside className="sidebar">
        <Link className="brand" to="/">
          ✳ Talko<span>内容与风险审核</span>
        </Link>
        <p className="nav-title">工作空间</p>
        <nav>
          <NavLink to="/" end>
            <LayoutDashboard size={18} />
            工作台
          </NavLink>
          <NavLink to="/tasks">
            <ListChecks size={18} />
            内容审核
          </NavLink>
          {canRisk && (
            <>
              <NavLink to="/reports">
                <ShieldAlert size={18} />
                举报处理
              </NavLink>
              <NavLink to="/appeals">
                <MessagesSquare size={18} />
                申诉复核
              </NavLink>
            </>
          )}
          {staff?.role === "admin" && (
            <NavLink to="/audit">
              <FileClock size={18} />
              操作审计
            </NavLink>
          )}
        </nav>
        <div className="staff">
          <span className="avatar">{staff?.email[0].toUpperCase()}</span>
          <div>
            <b>
              {staff?.role === "admin"
                ? "管理员"
                : staff?.role === "risk"
                  ? "风控审核员"
                  : "内容审核员"}
            </b>
            <small>{staff?.email}</small>
          </div>
          <button
            className="icon"
            aria-label="退出登录"
            onClick={async () => {
              await post("/auth/logout");
              await refresh();
            }}
          >
            <LogOut size={17} />
          </button>
        </div>
      </aside>
      <main>
        <header>
          <span>审核中心 / 工作空间</span>
          <div>
            <span className="live-dot" /> 人工审核模式
          </div>
        </header>
        <Routes>
          <Route path="/" element={<Dashboard />} />
          <Route path="/tasks" element={<Tasks />} />
          <Route path="/tasks/:id" element={<TaskDetail />} />
          <Route
            path="/reports"
            element={canRisk ? <Cases type="reports" /> : <Denied />}
          />
          <Route
            path="/appeals"
            element={canRisk ? <Cases type="appeals" /> : <Denied />}
          />
          <Route
            path="/audit"
            element={staff?.role === "admin" ? <Audit /> : <Denied />}
          />
          <Route path="*" element={<Denied />} />
        </Routes>
      </main>
    </>
  );
}
function Denied() {
  return (
    <section className="panel empty">
      <Lock size={32} />
      <h2>无权访问此页面</h2>
      <Link to="/">返回工作台</Link>
    </section>
  );
}
