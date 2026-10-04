// P5-5b PR-A2(ADR-0316): 構築1件のメンバー編集領域。下書き(MemberDraft)を持ち、[メンバーを保存] でだけ
// パーティ全体を update(全置換)する。応答を待ってから親の一覧を書き換える(楽観更新しない。失敗時は下書きを残す)。
// 領域は開いている間だけ mount される(閉じると未保存の下書きは捨てる)。

import { useEffect, useMemo, useRef, useState, type ReactNode } from "react";
import type { components } from "../api/openapi.gen";
import { megaStoneItemIds } from "../domain/mega";
import { megaItemText, teamMemberText } from "../i18n/ja";
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
import {
  changeSpecies,
  blankDraft,
  correctMegaItem,
  draftToMember,
  memberToDraft,
  type MegaItemCorrection,
  type MemberDraft,
} from "./teamMember";
import { defaultNatureId } from "./teamMemberOptions";

type Schemas = components["schemas"];

export interface TeamMemberEditorProps {
  readonly team: Schemas["Team"];
  /** パーティの上限(TeamScreen.tsx の MAX_TEAM_MEMBERS)。 */
  readonly maxMembers: number;
  readonly master: MasterData;
  readonly masterSearch: MasterSpeciesSearch | undefined;
  readonly teamClient: TeamClient;
  /** update が成功したときに、応答の Team で親の一覧を書き換える。 */
  readonly onSaved: (team: Schemas["Team"]) => void;
  readonly onClose: () => void;
  /** 保存の送信中かどうかを親に知らせる(送信中は名前変更を無効にし、同時送信で一方が失われるのを防ぐ)。 */
  readonly onSavingChange: (saving: boolean) => void;
  /** 名前変更の送信中(true の間は保存できない。update は全置換なので同時に送ると一方が失われる)。 */
  readonly locked: boolean;
}

/** 下書き1体(id は並べ替え・削除でも変わらない key)。 */
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
    entries: members.map((member, id) => {
      const draft = memberToDraft(member);
      if (!hasSpeciesList) {
        return { id, draft, correction: null };
      }
      const species = master.species.find((candidate) => candidate.key === member.speciesKey) ?? null;
      return { id, ...correctMegaItem(draft, species, master.items) };
    }),
    nextId: members.length,
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
  maxMembers,
  master,
  masterSearch,
  teamClient,
  onSaved,
  onClose,
  onSavingChange,
  locked,
}: TeamMemberEditorProps): ReactNode {
  const [state, setState] = useState<EditorState>(() => initialState(team.members, master));
  const [saving, setSaving] = useState(false);
  const [saved, setSaved] = useState(false);
  const [error, setError] = useState<TeamError | null>(null);
  const [failedKeys, setFailedKeys] = useState<ReadonlySet<string>>(new Set());
  const resolutions = useSpeciesResolutions();
  const { register } = resolutions;
  const stoneIds = useMemo(
    () => megaStoneItemIds([...master.species, ...resolutions.resolvedSpecies]),
    [master.species, resolutions.resolvedSpecies],
  );
  const addButtonRef = useRef<HTMLButtonElement>(null);
  const [focusAddToken, setFocusAddToken] = useState(0);

  // 種族の一覧が無いマスタ(オンライン): 保存済みメンバーの種族を開いたときに1体ずつ解決する(特性・技の実体を得る)。
  // 解決に失敗したメンバーは内容を書き換えず、alert だけ出す(ADR-0316 §7)。
  useEffect(() => {
    if (masterCapabilities(master).speciesList || masterSearch === undefined) {
      return;
    }
    let cancelled = false;
    // 開いた時点のメンバー(id は初期の並び順)。解決した種族に合わせて、古い保存データの持ち物を1回だけ直す。
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

  // 削除後のフォーカスは [メンバーを追加] に移す(消えた要素にフォーカスが残らないように)。
  useEffect(() => {
    if (focusAddToken > 0) {
      addButtonRef.current?.focus();
    }
  }, [focusAddToken]);

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

  function addMember(): void {
    if (state.entries.length >= maxMembers) {
      return;
    }
    setState((current) => ({
      entries: [
        ...current.entries,
        { id: current.nextId, draft: blankDraft(defaultNatureId(master)), correction: null },
      ],
      nextId: current.nextId + 1,
    }));
    setSaved(false);
  }

  function removeMember(id: number): void {
    edit((entries) => entries.filter((entry) => entry.id !== id));
    setFocusAddToken((token) => token + 1);
  }

  function moveMember(index: number, delta: -1 | 1): void {
    edit((entries) => {
      const target = index + delta;
      const moved = entries[index];
      const other = entries[target];
      if (moved === undefined || other === undefined) {
        return entries;
      }
      return entries.map((entry, position) =>
        position === index ? other : position === target ? moved : entry,
      );
    });
  }

  const results = state.entries.map((entry) => draftToMember(entry.draft));
  const canSave = !saving && !locked && results.every((result) => result.ok);

  async function save(): Promise<void> {
    if (saving || locked) {
      return;
    }
    const members: Schemas["TeamMember"][] = [];
    for (const result of results) {
      if (!result.ok) {
        return;
      }
      members.push(result.member);
    }
    setSaving(true);
    onSavingChange(true);
    setSaved(false);
    setError(null);
    const response = await teamClient.update(team.id, { name: team.name, members });
    if (response.ok) {
      onSaved(response.value);
      setSaved(true);
    } else {
      setError(response.error);
    }
    setSaving(false);
    onSavingChange(false);
  }

  const atLimit = state.entries.length >= maxMembers;

  return (
    <section aria-label={teamMemberText.editorLabel(team.name)} className="team-member-editor">
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
              removeMember(entry.id);
            }}
            onMove={(delta) => {
              moveMember(index, delta);
            }}
          />
        );
      })}

      <div className="team-member-editor__actions">
        <button type="button" ref={addButtonRef} disabled={atLimit} onClick={addMember}>
          {teamMemberText.addLabel}
        </button>
        {atLimit && (
          <p className="team-member-editor__notice">{teamMemberText.addDisabledNotice(maxMembers)}</p>
        )}
        <button
          type="button"
          disabled={!canSave}
          onClick={() => {
            void save();
          }}
        >
          {teamMemberText.saveLabel}
        </button>
        <button type="button" disabled={saving} onClick={onClose}>
          {teamMemberText.closeLabel}
        </button>
      </div>

      {saved && (
        <p role="status" className="team-member-editor__notice">
          {teamMemberText.savedNotice}
        </p>
      )}
      {error !== null && (
        <div role="alert" className="team-member-editor__error">
          <p>{teamMemberText.saveErrorHeading}</p>
          <p>{error.message}</p>
        </div>
      )}
    </section>
  );
}
