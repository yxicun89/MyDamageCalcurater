// P5-5 PR-A1: 構築ビルダーの画面(ADR-0309 §4)。
// この段階で扱うのは **一覧・新規作成(名前だけ・メンバーは空)・名前変更・削除** まで。
// メンバー(種族・技・持ち物・特性・性格・SP・テラスタイプ)の編集は PR-A2(TeamMemberEditor.tsx。ADR-0316)、
// Showdown 形式の入出力は判定レーンの web/src/team/showdownFormat.ts(別担当)。
//
// 状態の作り(SpeedScreen.tsx・BalanceScreen.tsx と同じ考え方):
//   - list() はマウント時に1回だけ呼び、cancelled フラグで古い応答を捨てる
//   - create()/update()/remove() が成功したら、応答の Team で手元の一覧を書き換える(list を呼び直さない)
//   - 一覧の読み込みに失敗しても、新規作成のフォームは先に使える(ADR-0309 §4)

import { useEffect, useRef, useState, type ReactNode } from "react";
import type { components } from "../api/openapi.gen";
import { teamMemberText, teamScreenText } from "../i18n/ja";
import type { MasterData, MasterSpeciesSearch } from "../master/types";
import "./TeamScreen.css";
import { TeamMemberEditor } from "./TeamMemberEditor";
import type { TeamClient, TeamError } from "./teamClient";
import { MAX_TEAM_NAME_LENGTH, teamNameNotice } from "./teamName";
import { TeamShowdownExport } from "./TeamShowdownExport";
import { TeamShowdownImport } from "./TeamShowdownImport";

type Schemas = components["schemas"];

/** パーティの上限(api/openapi.yaml の TeamInput.members の maxItems)。 */
export const MAX_TEAM_MEMBERS = 6;

export { MAX_TEAM_NAME_LENGTH };

/**
 * 画面の props(App.tsx が app/screens.tsx 経由で注入する。ADR-0309 §2)。
 * 一覧・作成・名前変更・削除は master を使わないが、メンバー編集(PR-A2。ADR-0316)が種族・技・持ち物などの
 * 選択肢をマスタから作る。種族の一覧が無いマスタ(オンライン)では masterSearch で都度引く。
 */
export interface TeamScreenProps {
  readonly teamClient: TeamClient;
  /**
   * 「この端末のデータを削除」の後に一覧を取り直す合図(P5-5d。ADR-0318 §6)。
   * マウント後に値が変わったら list() を呼び直す(最初の値ではマウント時の1回だけ)。
   */
  readonly reloadToken?: number;
  readonly master: MasterData;
  readonly masterSearch?: MasterSpeciesSearch;
}

/** list() 呼び出し1本の状態。読み込みに失敗しても新規作成のフォームは使える(ADR-0309 §4)。 */
type ListState =
  | { readonly status: "loading" }
  | { readonly status: "error"; readonly error: TeamError }
  | { readonly status: "loaded"; readonly teams: readonly Schemas["Team"][] };

/** 新規作成フォームの状態(notice = 送信前の検査で出す理由、error = create() が返した失敗)。 */
interface CreateFormState {
  readonly name: string;
  readonly submitting: boolean;
  readonly notice: string | null;
  readonly error: TeamError | null;
}

function initialCreateFormState(): CreateFormState {
  return { name: "", submitting: false, notice: null, error: null };
}

/**
 * 名前変更フォームの状態(同時に1件だけ開く。開いている構築の id を持つ)。
 * notice = 送信前の検査で出す理由、error = update() が返した失敗(CreateFormState と同じ形)。
 */
interface RenameState {
  readonly teamId: string;
  readonly name: string;
  readonly submitting: boolean;
  readonly notice: string | null;
  readonly error: TeamError | null;
}

/** 削除の確認の状態(2段階。確定するまで remove() を呼ばない。ADR-0309 §5)。 */
interface DeleteState {
  readonly teamId: string;
  readonly submitting: boolean;
  readonly error: TeamError | null;
}

/** create() が成功したら、応答の Team を一覧の先頭に足す(list を呼び直さない。ADR-0309 §4)。 */
function addCreatedTeam(list: ListState, team: Schemas["Team"]): ListState {
  if (list.status === "loaded") {
    return { status: "loaded", teams: [team, ...list.teams] };
  }
  // 一覧が読めていなくても作れる(ADR-0309 §4)。作れた分だけの一覧として出す。
  return { status: "loaded", teams: [team] };
}

/** update() が成功したら、その構築だけを応答の Team に差し替える(並び順は変えない)。 */
function replaceTeam(list: ListState, updated: Schemas["Team"]): ListState {
  if (list.status !== "loaded") {
    return list;
  }
  return { status: "loaded", teams: list.teams.map((team) => (team.id === updated.id ? updated : team)) };
}

/** remove() が成功したら、その構築を一覧から取り除く。 */
function removeTeamFromList(list: ListState, teamId: string): ListState {
  if (list.status !== "loaded") {
    return list;
  }
  return { status: "loaded", teams: list.teams.filter((team) => team.id !== teamId) };
}

/**
 * 構築ビルダーの画面(ADR-0309 §4)。
 */
export function TeamScreen({ teamClient, master, masterSearch, reloadToken }: TeamScreenProps): ReactNode {
  const [list, setList] = useState<ListState>({ status: "loading" });
  // create()/update()/remove() が一度でも成功したら true にする。list() は mount 時に1回しか呼ばないが、
  // その応答が書き込みの成功より後に届くと、古いスナップショットで手元の一覧を上書きしてしまう
  // (サーバーには存在するのに画面から消えて見える)。書き込み成功後に届いた list() 応答は捨てる。
  const hasWrittenRef = useRef(false);

  // マウント時と reloadToken が変わったときに list() を呼ぶ(cancelled フラグで古い応答を捨てる。
  // SpeedScreen.tsx と同じ形)。取り直しは「削除後の最新」なので、読み込み中に戻し、書き込み済みフラグも下ろす。
  const [prevReloadToken, setPrevReloadToken] = useState(reloadToken);
  if (reloadToken !== prevReloadToken) {
    setPrevReloadToken(reloadToken);
    setList({ status: "loading" });
  }
  useEffect(() => {
    let cancelled = false;
    hasWrittenRef.current = false;
    void teamClient.list().then((result) => {
      if (cancelled || hasWrittenRef.current) {
        return;
      }
      setList(
        result.ok ? { status: "loaded", teams: result.value } : { status: "error", error: result.error },
      );
    });
    return () => {
      cancelled = true;
    };
  }, [teamClient, reloadToken]);

  const [createState, setCreateState] = useState<CreateFormState>(initialCreateFormState);

  async function handleCreate(): Promise<void> {
    if (createState.submitting) {
      return;
    }
    const reason = teamNameNotice(createState.name);
    if (reason !== null) {
      setCreateState((current) => ({ ...current, notice: reason, error: null }));
      return;
    }
    const trimmed = createState.name.trim();
    setCreateState((current) => ({ ...current, submitting: true, notice: null, error: null }));
    const result = await teamClient.create({ name: trimmed, members: [] });
    if (result.ok) {
      hasWrittenRef.current = true;
      setCreateState(initialCreateFormState());
      setList((current) => addCreatedTeam(current, result.value));
    } else {
      setCreateState((current) => ({ ...current, submitting: false, error: result.error }));
    }
  }

  const [renameState, setRenameState] = useState<RenameState | null>(null);

  function openRename(team: Schemas["Team"]): void {
    setRenameState({ teamId: team.id, name: team.name, submitting: false, notice: null, error: null });
  }

  function cancelRename(): void {
    setRenameState(null);
  }

  async function saveRename(team: Schemas["Team"]): Promise<void> {
    if (renameState === null || renameState.teamId !== team.id || renameState.submitting || editorSaving) {
      return;
    }
    // 送信前の検査は新規作成と同じ範囲(契約と同じ。ADR-0309 §4)。範囲外は update() を呼ばずに理由を出す。
    const reason = teamNameNotice(renameState.name);
    if (reason !== null) {
      setRenameState((current) => (current === null ? current : { ...current, notice: reason, error: null }));
      return;
    }
    const trimmed = renameState.name.trim();
    setRenameState((current) =>
      current === null ? current : { ...current, submitting: true, notice: null, error: null },
    );
    const result = await teamClient.update(team.id, { name: trimmed, members: team.members });
    if (result.ok) {
      hasWrittenRef.current = true;
      setRenameState(null);
      setList((current) => replaceTeam(current, result.value));
    } else {
      setRenameState((current) =>
        current === null ? current : { ...current, submitting: false, error: result.error },
      );
    }
  }

  const [deleteState, setDeleteState] = useState<DeleteState | null>(null);

  function openDeleteConfirm(teamId: string): void {
    setDeleteState({ teamId, submitting: false, error: null });
  }

  function cancelDelete(): void {
    setDeleteState(null);
  }

  async function confirmDelete(teamId: string): Promise<void> {
    if (deleteState === null || deleteState.teamId !== teamId || deleteState.submitting) {
      return;
    }
    setDeleteState((current) => (current === null ? current : { ...current, submitting: true, error: null }));
    const result = await teamClient.remove(teamId);
    if (result.ok) {
      hasWrittenRef.current = true;
      setDeleteState(null);
      setList((current) => removeTeamFromList(current, teamId));
    } else {
      setDeleteState((current) =>
        current === null ? current : { ...current, submitting: false, error: result.error },
      );
    }
  }

  // メンバー編集の領域は同時に1つだけ(開いている構築の id)。開く・閉じるで API は呼ばない(ADR-0316 §1)。
  const [editingTeamId, setEditingTeamId] = useState<string | null>(null);
  // メンバー保存の送信中(名前変更と同時に送ると、全置換の update の一方が失われる)。
  const [editorSaving, setEditorSaving] = useState(false);

  return (
    <section aria-label={teamScreenText.regionLabel} className="team-screen">
      <div className="team-screen__create">
        <h2>{teamScreenText.createHeading}</h2>
        <div className="team-screen__field">
          <span>{teamScreenText.nameLabel}</span>
          <input
            type="text"
            aria-label={teamScreenText.nameLabel}
            value={createState.name}
            onChange={(event) => {
              const { value } = event.target;
              setCreateState((current) => ({ ...current, name: value }));
            }}
          />
        </div>
        <button
          type="button"
          disabled={createState.submitting}
          onClick={() => {
            void handleCreate();
          }}
        >
          {teamScreenText.createLabel}
        </button>
        {createState.notice !== null && <p className="team-screen__notice">{createState.notice}</p>}
        {createState.error !== null && (
          <div role="alert" className="team-screen__error">
            <p>{teamScreenText.createErrorHeading}</p>
            <p>{createState.error.message}</p>
          </div>
        )}
      </div>

      <TeamShowdownImport
        teamClient={teamClient}
        master={master}
        masterSearch={masterSearch}
        onCreated={(team) => {
          hasWrittenRef.current = true;
          setList((current) => addCreatedTeam(current, team));
        }}
      />

      <h2>{teamScreenText.listHeading}</h2>
      {list.status === "loading" && <p className="team-screen__notice">{teamScreenText.loadingNotice}</p>}
      {list.status === "error" && (
        <div role="alert" className="team-screen__error">
          <p>{teamScreenText.loadErrorHeading}</p>
          <p>{list.error.message}</p>
        </div>
      )}
      {list.status === "loaded" && list.teams.length === 0 && (
        <p className="team-screen__notice">{teamScreenText.emptyNotice}</p>
      )}
      {list.status === "loaded" && list.teams.length > 0 && (
        <ul aria-label={teamScreenText.listLabel} className="team-screen__list">
          {list.teams.map((team) => (
            <TeamRow
              key={team.id}
              team={team}
              renameState={renameState !== null && renameState.teamId === team.id ? renameState : null}
              deleteState={deleteState !== null && deleteState.teamId === team.id ? deleteState : null}
              editor={
                editingTeamId === team.id ? (
                  <TeamMemberEditor
                    team={team}
                    maxMembers={MAX_TEAM_MEMBERS}
                    master={master}
                    masterSearch={masterSearch}
                    teamClient={teamClient}
                    onSaved={(updated) => {
                      hasWrittenRef.current = true;
                      setList((current) => replaceTeam(current, updated));
                    }}
                    onClose={() => {
                      setEditingTeamId(null);
                    }}
                    onSavingChange={setEditorSaving}
                    locked={renameState !== null && renameState.teamId === team.id && renameState.submitting}
                  />
                ) : null
              }
              onOpenEditor={() => {
                setEditingTeamId(team.id);
              }}
              exporter={<TeamShowdownExport team={team} master={master} masterSearch={masterSearch} />}
              renameLocked={editorSaving}
              onOpenRename={() => {
                openRename(team);
              }}
              onChangeRenameName={(name) => {
                setRenameState((current) => (current === null ? current : { ...current, name }));
              }}
              onCancelRename={cancelRename}
              onSaveRename={() => {
                void saveRename(team);
              }}
              onOpenDelete={() => {
                openDeleteConfirm(team.id);
              }}
              onCancelDelete={cancelDelete}
              onConfirmDelete={() => {
                void confirmDelete(team.id);
              }}
            />
          ))}
        </ul>
      )}
    </section>
  );
}

interface TeamRowProps {
  readonly team: Schemas["Team"];
  readonly renameState: RenameState | null;
  readonly deleteState: DeleteState | null;
  /** メンバー編集の領域(開いているときだけ。null は閉じている)。 */
  readonly editor: ReactNode;
  readonly onOpenEditor: () => void;
  /** Showdown 形式の書き出し(ボタンと、開いたときの領域)。 */
  readonly exporter: ReactNode;
  /** メンバー保存の送信中(true の間は名前変更を保存できない)。 */
  readonly renameLocked: boolean;
  readonly onOpenRename: () => void;
  readonly onChangeRenameName: (name: string) => void;
  readonly onCancelRename: () => void;
  readonly onSaveRename: () => void;
  readonly onOpenDelete: () => void;
  readonly onCancelDelete: () => void;
  readonly onConfirmDelete: () => void;
}

/** 構築1件の行(名前・メンバー数・最終更新、名前変更・削除)。 */
function TeamRow({
  team,
  renameState,
  deleteState,
  editor,
  onOpenEditor,
  exporter,
  renameLocked,
  onOpenRename,
  onChangeRenameName,
  onCancelRename,
  onSaveRename,
  onOpenDelete,
  onCancelDelete,
  onConfirmDelete,
}: TeamRowProps) {
  return (
    <li className="team-screen__item">
      <span className="team-screen__item-name">{team.name}</span>
      <span className="team-screen__item-meta">
        {teamScreenText.memberCountLabel(team.members.length, MAX_TEAM_MEMBERS)}
      </span>
      <span className="team-screen__item-meta">
        {teamScreenText.updatedAtLabel(team.updatedAt.slice(0, 10))}
      </span>

      {renameState === null ? (
        <button type="button" onClick={onOpenRename}>
          {teamScreenText.renameLabel(team.name)}
        </button>
      ) : (
        <div className="team-screen__rename">
          <div className="team-screen__field">
            <span>{teamScreenText.renameFieldLabel(team.name)}</span>
            <input
              type="text"
              aria-label={teamScreenText.renameFieldLabel(team.name)}
              value={renameState.name}
              disabled={renameState.submitting}
              onChange={(event) => {
                onChangeRenameName(event.target.value);
              }}
            />
          </div>
          <button type="button" disabled={renameState.submitting || renameLocked} onClick={onSaveRename}>
            {teamScreenText.renameSaveLabel}
          </button>
          <button type="button" disabled={renameState.submitting} onClick={onCancelRename}>
            {teamScreenText.renameCancelLabel}
          </button>
          {renameState.notice !== null && <p className="team-screen__notice">{renameState.notice}</p>}
          {renameState.error !== null && (
            <div role="alert" className="team-screen__error">
              <p>{teamScreenText.renameErrorHeading}</p>
              <p>{renameState.error.message}</p>
            </div>
          )}
        </div>
      )}

      {editor === null && (
        <button type="button" onClick={onOpenEditor}>
          {teamMemberText.editLabel(team.name)}
        </button>
      )}

      {exporter}

      {deleteState === null ? (
        <button type="button" onClick={onOpenDelete}>
          {teamScreenText.deleteLabel(team.name)}
        </button>
      ) : (
        <div className="team-screen__delete-confirm">
          <p>{teamScreenText.deleteConfirmNotice(team.name)}</p>
          <button type="button" disabled={deleteState.submitting} onClick={onConfirmDelete}>
            {teamScreenText.deleteConfirmLabel(team.name)}
          </button>
          <button type="button" disabled={deleteState.submitting} onClick={onCancelDelete}>
            {teamScreenText.deleteCancelLabel(team.name)}
          </button>
          {deleteState.error !== null && (
            <div role="alert" className="team-screen__error">
              <p>{teamScreenText.deleteErrorHeading}</p>
              <p>{deleteState.error.message}</p>
            </div>
          )}
        </div>
      )}

      {editor}
    </li>
  );
}
