// P5-5e(ADR-0321 §2): Showdown 形式から新しい構築を取り込む領域(2段階: 内容を確認 → この内容で作成)。
// 入力・プレビューはこの部品の useState(TeamScreen は訪れたタブでも mount し続けるので、タブを往復しても残る。ADR-0308)。

import { useEffect, useRef, useState, type ReactNode } from "react";
import type { components } from "../api/openapi.gen";
import { teamShowdownText } from "../i18n/team";
import type { MasterData, MasterSpeciesSearch } from "../master/types";
import { parseShowdownTeam } from "./showdownFormat";
import { planShowdownImport, type ImportPlan } from "./showdownImportPlan";
import { resolveMasterForImport } from "./showdownMaster";
import type { TeamClient, TeamError } from "./teamClient";
import { teamNameNotice } from "./teamName";

type Schemas = components["schemas"];

/** 確認の状態。ready の plan と、名前の表示に使う解決後のマスタを持つ。 */
type PreviewState =
  | { readonly status: "idle" }
  | { readonly status: "resolving" }
  | { readonly status: "ready"; readonly plan: ImportPlan; readonly resolved: MasterData };

export interface TeamShowdownImportProps {
  readonly teamClient: TeamClient;
  readonly master: MasterData;
  readonly masterSearch?: MasterSpeciesSearch;
  /** create() が成功したときの応答(TeamScreen が一覧の先頭に足す)。 */
  readonly onCreated: (team: Schemas["Team"]) => void;
}

export function TeamShowdownImport({
  teamClient,
  master,
  masterSearch,
  onCreated,
}: TeamShowdownImportProps): ReactNode {
  const [text, setText] = useState("");
  const [name, setName] = useState("");
  const [preview, setPreview] = useState<PreviewState>({ status: "idle" });
  const [submitting, setSubmitting] = useState(false);
  const [notice, setNotice] = useState<string | null>(null);
  const [error, setError] = useState<TeamError | null>(null);
  const [created, setCreated] = useState<string | null>(null);
  // 確認の世代。編集・マスタの切り替えで進め、古い確認の応答を捨てる。
  const generationRef = useRef(0);

  function discardPreview(): void {
    generationRef.current += 1;
    setPreview({ status: "idle" });
    setNotice(null);
    setCreated(null);
  }

  // 計算モードの切り替えなどでマスタが変わったら、古いマスタで作ったプレビューを捨てる(ADR-0304 A-6)。
  const [prevMaster, setPrevMaster] = useState({ master, masterSearch });
  if (prevMaster.master !== master || prevMaster.masterSearch !== masterSearch) {
    setPrevMaster({ master, masterSearch });
    setPreview({ status: "idle" });
    setNotice(null);
    setCreated(null);
  }
  // 解決中の確認が古いマスタのまま終わっても反映しない。
  useEffect(() => {
    generationRef.current += 1;
  }, [master, masterSearch]);

  async function handlePreview(): Promise<void> {
    generationRef.current += 1;
    const generation = generationRef.current;
    setPreview({ status: "resolving" });
    setNotice(null);
    setCreated(null);
    setError(null);
    const resolved = await resolveMasterForImport(text, master, masterSearch);
    if (generation !== generationRef.current) {
      return;
    }
    const plan = planShowdownImport(parseShowdownTeam(text, resolved), resolved);
    setPreview({ status: "ready", plan, resolved });
  }

  async function handleCreate(): Promise<void> {
    if (submitting || preview.status !== "ready" || !preview.plan.canCreate) {
      return;
    }
    const reason = teamNameNotice(name);
    if (reason !== null) {
      setNotice(reason);
      setError(null);
      return;
    }
    const trimmed = name.trim();
    const { members } = preview.plan;
    setSubmitting(true);
    setNotice(null);
    setError(null);
    const result = await teamClient.create({ name: trimmed, members });
    setSubmitting(false);
    if (result.ok) {
      generationRef.current += 1;
      setText("");
      setName("");
      setPreview({ status: "idle" });
      setCreated(teamShowdownText.importCreated(trimmed, members.length));
      onCreated(result.value);
    } else {
      setError(result.error);
    }
  }

  const resolving = preview.status === "resolving";
  const canCreate = preview.status === "ready" && preview.plan.canCreate && !submitting;

  return (
    <section aria-label={teamShowdownText.importRegionLabel} className="team-showdown">
      <h2>{teamShowdownText.importRegionLabel}</h2>
      <label className="team-screen__field">
        <span>{teamShowdownText.importTextLabel}</span>
        <textarea
          rows={8}
          value={text}
          onChange={(event) => {
            setText(event.target.value);
            discardPreview();
          }}
        />
      </label>
      <label className="team-screen__field">
        <span>{teamShowdownText.importNameLabel}</span>
        <input
          type="text"
          value={name}
          onChange={(event) => {
            setName(event.target.value);
            discardPreview();
          }}
        />
      </label>
      <div className="team-showdown__actions">
        <button
          type="button"
          className="ui-button ui-button--secondary"
          disabled={resolving || submitting}
          onClick={() => {
            void handlePreview();
          }}
        >
          {teamShowdownText.importPreviewLabel}
        </button>
        <button
          type="button"
          className="ui-button ui-button--primary"
          disabled={!canCreate}
          onClick={() => {
            void handleCreate();
          }}
        >
          {teamShowdownText.importCreateLabel}
        </button>
      </div>

      {resolving && (
        <p role="status" className="team-screen__notice">
          {teamShowdownText.importResolving}
        </p>
      )}
      {preview.status === "ready" && <PreviewResult plan={preview.plan} resolved={preview.resolved} />}
      {notice !== null && <p className="team-screen__notice">{notice}</p>}
      {error !== null && (
        <div role="alert" className="team-screen__error">
          <p>{teamShowdownText.importErrorHeading}</p>
          <p>{error.message}</p>
        </div>
      )}
      {created !== null && (
        <p role="status" className="team-screen__notice">
          {created}
        </p>
      )}
    </section>
  );
}

function PreviewResult({ plan, resolved }: { plan: ImportPlan; resolved: MasterData }): ReactNode {
  const hasError = plan.issues.some((issue) => issue.severity === "error");
  return (
    <>
      <p role="status" className="team-screen__notice">
        {plan.canCreate ? teamShowdownText.previewSummary(plan.members.length) : teamShowdownText.previewNone}
      </p>
      {plan.issues.length > 0 && (
        <div role={hasError ? "alert" : "status"} className="team-showdown__issues">
          <ul aria-label={teamShowdownText.issuesLabel}>
            {plan.issues.map((issue, index) => (
              <li key={index}>{teamShowdownText.issueText(issue)}</li>
            ))}
          </ul>
        </div>
      )}
      {plan.notes.length > 0 && (
        <ul aria-label={teamShowdownText.notesLabel} className="team-showdown__notes">
          {plan.notes.map((note, index) => (
            <li key={index}>
              {teamShowdownText.megaNoteText(
                note,
                resolved.species.find((species) => species.key === note.speciesKey)?.nameJa ??
                  note.speciesKey,
              )}
            </li>
          ))}
        </ul>
      )}
    </>
  );
}
