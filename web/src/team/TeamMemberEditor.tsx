// F-08(ADR-0332 §2): 構築1件の編集画面。常に6つの枠(TEAM_SLOT_COUNT)を持ち、下書き(MemberDraft)を枠ごとに持つ。
// [保存] でだけ、種族の決まった枠を枠の順に詰めて update(全置換)する。応答を待ってから親の一覧を書き換える
// (楽観更新しない。失敗時は下書きを残す)。画面は TeamScreen が開いている間だけ mount される(閉じると下書きは捨てる)。

import { useEffect, useMemo, useRef, useState, type ReactNode } from "react";
import type { components } from "../api/openapi.gen";
import { megaStoneItemIds } from "../domain/mega";
import { megaItemText, teamMemberText } from "../i18n/ja";
import { teamShowdownText } from "../i18n/team";
import { itemRoleText } from "../i18n/items";
import { megaStoneLabel } from "../domain/itemRoles";
import { masterCapabilities } from "../master/capabilities";
import type {
  MasterData,
  MasterSpecies,
  MasterSpeciesResolution,
  MasterSpeciesSearch,
} from "../master/types";
import { selectableAbilities } from "../domain/requests";
import { useSpeciesResolutions } from "../screens/speciesResolution";
import "./TeamMemberEditor.css";
import { TeamMemberFields } from "./TeamMemberFields";
import type { TeamClient, TeamError } from "./teamClient";
import { changeSpecies, correctMegaItem, type MegaItemCorrection, type MemberDraft } from "./teamMember";
import { defaultNatureId } from "./teamMemberOptions";
import {
  TEAM_SLOT_COUNT,
  clearSlot,
  hasUnsavedChanges,
  isEmptySlot,
  slotsFromMembers,
  slotsToMembers,
  swapSlots,
  teamInputFromMembers,
} from "./teamSlots";
import { TeamShowdownExport } from "./TeamShowdownExport";

type Schemas = components["schemas"];

export interface TeamMemberEditorProps {
  readonly team: Schemas["Team"];
  /** 画面に出す構築の名前(team/teamName.ts の teamDisplayNames)。 */
  readonly displayName: string;
  readonly master: MasterData;
  readonly masterSearch: MasterSpeciesSearch | undefined;
  readonly teamClient: TeamClient;
  /** update が成功したときに、応答の Team で親の一覧を書き換える。 */
  readonly onSaved: (team: Schemas["Team"]) => void;
  /** 一覧に戻る(未保存の変更があるときは、確認の後に呼ばれる)。 */
  readonly onClose: () => void;
}

/** 枠1つ(id は入れ替え・外すでも枠の中身に付いて動く key)。 */
interface Entry {
  readonly id: number;
  readonly draft: MemberDraft;
  /** 古い保存データを開いたときに直した内容(issue 515。種族を変えると消える。未保存の変更)。 */
  readonly correction: MegaItemCorrection | null;
}

interface EditorState {
  readonly entries: readonly Entry[];
  readonly nextId: number;
}

/**
 * 開いたときの状態。種族の一覧があるマスタでは、メガ種族の古い保存データ(別の持ち物)をここで1回だけ直す。
 * 一覧が無いマスタ(オンライン)は種族の解決後に直す(TeamMemberEditor の effect)。
 */
function initialState(members: readonly Schemas["TeamMember"][], master: MasterData): EditorState {
  const hasSpeciesList = masterCapabilities(master).speciesList;
  return {
    entries: slotsFromMembers(members, defaultNatureId(master)).map((draft, id) => {
      if (!hasSpeciesList || isEmptySlot(draft)) {
        return { id, draft, correction: null };
      }
      const species = master.species.find((candidate) => candidate.key === draft.speciesKey) ?? null;
      return { id, ...correctMegaItem(draft, species, master.items) };
    }),
    nextId: TEAM_SLOT_COUNT,
  };
}

function correctionNoticeText(
  correction: MegaItemCorrection | null,
  species: MasterSpecies | null,
): string | null {
  if (correction === null) {
    return null;
  }
  return correction.kind === "fixed"
    ? megaItemText.correctedNotice(
        species === null ? itemRoleText.megaStoneUnnamed : megaStoneLabel(species, correction.item.nameJa),
      )
    : megaItemText.clearedNotice;
}

export function TeamMemberEditor({
  team,
  displayName,
  master,
  masterSearch,
  teamClient,
  onSaved,
  onClose,
}: TeamMemberEditorProps): ReactNode {
  const [state, setState] = useState<EditorState>(() => initialState(team.members, master));
  const [saving, setSaving] = useState(false);
  const [saved, setSaved] = useState(false);
  const [error, setError] = useState<TeamError | null>(null);
  const [confirmingLeave, setConfirmingLeave] = useState(false);
  const [failedKeys, setFailedKeys] = useState<ReadonlySet<string>>(new Set());
  const resolutions = useSpeciesResolutions();
  const { register } = resolutions;
  const stoneIds = useMemo(
    () => megaStoneItemIds([...master.species, ...resolutions.resolvedSpecies]),
    [master.species, resolutions.resolvedSpecies],
  );
  const editorRef = useRef<HTMLElement>(null);
  const headingRef = useRef<HTMLHeadingElement>(null);
  const backButtonRef = useRef<HTMLButtonElement>(null);

  // 編集画面を開いたら見出しへフォーカスを移す(一覧のボタンが消えてもフォーカスを失わない。ADR-0332 §7)。
  useEffect(() => {
    headingRef.current?.focus();
  }, []);
  // 外した枠の「ポケモン」欄へフォーカスを移す合図(枠の位置と回数。0 回の間は動かさない)。
  const [focusRequest, setFocusRequest] = useState<{ readonly index: number; readonly count: number }>({
    index: 0,
    count: 0,
  });

  // 種族の一覧が無いマスタ(オンライン): 保存済みメンバーの種族を開いたときに1体ずつ解決する(特性・技の実体を得る)。
  // 解決に失敗したメンバーは内容を書き換えず、alert だけ出す(ADR-0316 §7)。
  useEffect(() => {
    if (masterCapabilities(master).speciesList || masterSearch === undefined) {
      return;
    }
    let cancelled = false;
    // 開いた時点のメンバー(id は初期の枠の位置)。解決した種族に合わせて、古い保存データの持ち物を1回だけ直す。
    const openedCount = team.members.length;
    for (const key of new Set(team.members.map((member) => member.speciesKey))) {
      masterSearch.resolveSpecies(key).then(
        (resolution) => {
          if (!cancelled) {
            register(resolution);
            setState((current) => ({
              ...current,
              entries: current.entries.map((entry) => {
                if (entry.id >= openedCount || entry.draft.speciesKey !== key) {
                  return entry;
                }
                return { id: entry.id, ...correctMegaItem(entry.draft, resolution.species, master.items) };
              }),
            }));
            setSaved(false);
          }
        },
        () => {
          if (!cancelled) {
            setFailedKeys((current) => new Set(current).add(key));
          }
        },
      );
    }
    return () => {
      cancelled = true;
    };
    // 開いた時点のメンバーだけを解決する(下書きの編集では引き直さない)。
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // 枠を外した後は、その枠の先頭の入力欄(空の枠では「ポケモン」欄)へフォーカスを移す(消えた要素に残さない)。
  useEffect(() => {
    if (focusRequest.count === 0) {
      return;
    }
    const slot = editorRef.current?.querySelectorAll<HTMLElement>("fieldset.team-member")[focusRequest.index];
    slot?.querySelector<HTMLElement>("select, input")?.focus();
  }, [focusRequest]);

  function edit(update: (entries: readonly Entry[]) => readonly Entry[]): void {
    setState((current) => ({ ...current, entries: update(current.entries) }));
    setSaved(false);
  }

  function updateDraft(id: number, next: MemberDraft): void {
    edit((entries) =>
      entries.map((entry) =>
        entry.id === id
          ? {
              ...entry,
              draft: next,
              correction: next.speciesKey === entry.draft.speciesKey ? entry.correction : null,
            }
          : entry,
      ),
    );
  }

  function handleResolved(id: number, resolution: MasterSpeciesResolution): void {
    register(resolution);
    edit((entries) =>
      entries.map((entry) =>
        entry.id === id
          ? {
              ...entry,
              draft: changeSpecies(
                entry.draft,
                resolution.species,
                selectableAbilities(resolution.species, resolution.abilities),
                {
                  previous:
                    entry.draft.speciesKey === null
                      ? null
                      : resolutions.speciesFor(master.species, entry.draft.speciesKey),
                  items: master.items,
                },
              ),
              correction: null,
            }
          : entry,
      ),
    );
  }

  /** 枠を外す: その枠だけ空に戻す(他の枠は動かさない)。 */
  function removeMember(index: number): void {
    setState((current) => ({
      entries: replaceWithBlank(current.entries, index, current.nextId, master),
      nextId: current.nextId + 1,
    }));
    setSaved(false);
    setFocusRequest((current) => ({ index, count: current.count + 1 }));
  }

  function moveMember(index: number, delta: -1 | 1): void {
    edit((entries) => swapSlots(entries, index, delta));
  }

  const drafts = state.entries.map((entry) => entry.draft);
  const result = slotsToMembers(drafts);
  const unsaved = hasUnsavedChanges(team.members, drafts);
  const canSave = !saving && result.ok;

  async function save(): Promise<void> {
    if (saving || !result.ok) {
      return;
    }
    setSaving(true);
    setSaved(false);
    setError(null);
    const response = await teamClient.update(team.id, teamInputFromMembers(result.members));
    if (response.ok) {
      onSaved(response.value);
      setSaved(true);
      setConfirmingLeave(false);
    } else {
      setError(response.error);
    }
    setSaving(false);
  }

  function handleBack(): void {
    if (unsaved) {
      setConfirmingLeave(true);
    } else {
      onClose();
    }
  }

  return (
    <section
      ref={editorRef}
      aria-label={teamMemberText.editorLabel(displayName)}
      className="team-member-editor"
    >
      <div className="team-member-editor__header">
        <h2 ref={headingRef} tabIndex={-1}>
          {displayName}
        </h2>
        <button
          ref={backButtonRef}
          type="button"
          className="ui-button ui-button--secondary"
          disabled={saving}
          onClick={handleBack}
        >
          {teamMemberText.closeLabel}
        </button>
      </div>
      {confirmingLeave && unsaved && (
        <div className="ui-notice team-member-editor__leave">
          <p>{teamMemberText.leaveConfirmNotice}</p>
          <div className="team-member-editor__actions">
            <button type="button" className="ui-button ui-button--danger" disabled={saving} onClick={onClose}>
              {teamMemberText.leaveDiscardLabel}
            </button>
            <button
              type="button"
              className="ui-button ui-button--secondary"
              onClick={() => {
                setConfirmingLeave(false);
                backButtonRef.current?.focus();
              }}
            >
              {teamMemberText.leaveCancelLabel}
            </button>
          </div>
        </div>
      )}

      {state.entries.map((entry, index) => {
        const key = entry.draft.speciesKey;
        const species = key === null ? null : resolutions.speciesFor(master.species, key);
        return (
          <TeamMemberFields
            key={entry.id}
            position={index + 1}
            count={state.entries.length}
            draft={entry.draft}
            master={master}
            masterSearch={masterSearch}
            species={species}
            abilityPool={key === null ? master.abilities : resolutions.abilitiesFor(master.abilities, key)}
            movePool={key === null ? master.moves : resolutions.movesFor(master.moves, key)}
            resolveFailed={key !== null && species === null && failedKeys.has(key)}
            stoneIds={stoneIds}
            correctionNotice={correctionNoticeText(entry.correction, species)}
            onChange={(next) => {
              updateDraft(entry.id, next);
            }}
            onResolved={(resolution) => {
              handleResolved(entry.id, resolution);
            }}
            onRemove={() => {
              removeMember(index);
            }}
            onMove={(delta) => {
              moveMember(index, delta);
            }}
          />
        );
      })}

      <div className="team-member-editor__actions">
        <button
          type="button"
          className="ui-button ui-button--primary"
          disabled={!canSave}
          onClick={() => {
            void save();
          }}
        >
          {teamMemberText.saveLabel}
        </button>
        {unsaved && <span className="team-member-editor__unsaved">{teamMemberText.unsavedNotice}</span>}
      </div>

      {saved && (
        <p role="status" className="team-member-editor__notice">
          {teamMemberText.savedNotice}
        </p>
      )}
      {error !== null && (
        <div role="alert" className="ui-notice ui-notice--error team-member-editor__error">
          <p>{teamMemberText.saveErrorHeading}</p>
          <p>{error.message}</p>
        </div>
      )}

      <details className="ui-card team-fold">
        <summary>{teamShowdownText.exportFoldLabel}</summary>
        <p className="team-fold__help">{teamShowdownText.exportHelp}</p>
        <TeamShowdownExport team={team} name={displayName} master={master} masterSearch={masterSearch} />
      </details>
    </section>
  );
}

/** index の枠を新しい id の空の枠に置き換える(外した枠の入力欄を作り直し、他の枠は動かさない)。 */
function replaceWithBlank(
  entries: readonly Entry[],
  index: number,
  id: number,
  master: MasterData,
): readonly Entry[] {
  const cleared = clearSlot(
    entries.map((entry) => entry.draft),
    index,
    defaultNatureId(master),
  );
  return entries.map((entry, position) =>
    position === index ? { id, draft: cleared[index] ?? entry.draft, correction: null } : entry,
  );
}
