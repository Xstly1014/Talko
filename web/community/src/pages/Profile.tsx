import { Card } from "./Feed";
import { useEffect, useState } from "react";
import { useParams } from "react-router-dom";

import { api, send, type User, type Post } from "../api";
import { useAuth } from "../state/auth";
import { useData } from "../state/useData";
import { Feedback, Avatar, Require, status } from "../components/ui";

export function Profile() {
  const { user, refresh } = useAuth();
  const [nick, setNick] = useState(user?.nickname || ""),
    [bio, setBio] = useState(user?.bio || ""),
    [avatar, setAvatar] = useState(user?.avatar_id || ""),
    [message, msg] = useState(""),
    [busy, setBusy] = useState(false);
  useEffect(() => {
    if (user) {
      setNick(user.profile_change?.nickname || user.nickname);
      setBio(user.profile_change?.bio ?? user.bio);
      setAvatar(user.profile_change?.avatar_id ?? user.avatar_id);
    }
  }, [user]);
  return (
    <Require>
      <div className="page-heading">
        <h1>个人资料</h1>
        <p>资料修改须通过审核，审核期间原公开资料保持展示。</p>
        {user?.profile_change && (
          <div className="risk">
            最近资料状态：{status(user.profile_change.status)}　
            {user.profile_change.reason}
          </div>
        )}
      </div>
      <form
        className="panel narrow"
        onSubmit={async (e) => {
          e.preventDefault();
          setBusy(true);
          msg("");
          try {
            await send(
              "/me/profile",
              { nickname: nick, bio, avatar_id: avatar },
              "PATCH",
            );
            await refresh();
            msg("资料已提交审核，通过后替换公开资料。");
          } catch (e) {
            msg((e as Error).message);
          } finally {
            setBusy(false);
          }
        }}
      >
        <Avatar name={nick || "你"} id={avatar} size={96} />
        <label>
          上传头像
          <input
            type="file"
            accept="image/jpeg,image/png"
            onChange={async (e) => {
              const file = e.target.files?.[0];
              if (!file) return;
              try {
                const r = await api<{ id: string }>("/media", {
                  method: "POST",
                  body: file,
                });
                setAvatar(r.id);
              } catch (e) {
                msg((e as Error).message);
              }
            }}
          />
        </label>
        <label>
          昵称
          <input
            required
            maxLength={30}
            value={nick}
            onChange={(e) => setNick(e.target.value)}
          />
        </label>
        <label>
          简介
          <textarea
            maxLength={150}
            value={bio}
            onChange={(e) => setBio(e.target.value)}
          />
        </label>
        <p>{message}</p>
        <button className="primary" disabled={busy}>
          {busy ? "正在提交…" : "提交资料审核"}
        </button>
        <p className="fine">{user?.email} · 已验证邮箱</p>
      </form>
    </Require>
  );
}

export function PublicProfile() {
  const { id } = useParams();
  const { data: u, error, loading } = useData<User>("/users/" + id);
  const { data } = useData<{ items: Post[] }>("/feed?author=" + id);
  return (
    <>
      <Feedback error={error} loading={loading} />
      {u && (
        <section className="panel profile-banner">
          <Avatar name={u.nickname} id={u.avatar_id} size={96} />
          <div>
            <h1>{u.nickname}</h1>
            <p>{u.bio || "还没有填写简介"}</p>
            <p className="muted">
              {u.followers} 粉丝 · {u.following} 关注
            </p>
          </div>
        </section>
      )}
      <h2>公开内容</h2>
      <div className="cards">
        {data?.items.map((p) => (
          <Card key={p.id} post={p} />
        ))}
      </div>
    </>
  );
}
