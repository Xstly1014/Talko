import { Card } from "./Feed";
import { useEffect, useState } from "react";
import { useParams } from "react-router-dom";

import { api, send, type User, type Post } from "../api";
import { useAuth } from "../state/auth";
import { useData } from "../state/useData";
import { Feedback, Avatar, Require } from "../components/ui";

export function Profile() {
  const { user, refresh } = useAuth();
  const [nick, setNick] = useState(user?.nickname || ""),
    [bio, setBio] = useState(user?.bio || ""),
    [avatar, setAvatar] = useState(user?.avatar_id || ""),
    [message, msg] = useState("");
  useEffect(() => {
    if (user) {
      setNick(user.nickname);
      setBio(user.bio);
      setAvatar(user.avatar_id);
    }
  }, [user]);
  return (
    <Require>
      <div className="page-heading">
        <h1>个人资料</h1>
        <p>让社区认识你。</p>
      </div>
      <form
        className="panel narrow"
        onSubmit={async (e) => {
          e.preventDefault();
          try {
            await send(
              "/me/profile",
              { nickname: nick, bio, avatar_id: avatar },
              "PATCH",
            );
            await refresh();
            msg("资料已保存");
          } catch (e) {
            msg((e as Error).message);
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
        <button className="primary">保存资料</button>
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
