import { useEffect, type ReactNode } from "react";

interface Props {
  /** アクセシブルネーム(「詳細」「登録」など) */
  label: string;
  role?: "dialog" | "alertdialog";
  onClose: () => void;
  children: ReactNode;
}

/** 下から出るシート。背景(オーバーレイ)のタップと Escape で閉じる。 */
export function Modal({ label, role = "dialog", onClose, children }: Props) {
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") onClose();
    };
    window.addEventListener("keydown", onKey);
    return () => {
      window.removeEventListener("keydown", onKey);
    };
  }, [onClose]);

  return (
    <div className="overlay" onClick={onClose}>
      <div
        className="panel"
        role={role}
        aria-modal="true"
        aria-label={label}
        onClick={(e) => {
          e.stopPropagation();
        }}
      >
        {children}
      </div>
    </div>
  );
}

export function ErrorText({ message }: { message: string | null }) {
  return message === null ? null : (
    <p role="alert" className="error">
      {message}
    </p>
  );
}

export function errorMessage(e: unknown): string {
  if (e instanceof Error) return e.message;
  return "失敗しました";
}
