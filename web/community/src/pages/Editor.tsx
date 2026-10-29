import { useEffect, useState } from "react";
import { useNavigate, useParams } from "react-router-dom";
import { ImagePlus, ShieldCheck } from "lucide-react";
import { api, send, media, type Post, type Opportunity } from "../api";

import { Feedback, Require } from "../components/ui";
import { categories } from "../constants";
export function Editor() {
  const { id } = useParams(),
    nav = useNavigate();
  const [title, setTitle] = useState(""),
    [body, setBody] = useState(""),
    [type, setType] = useState("note"),
    [category, setCategory] = useState("校园生活"),
    [ids, setIds] = useState<string[]>([]),
    [rev, setRev] = useState(""),
    [error, fail] = useState(""),
    [busy, setBusy] = useState(false),
    [opp, setOpp] = useState<Opportunity>({
      publisher: "",
      city: "",
      compensation: "",
      settlement: "",
      expires_at: "",
      contact: "",
    });
  useEffect(() => {
    if (id)
      api<Post>("/posts/" + id + "?owner=1")
        .then((p) => {
          setTitle(p.title);
          setBody(p.body);
          setType(p.type);
          setCategory(p.category);
          setIds(p.media_ids);
          setRev(p.revision_id);
          if (p.opportunity) setOpp(p.opportunity);
        })
        .catch((e) => fail(e.message));
  }, [id]);
  async function save(submit: boolean) {
    setBusy(true);
    fail("");
    try {
      const d = {
        title,
        body,
        type,
        category,
        media_ids: ids,
        revision_id: rev,
        opportunity:
          type === "opportunity"
            ? { ...opp, expires_at: new Date(opp.expires_at).toISOString() }
            : null,
      };
      const result = await send(
        id ? "/posts/" + id + "/draft" : "/posts",
        d,
        id ? "PATCH" : "POST",
        crypto.randomUUID(),
      );
      if (submit)
        await send("/posts/" + result.id + "/submit", {
          revision_id: result.revision_id,
        });
      nav("/mine");
    } catch (e) {
      fail((e as Error).message);
    } finally {
      setBusy(false);
    }
  }
  return (
    <Require>
      <div className="page-heading">
        <span className="eyebrow">CREATOR STUDIO</span>
        <h1>{id ? "编辑内容" : "分享一段校园经历"}</h1>
        <p>提交后进入人工审核，通过后公开。请避免发布他人的敏感信息。</p>
      </div>
      <div className="editor-layout">
        <form
          className="panel"
          onSubmit={(e) => {
            e.preventDefault();
            void save(true);
          }}
        >
          <div className="inline">
            <label>
              内容类型
              <select
                value={type}
                disabled={!!id}
                onChange={(e) => setType(e.target.value)}
              >
                <option value="note">图文笔记</option>
                <option value="opportunity">兼职 / 家教 / 实习机会</option>
              </select>
            </label>
            <label>
              分类
              <select
                value={category}
                onChange={(e) => setCategory(e.target.value)}
              >
                {categories.slice(1).map((c) => (
                  <option key={c}>{c}</option>
                ))}
              </select>
            </label>
          </div>
          <label>
            标题 <small>{title.length}/60</small>
            <input
              required
              maxLength={60}
              value={title}
              onChange={(e) => setTitle(e.target.value)}
              placeholder="清晰的标题，让好内容被看见"
            />
          </label>
          <label>
            正文
            <textarea
              required
              rows={10}
              maxLength={10000}
              value={body}
              onChange={(e) => setBody(e.target.value)}
              placeholder="写下经验、背景和具体信息…"
            />
          </label>
          <label className="upload">
            <ImagePlus size={24} /> 添加图片（JPEG / PNG，最多 6 张，每张 5 MB）
            <input
              type="file"
              accept="image/jpeg,image/png"
              multiple
              disabled={busy}
              onChange={async (e) => {
                setBusy(true);
                fail("");
                try {
                  const files = Array.from(e.target.files || []);
                  if (files.length + ids.length > 6)
                    throw new Error("最多 6 张图片");
                  const next: string[] = [];
                  for (const f of files) {
                    if (f.size > 5 * 1024 * 1024)
                      throw new Error("每张图片不能超过 5 MB");
                    const r = await api<{ id: string }>("/media", {
                      method: "POST",
                      body: f,
                      headers: { "Content-Type": f.type },
                    });
                    next.push(r.id);
                  }
                  setIds((old) => [...old, ...next]);
                } catch (e) {
                  fail((e as Error).message);
                } finally {
                  setBusy(false);
                }
              }}
            />
          </label>
          <div className="previews">
            {ids.map((mid) => (
              <div key={mid}>
                <img src={media(mid)} alt="待发布图片" />
                <button
                  type="button"
                  aria-label="移除图片"
                  onClick={() => setIds(ids.filter((i) => i !== mid))}
                >
                  ×
                </button>
              </div>
            ))}
          </div>
          {type === "opportunity" && (
            <fieldset>
              <legend>机会信息</legend>
              {[
                ["publisher", "发布方"],
                ["city", "工作城市 / 地点"],
                ["compensation", "报酬及单位"],
                ["settlement", "结算方式"],
                ["contact", "联系渠道（公开展示，请勿填写证件信息）"],
                ["expires_at", "有效期"],
              ].map(([k, label]) => (
                <label key={k}>
                  {label}
                  <input
                    required
                    type={k === "expires_at" ? "datetime-local" : "text"}
                    value={opp[k as keyof Opportunity]?.slice(
                      0,
                      k === "expires_at" ? 16 : undefined,
                    )}
                    onChange={(e) => setOpp({ ...opp, [k]: e.target.value })}
                  />
                </label>
              ))}
            </fieldset>
          )}
          <Feedback error={error} />
          <div className="form-actions">
            <button
              type="button"
              disabled={busy || !title || !body}
              onClick={() => void save(false)}
            >
              保存草稿
            </button>
            <button className="primary" disabled={busy}>
              {busy ? "正在处理…" : "提交审核"}
            </button>
          </div>
        </form>
        <aside className="panel guidance">
          <ShieldCheck size={25} />
          <h3>真实、清楚、有帮助</h3>
          <p>分享你实际经历的事情，尊重他人的隐私和版权。</p>
          <h3>机会信息提醒</h3>
          <p>
            个人可以发布。涉及金钱的内容会展示来源未核实提示；禁止押金、违规收费和虚假承诺。
          </p>
          <p className="fine">
            模型审核尚未接入，当前所有内容和评论进入人工审核。
          </p>
        </aside>
      </div>
    </Require>
  );
}
