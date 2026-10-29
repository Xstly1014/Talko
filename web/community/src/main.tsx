import React, { useState, useEffect } from "react";
import { createRoot } from "react-dom/client";
import {
  BrowserRouter,
  Link,
  NavLink,
  Routes,
  Route,
  useNavigate,
} from "react-router-dom";
import {
  Briefcase,
  Home,
  Users,
  Search,
  Plus,
  BookOpen,
  Bell,
  Bookmark,
  LogOut,
  ArrowUpRight,
} from "lucide-react";
import { api, send, type User } from "./api";
import { Auth, useAuth } from "./state/auth";
import { Avatar, Empty } from "./components/ui";
import { Feed } from "./pages/Feed";
import { Login } from "./pages/Login";
import { Editor } from "./pages/Editor";
import { Mine } from "./pages/Mine";
import { Detail } from "./pages/Detail";
import { Profile, PublicProfile } from "./pages/Profile";
import { Records } from "./pages/Records";
import { Help } from "./pages/Help";
import "./style.css";
function App() {
  const [user, setUser] = useState<User | null>(null);
  const refresh = async () => {
    try {
      setUser(await api<User>("/me"));
    } catch {
      setUser(null);
    }
  };
  useEffect(() => {
    void refresh();
  }, []);
  return (
    <Auth.Provider value={{ user, refresh }}>
      <BrowserRouter>
        <Shell />
      </BrowserRouter>
    </Auth.Provider>
  );
}
function Shell() {
  const { user, refresh } = useAuth(),
    navigate = useNavigate();
  const [search, setSearch] = useState("");
  return (
    <>
      <aside className="sidebar">
        <Link className="logo" to="/">
          ✳ <b>talko</b>
          <small>CAMPUS & BEYOND</small>
        </Link>
        <nav>
          {[
            [Home, "发现", "/"],
            [Users, "关注", "/following"],
            [Briefcase, "校园机会", "/opportunities"],
            [BookOpen, "我的发布", "/mine"],
            [Bookmark, "我的收藏", "/saved"],
            [Bell, "消息", "/notifications"],
          ].map(([Icon, label, to]) => {
            const I = Icon as typeof Home;
            return (
              <NavLink key={String(to)} to={String(to)} end>
                <I size={20} />
                <span>{String(label)}</span>
              </NavLink>
            );
          })}
        </nav>
        <Link to="/compose" className="button primary publish">
          <Plus size={19} />
          发布内容
        </Link>
        <div className="side-bottom">
          <Link to="/help">
            社区规则与帮助 <ArrowUpRight size={14} />
          </Link>
          {user ? (
            <>
              <Link to="/profile">
                <Avatar name={user.nickname} id={user.avatar_id} />
                <b>{user.nickname}</b>
              </Link>
              <button
                className="text"
                onClick={async () => {
                  await send("/auth/logout", {});
                  await refresh();
                  navigate("/");
                }}
              >
                <LogOut size={14} />
                退出登录
              </button>
            </>
          ) : (
            <Link to="/auth" className="button">
              登录 / 注册
            </Link>
          )}
        </div>
      </aside>
      <main>
        <header>
          <form
            onSubmit={(e) => {
              e.preventDefault();
              navigate("/?q=" + encodeURIComponent(search));
            }}
            className="search"
          >
            <Search size={18} />
            <input
              aria-label="搜索内容"
              placeholder="搜索校园经验、实习与学习记录"
              value={search}
              onChange={(e) => setSearch(e.target.value)}
            />
          </form>
          <span className="environment">邀请试运营筹备中</span>
          {user && (
            <Link to="/profile">
              <Avatar name={user.nickname} id={user.avatar_id} />
            </Link>
          )}
        </header>
        <Routes>
          <Route path="/" element={<Feed />} />
          <Route path="/following" element={<Feed mode="following" />} />
          <Route path="/opportunities" element={<Feed mode="opportunity" />} />
          <Route path="/saved" element={<Feed mode="saved" />} />
          <Route path="/posts/:id" element={<Detail />} />
          <Route path="/compose" element={<Editor />} />
          <Route path="/edit/:id" element={<Editor />} />
          <Route path="/mine" element={<Mine />} />
          <Route path="/auth" element={<Login />} />
          <Route path="/profile" element={<Profile />} />
          <Route path="/users/:id" element={<PublicProfile />} />
          <Route
            path="/notifications"
            element={<Records type="notifications" />}
          />
          <Route path="/reports" element={<Records type="reports" />} />
          <Route path="/appeals" element={<Records type="appeals" />} />
          <Route path="/help" element={<Help />} />
          <Route path="*" element={<Empty title="页面不存在" />} />
        </Routes>
      </main>
    </>
  );
}
createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <App />
  </React.StrictMode>,
);
