// P5-5e(ADR-0321 §4): 構築1件を Showdown 形式で書き出す領域(構築の行の中)。API は呼ばない。
// 名前の無い種族は resolveMasterForExport で必要な分だけ引く。クリップボードが使えなくても例外にせず、手動コピーを案内する。

import { useEffect, useRef, useState, type ReactNode } from "react";
import type { components } from "../api/openapi.gen";
import { teamShowdownText } from "../i18n/team";
import type { MasterData, MasterSpeciesSearch } from "../master/types";
import { exportShowdownTeam, type ShowdownIssue } from "./showdownFormat";
import { resolveMasterForExport } from "./showdownMaster";

type Schemas = components["schemas"];

type CopyState = "idle" | "copied" | "failed";

interface OpenExport {
  /** この書き出しを作ったときのメンバー(構築が保存で変わったら古い書き出しは捨てる)。 */
  readonly members: Schemas["Team"]["members"];
  readonly text: string;
  readonly issues: readonly ShowdownIssue[];
}

export interface TeamShowdownExportProps {
  readonly team: Schemas["Team"];
  readonly master: MasterData;
  readonly masterSearch?: MasterSpeciesSearch;
}

/** 使えるときだけクリップボードを返す(未対応・権限なしの環境では undefined)。 */
function readClipboard(): Clipboard | undefined {
  const value: unknown = Reflect.get(navigator, "clipboard");
  return value === undefined || value === null ? undefined : (value as Clipboard);
}

export function TeamShowdownExport({ team, master, masterSearch }: TeamShowdownExportProps): ReactNode {
  const [open, setOpen] = useState<OpenExport | null>(null);
  const [busy, setBusy] = useState(false);
  const [copy, setCopy] = useState<CopyState>("idle");
  const textareaRef = useRef<HTMLTextAreaElement>(null);
  // 開く・閉じるの世代(解決中に閉じた・別の構築内容になった結果を捨てる)。
  const generationRef = useRef(0);

  const current = open !== null && open.members === team.members ? open : null;

  // 開いた直後に textarea へフォーカスする(コピー・選択をすぐできるように)。
  const shownText = current?.text ?? null;
  useEffect(() => {
    if (shownText !== null) {
      textareaRef.current?.focus();
    }
  }, [shownText]);

  const empty = team.members.length === 0;
  const emptyNoticeId = `team-showdown-empty-${team.id}`;

  async function handleOpen(): Promise<void> {
    generationRef.current += 1;
    const generation = generationRef.current;
    setBusy(true);
    const resolved = await resolveMasterForExport(team.members, master, masterSearch);
    if (generation !== generationRef.current) {
      return;
    }
    const result = exportShowdownTeam(team.members, resolved);
    setCopy("idle");
    setOpen({ members: team.members, text: result.text, issues: result.issues });
    setBusy(false);
  }

  function handleClose(): void {
    generationRef.current += 1;
    setOpen(null);
    setBusy(false);
    setCopy("idle");
  }

  async function handleCopy(text: string): Promise<void> {
    const clipboard = readClipboard();
    try {
      if (clipboard === undefined) {
        throw new Error("clipboard unavailable");
      }
      await clipboard.writeText(text);
      setCopy("copied");
    } catch {
      const textarea = textareaRef.current;
      if (textarea !== null) {
        textarea.focus();
        textarea.setSelectionRange(0, textarea.value.length);
      }
      setCopy("failed");
    }
  }

  return (
    <>
      <button
        type="button"
        className="ui-button ui-button--secondary"
        disabled={empty || busy}
        aria-describedby={empty ? emptyNoticeId : undefined}
        onClick={() => {
          void handleOpen();
        }}
      >
        {teamShowdownText.exportLabel(team.name)}
      </button>
      {empty && (
        <span id={emptyNoticeId} className="team-screen__item-meta">
          {teamShowdownText.exportEmptyNotice}
        </span>
      )}
      {current !== null && (
        <section aria-label={teamShowdownText.exportRegionLabel(team.name)} className="team-showdown">
          <textarea
            ref={textareaRef}
            readOnly
            rows={10}
            aria-label={teamShowdownText.exportTextLabel(team.name)}
            value={current.text}
          />
          <div className="team-showdown__actions">
            <button
              type="button"
              className="ui-button ui-button--primary"
              onClick={() => {
                void handleCopy(current.text);
              }}
            >
              {teamShowdownText.exportCopyLabel}
            </button>
            <button type="button" className="ui-button ui-button--secondary" onClick={handleClose}>
              {teamShowdownText.exportCloseLabel}
            </button>
          </div>
          {copy !== "idle" && (
            <p role="status" className="team-screen__notice">
              {copy === "copied" ? teamShowdownText.exportCopied : teamShowdownText.exportCopyFailed}
            </p>
          )}
          {current.issues.length > 0 && (
            <ul aria-label={teamShowdownText.exportIssuesLabel} className="team-showdown__issues">
              {current.issues.map((issue, index) => (
                <li key={index}>{teamShowdownText.issueText(issue)}</li>
              ))}
            </ul>
          )}
        </section>
      )}
    </>
  );
}
