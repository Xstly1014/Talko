import { useEffect, useRef, useState } from "react";
import { Link } from "react-router-dom";
import { send } from "../api";
export function ReportDialog({
  path,
  title,
  onClose,
}: {
  path: string;
  title: string;
  onClose: () => void;
}) {
  const ref = useRef<HTMLDialogElement>(null);
  const [reason, setReason] = useState(""),
    [busy, setBusy] = useState(false),
    [error, fail] = useState(""),
    [done, setDone] = useState(false);
  useEffect(() => {
    ref.current?.showModal();
    return () => ref.current?.close();
  }, []);
  return (
    <dialog
      ref={ref}
      className="report-dialog"
      onCancel={(e) => {
        e.preventDefault();
        if (!busy) onClose();
      }}
    >
      <form
        onSubmit={async (e) => {
          e.preventDefault();
          setBusy(true);
          fail("");
          try {
            await send(path, { reason });
            setDone(true);
          } catch (e) {
            fail((e as Error).message);
          } finally {
            setBusy(false);
          }
        }}
      >
        <h2>{done ? "举报已提交" : title}</h2>
        {done ? (
          <>
            <p>平台将核查内容与举报依据。处理结果可在举报记录中查看。</p>
            <Link
              to={
                path.startsWith("/comments/") ? "/comment-reports" : "/reports"
              }
              onClick={onClose}
            >
              查看处理进度 →
            </Link>
            <div className="toolbar">
              <button type="button" className="primary" onClick={onClose}>
                完成
              </button>
            </div>
          </>
        ) : (
          <>
            <p className="muted">请说明具体问题，帮助审核人员准确判断。</p>
            <label>
              举报说明
              <textarea
                autoFocus
                required
                minLength={5}
                maxLength={500}
                rows={5}
                value={reason}
                onChange={(e) => setReason(e.target.value)}
                placeholder="例如：招聘内容要求先缴纳押金，与描述不符"
              />
            </label>
            <p className="fine">{reason.length} / 500 字 · 至少 5 字</p>
            {error && (
              <p className="error" role="alert">
                {error}
              </p>
            )}
            <div className="toolbar">
              <button type="button" disabled={busy} onClick={onClose}>
                取消
              </button>
              <button
                className="primary"
                disabled={busy || reason.trim().length < 5}
              >
                {busy ? "正在提交…" : "提交举报"}
              </button>
            </div>
          </>
        )}
      </form>
    </dialog>
  );
}
