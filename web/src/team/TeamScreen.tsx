// 構築ビルダーの画面(ADR-0309 §4・ADR-0332)。一覧と編集の2つの表示を切り替える(URL は /team のまま)。
// 一覧: 構築のカード・[新しい構築]。編集: 6つの枠(TeamMemberEditor.tsx)。構築名は無い。
//
// 状態の作り(SpeedScreen.tsx・BalanceScreen.tsx と同じ考え方):
//   - list() はマウント時に1回だけ呼び、cancelled フラグで古い応答を捨てる
//   - create()/update()/remove() が成功したら、応答の Team で手元の一覧を書き換える(list を呼び直さない)
//   - 一覧の読み込みに失敗しても、[新しい構築]は先に使える(ADR-0309 §4)
//   - 編集中の下書き・開いている構築はこの画面の state に持つ(ADR-0308)

import { useEffect, useRef, useState, type ReactNode } from "react";
import type { components } from "../api/openapi.gen";
import { teamMemberText, teamScreenText } from "../i18n/ja";
import { PokemonImage } from "../images/PokemonImage";
import type { MasterData, MasterSpeciesSearch } from "../master/types";
import { Icon } from "../ui/Icon";
import { typeAccentStyle } from "../ui/typeAccent";
import "./TeamScreen.css";
import { TeamMemberEditor } from "./TeamMemberEditor";
import type { TeamClient, TeamError } from "./teamClient";
import { teamDisplayNames } from "./teamName";
import { TEAM_SLOT_COUNT, teamInputFromMembers } from "./teamSlots";

type Schemas = components["schemas"];

/** パーティの上限(api/openapi.yaml の TeamInput.members の maxItems)。 */
export const MAX_TEAM_MEMBERS = TEAM_SLOT_COUNT;

/**
 * 画面の props(App.tsx が app/screens.tsx 経由で注入する。ADR-0309 §2)。
 * 一覧のアイコン列の種族の引き当てと、メンバー編集(ADR-0316)の種族・技・持ち物などの選択肢にマスタを使う。
 * 種族の一覧が無いマスタ(オンライン)では、編集画面が masterSearch で都度引く。
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

/** list() 呼び出し1本の状態。読み込みに失敗しても[新しい構築]は使える(ADR-0309 §4)。 */
type ListState =
  | { readonly status: "loading" }
  | { readonly status: "error"; readonly error: TeamError }
  | { readonly status: "loaded"; readonly teams: readonly Schemas["Team"][] };

/** [新しい構築]の状態(error = create() が返した失敗)。 */
interface CreateState {
  readonly submitting: boolean;
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
 * 構築ビルダーの画面(ADR-0309 §4・ADR-0332)。
 */
export function TeamScreen({ teamClient, master, masterSearch, reloadToken }: TeamScreenProps): ReactNode {
  const [list, setList] = useState<ListState>({ status: "loading" });
  // create()/update()/remove() が一度でも成功したら true にする。list() は mount 時に1回しか呼ばないが、
  // その応答が書き込みの成功より後に届くと、古いスナップショットで手元の一覧を上書きしてしまう
  // (サーバーには存在するのに画面から消えて見える)。書き込み成功後に届いた list() 応答は捨てる。
  const hasWrittenRef = useRef(false);
  // 編集画面で開いている構築の id(null は一覧の表示)。開く・戻るで API は呼ばない。
  const [editingTeamId, setEditingTeamId] = useState<string | null>(null);
  const [deleteState, setDeleteState] = useState<DeleteState | null>(null);
  // 編集画面から一覧に戻ったとき、フォーカスを戻す構築の id(ADR-0332 §7)。
  const returnFocusRef = useRef<string | null>(null);
  const screenRef = useRef<HTMLElement>(null);
  useEffect(() => {
    const teamId = returnFocusRef.current;
    if (editingTeamId !== null || teamId === null) {
      return;
    }
    returnFocusRef.current = null;
    const section = screenRef.current;
    const target =
      section?.querySelector<HTMLElement>(`[data-open-team="${teamId}"]`) ??
      section?.querySelector<HTMLElement>("[data-create-team]");
    target?.focus();
  }, [editingTeamId]);

  // マウント時と reloadToken が変わったときに list() を呼ぶ(cancelled フラグで古い応答を捨てる。
  // SpeedScreen.tsx と同じ形)。取り直しは「削除後の最新」なので、読み込み中に戻し、編集画面も閉じ、書き込み済みフラグも下ろす。
  const [prevReloadToken, setPrevReloadToken] = useState(reloadToken);
  if (reloadToken !== prevReloadToken) {
    setPrevReloadToken(reloadToken);
    setList({ status: "loading" });
    setEditingTeamId(null);
    setDeleteState(null);
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

  const [createState, setCreateState] = useState<CreateState>({ submitting: false, error: null });

  // 空の構築を作り、成功したらすぐ6枠の編集画面を開く。name は送らない(サーバーが既定名を入れる。ADR-0229)。
  async function handleCreate(): Promise<void> {
    if (createState.submitting) {
      return;
    }
    setCreateState({ submitting: true, error: null });
    const result = await teamClient.create(teamInputFromMembers([]));
    if (result.ok) {
      hasWrittenRef.current = true;
      setCreateState({ submitting: false, error: null });
      setList((current) => addCreatedTeam(current, result.value));
      setEditingTeamId(result.value.id);
    } else {
      setCreateState({ submitting: false, error: result.error });
    }
  }

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

  const teams = list.status === "loaded" ? list.teams : [];
  const displayNames = teamDisplayNames(teams);
  const editingTeam = editingTeamId === null ? undefined : teams.find((team) => team.id === editingTeamId);

  if (editingTeam !== undefined) {
    return (
      <section ref={screenRef} aria-label={teamScreenText.regionLabel} className="team-screen">
        <TeamMemberEditor
          key={editingTeam.id}
          team={editingTeam}
          displayName={displayNames.get(editingTeam.id) ?? editingTeam.name}
          master={master}
          masterSearch={masterSearch}
          teamClient={teamClient}
          onSaved={(updated) => {
            hasWrittenRef.current = true;
            setList((current) => replaceTeam(current, updated));
          }}
          onClose={() => {
            returnFocusRef.current = editingTeam.id;
            setEditingTeamId(null);
          }}
        />
      </section>
    );
  }

  return (
    <section ref={screenRef} aria-label={teamScreenText.regionLabel} className="team-screen">
      <div className="ui-card team-screen__create">
        <button
          data-create-team=""
          type="button"
          className="ui-button ui-button--primary"
          disabled={createState.submitting}
          onClick={() => {
            void handleCreate();
          }}
        >
          <Icon name="plus" size={20} />
          {teamScreenText.createLabel}
        </button>
        {createState.error !== null && (
          <div role="alert" className="ui-notice ui-notice--error team-screen__error">
            <p>{teamScreenText.createErrorHeading}</p>
            <p>{createState.error.message}</p>
          </div>
        )}
      </div>

      <h2>{teamScreenText.listHeading}</h2>
      {list.status === "loading" && (
        <p className="ui-notice ui-notice--loading team-screen__notice">{teamScreenText.loadingNotice}</p>
      )}
      {list.status === "error" && (
        <div role="alert" className="ui-notice ui-notice--error team-screen__error">
          <p>{teamScreenText.loadErrorHeading}</p>
          <p>{list.error.message}</p>
        </div>
      )}
      {list.status === "loaded" && list.teams.length === 0 && (
        <p className="ui-notice ui-notice--empty team-screen__notice team-screen__empty">
          <Icon name="team" size={48} />
          {teamScreenText.emptyNotice}
        </p>
      )}
      {list.status === "loaded" && list.teams.length > 0 && (
        <ul aria-label={teamScreenText.listLabel} className="team-screen__list">
          {list.teams.map((team) => {
            const displayName = displayNames.get(team.id) ?? team.name;
            return (
              <TeamCard
                key={team.id}
                team={team}
                displayName={displayName}
                master={master}
                deleteState={deleteState !== null && deleteState.teamId === team.id ? deleteState : null}
                onOpen={() => {
                  setEditingTeamId(team.id);
                }}
                onOpenDelete={() => {
                  openDeleteConfirm(team.id);
                }}
                onCancelDelete={cancelDelete}
                onConfirmDelete={() => {
                  void confirmDelete(team.id);
                }}
              />
            );
          })}
        </ul>
      )}
    </section>
  );
}

interface TeamCardProps {
  readonly team: Schemas["Team"];
  readonly displayName: string;
  readonly master: MasterData;
  readonly deleteState: DeleteState | null;
  readonly onOpen: () => void;
  readonly onOpenDelete: () => void;
  readonly onCancelDelete: () => void;
  readonly onConfirmDelete: () => void;
}

/** 構築1件のカード(名前・メンバーのアイコン列・n/6体・最終更新、[開く]・[削除])。 */
function TeamCard({
  team,
  displayName,
  master,
  deleteState,
  onOpen,
  onOpenDelete,
  onCancelDelete,
  onConfirmDelete,
}: TeamCardProps) {
  return (
    <li className="ui-card team-card">
      <h3 className="team-card__name">{displayName}</h3>
      <div
        role="group"
        aria-label={teamScreenText.memberIconsLabel(displayName)}
        className="team-card__icons"
      >
        {team.members.map((member, index) => {
          // 一覧のためにオンラインの種族解決 API は呼ばない(構築の数×6回の通信を避ける。ADR-0332 §4)。
          const species = master.species.find((candidate) => candidate.key === member.speciesKey);
          return (
            <span
              key={index}
              role="img"
              aria-label={species?.nameJa ?? teamScreenText.unknownMemberIcon(index + 1)}
              className="team-card__icon team-card__slot"
            >
              <PokemonImage
                speciesKey={member.speciesKey}
                size="thumb"
                className="team-card__image"
                fallback={
                  <span
                    className="team-card__emblem"
                    data-testid="type-emblem"
                    style={typeAccentStyle(species?.types[0])}
                  />
                }
              />
            </span>
          );
        })}
        {Array.from({ length: Math.max(0, MAX_TEAM_MEMBERS - team.members.length) }, (_, index) => (
          <span
            key={`empty-${String(index)}`}
            aria-hidden="true"
            className="team-card__slot team-card__slot--empty"
          />
        ))}
      </div>
      <span className="team-card__meta">
        {teamScreenText.memberCountLabel(team.members.length, MAX_TEAM_MEMBERS)}
      </span>
      <span className="team-card__meta">{teamScreenText.updatedAtLabel(team.updatedAt.slice(0, 10))}</span>

      <div className="team-card__actions">
        <button
          type="button"
          className="ui-button ui-button--primary"
          aria-label={teamMemberText.editLabel(displayName)}
          data-open-team={team.id}
          onClick={onOpen}
        >
          <Icon name="edit" size={20} />
          {teamScreenText.openLabel}
        </button>
        {deleteState === null && (
          <button
            type="button"
            className="ui-button ui-button--danger team-screen__icon-button"
            aria-label={teamScreenText.deleteLabel(displayName)}
            onClick={onOpenDelete}
          >
            <Icon name="trash" size={20} />
          </button>
        )}
      </div>

      {deleteState !== null && (
        <div className="team-screen__delete-confirm">
          <p>{teamScreenText.deleteConfirmNotice(displayName)}</p>
          <button
            type="button"
            className="ui-button ui-button--danger"
            disabled={deleteState.submitting}
            onClick={onConfirmDelete}
          >
            {teamScreenText.deleteConfirmLabel(displayName)}
          </button>
          <button
            type="button"
            className="ui-button ui-button--secondary"
            disabled={deleteState.submitting}
            onClick={onCancelDelete}
          >
            {teamScreenText.deleteCancelLabel(displayName)}
          </button>
          {deleteState.error !== null && (
            <div role="alert" className="ui-notice ui-notice--error team-screen__error">
              <p>{teamScreenText.deleteErrorHeading}</p>
              <p>{deleteState.error.message}</p>
            </div>
          )}
        </div>
      )}
    </li>
  );
}
