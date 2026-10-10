// G-05(ADR-0339): シート。狭い幅は下から出るシート、広い幅は右の面(CSS の @media で切り替える)。
// 作法は「このアプリについて」の確認ダイアログ(role="alertdialog")と同じ: 開いたら中へフォーカス・Esc で閉じる・
// Tab は中で回る・閉じたら開く前にフォーカスのあった要素へ戻す。
// 出入りは操作のときだけの transition(時間は --duration-sheet。「視差効果を減らす」で 0)。

import {
  useEffect,
  useId,
  useRef,
  useState,
  type KeyboardEvent,
  type ReactElement,
  type ReactNode,
} from "react";
import { createPortal } from "react-dom";
import { uiText } from "../i18n/ui";
import { Icon } from "./Icon";
import { isTopModal, lockModal } from "./modalLock";
import { prefersReducedMotion } from "./motion";

export interface SheetProps {
  readonly open: boolean;
  /** 見出し(dialog の名前)。 */
  readonly title: string;
  readonly onClose: () => void;
  readonly children: ReactNode;
}

const FOCUSABLE =
  'a[href], button:not([disabled]), input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])';

/** CSS の --duration-sheet(ミリ秒)。読めない・「視差効果を減らす」のときは 0。 */
function sheetDurationMs(element: Element): number {
  if (prefersReducedMotion()) {
    return 0;
  }
  const raw = getComputedStyle(element).getPropertyValue("--duration-sheet").trim();
  const seconds = /^([\d.]+)s$/.exec(raw)?.[1];
  const ms = /^([\d.]+)ms$/.exec(raw)?.[1];
  if (seconds !== undefined) {
    return Number(seconds) * 1000;
  }
  return ms === undefined ? 0 : Number(ms);
}

export function Sheet({ open, title, onClose, children }: SheetProps): ReactElement | null {
  const titleId = useId();
  const onCloseRef = useRef(onClose);
  useEffect(() => {
    onCloseRef.current = onClose;
  });
  const scrimRef = useRef<HTMLDivElement>(null);
  const pressStartedOnScrim = useRef(false);
  const panelRef = useRef<HTMLDivElement>(null);
  const bodyRef = useRef<HTMLDivElement>(null);
  const returnRef = useRef<HTMLElement | null>(null);
  // mounted: DOM にある。shown: 出ている見た目(transition の行き先)。
  const [mounted, setMounted] = useState(open);
  const [shown, setShown] = useState(false);

  useEffect(() => {
    if (open) {
      returnRef.current = document.activeElement instanceof HTMLElement ? document.activeElement : null;
      // props(open)に同期して DOM に出し入れする。出入りの遅延(タイマー)を扱うため、レンダー中の state 調整にはできない。
      // eslint-disable-next-line react-hooks/set-state-in-effect
      setMounted(true);
      return;
    }
    setShown(false);
    const panel = panelRef.current;
    const wait = panel === null ? 0 : sheetDurationMs(panel);
    if (wait === 0) {
      setMounted(false);
      return;
    }
    const timer = window.setTimeout(() => {
      setMounted(false);
    }, wait);
    return () => {
      window.clearTimeout(timer);
    };
  }, [open]);

  // 出したあと、次のフレームで「出ている」にして transition を走らせ、中へフォーカスを移す。
  useEffect(() => {
    if (!open || !mounted) {
      return;
    }
    const frame = window.requestAnimationFrame(() => {
      setShown(true);
    });
    const first = bodyRef.current?.querySelector<HTMLElement>(FOCUSABLE);
    (first ?? panelRef.current?.querySelector<HTMLElement>(FOCUSABLE))?.focus();
    return () => {
      window.cancelAnimationFrame(frame);
    };
  }, [open, mounted]);

  // 開いている間: body のスクロールを止め、背景を inert にする。Esc は一番上のシートだけが拾う(フォーカスが外にあっても)。
  // 閉じる(open が false)とき cleanup が先に走るので、フォーカスを戻す前に inert は外れている。
  useEffect(() => {
    const scrim = scrimRef.current;
    if (!open || !mounted || scrim === null) {
      return;
    }
    const lock = lockModal(scrim);
    function onDocumentKeyDown(event: globalThis.KeyboardEvent): void {
      if (event.key === "Escape" && !event.defaultPrevented && isTopModal(lock.id)) {
        event.preventDefault();
        onCloseRef.current();
      }
    }
    document.addEventListener("keydown", onDocumentKeyDown);
    return () => {
      document.removeEventListener("keydown", onDocumentKeyDown);
      lock.unlock();
    };
  }, [open, mounted]);

  // 閉じたら、開く前の要素へ戻す。
  const wasOpen = useRef(false);
  useEffect(() => {
    if (wasOpen.current && !open) {
      returnRef.current?.focus();
    }
    wasOpen.current = open;
  }, [open]);

  function onKeyDown(event: KeyboardEvent<HTMLDivElement>): void {
    if (event.key !== "Tab" || panelRef.current === null) {
      return;
    }
    const items = [...panelRef.current.querySelectorAll<HTMLElement>(FOCUSABLE)];
    const first = items[0];
    const last = items[items.length - 1];
    if (first === undefined || last === undefined) {
      event.preventDefault();
      return;
    }
    if (event.shiftKey && document.activeElement === first) {
      event.preventDefault();
      last.focus();
    } else if (!event.shiftKey && document.activeElement === last) {
      event.preventDefault();
      first.focus();
    }
  }

  if (!mounted) {
    return null;
  }

  return createPortal(
    <div
      ref={scrimRef}
      className={`ui-sheet__scrim${shown ? " is-shown" : ""}`}
      onMouseDown={(event) => {
        pressStartedOnScrim.current = event.target === event.currentTarget;
      }}
      onClick={(event) => {
        // 押し始めも幕の上だったときだけ閉じる(パネル内で押して幕の上で離しても閉じない)。
        const startedOnScrim = pressStartedOnScrim.current;
        pressStartedOnScrim.current = false;
        if (startedOnScrim && event.target === event.currentTarget) {
          onClose();
        }
      }}
    >
      <div
        ref={panelRef}
        role="dialog"
        aria-modal="true"
        aria-labelledby={titleId}
        className="ui-sheet"
        onKeyDown={onKeyDown}
      >
        <div className="ui-sheet__header">
          <h2 id={titleId} className="ui-sheet__title">
            {title}
          </h2>
          <button type="button" className="ui-sheet__close" aria-label={uiText.close} onClick={onClose}>
            <Icon name="close" size={20} />
          </button>
        </div>
        <div ref={bodyRef} className="ui-sheet__body">
          {children}
        </div>
      </div>
    </div>,
    document.body,
  );
}
