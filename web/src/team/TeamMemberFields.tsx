// P5-5b PR-A2(ADR-0316 §4・§5・§7・§8・§10): メンバー1体の入力欄(種族・技4枠・持ち物・特性・性格・テラスタイプ・SP)。
// 状態は持たない(下書きは TeamMemberEditor が持つ)。選択肢はマスタ(と、オンラインでは検索で解決した実体)から作る。

import { useId, type ReactNode } from "react";
import { isMegaStoneItem, itemsForRole, megaStoneLabel, TEAM_ITEM_ROLE_FILTER } from "../domain/itemRoles";
import { itemRoleText } from "../i18n/items";
import { megaItemLock } from "../domain/mega";
import { MAX_SP_PER_STAT, MAX_SP_TOTAL, selectableAbilities } from "../domain/requests";
import type { Ability, Item, Move, StatKey } from "../engine/types";
import { isTypeId, statLetterJa, teamMemberText, typeNameJa } from "../i18n/ja";
import { masterCapabilities } from "../master/capabilities";
import type {
  MasterData,
  MasterItem,
  MasterSpecies,
  MasterSpeciesResolution,
  MasterSpeciesSearch,
} from "../master/types";
import { AbilitySelect } from "../screens/AbilitySelect";
import { MegaItemReason } from "../screens/MegaItemReason";
import { SpeciesSearchField } from "../screens/SpeciesSearchField";
import {
  MAX_MEMBER_MOVES,
  SP_STATS,
  changeSpecies,
  checkSpDraft,
  draftToMember,
  type MemberDraft,
  type MemberIssue,
} from "./teamMember";
import { abilityOptions, itemOptions, moveOptions, natureOptions } from "./teamMemberOptions";

export interface TeamMemberFieldsProps {
  /** 1 始まりの体の番号(legend・ボタンの名前に使う)。 */
  readonly position: number;
  readonly count: number;
  readonly draft: MemberDraft;
  readonly master: MasterData;
  readonly masterSearch: MasterSpeciesSearch | undefined;
  /** この体の種族の実体(マスタか検索で解決したもの。未解決・未選択は null)。 */
  readonly species: MasterSpecies | null;
  /** 特性・技の名前解決の元(マスタ + 検索で解決した分)。 */
  readonly abilityPool: readonly Ability[];
  readonly movePool: readonly Move[];
  /** 種族の解決に失敗した(オンラインで保存済みメンバーを引けなかった)。 */
  readonly resolveFailed: boolean;
  /** メガストーンの持ち物 ID(master.species + 検索で解決した種族から導く。単独の選択肢から除く)。 */
  readonly stoneIds: ReadonlySet<string>;
  /** 古い保存データを直したときの通知(issue 515。無ければ null)。この枠の中に role="status" で出す。 */
  readonly correctionNotice: string | null;
  readonly onChange: (next: MemberDraft) => void;
  readonly onResolved: (resolution: MasterSpeciesResolution) => void;
  readonly onRemove: () => void;
  readonly onMove: (delta: -1 | 1) => void;
}

interface LabeledSelectProps {
  readonly label: string;
  readonly value: string;
  readonly onChange: (value: string) => void;
  readonly children: ReactNode;
  readonly disabled?: boolean;
  readonly describedBy?: string;
}

/** 見えるラベル付きの select(accessible name はラベルから)。 */
function LabeledSelect({ label, value, onChange, children, disabled, describedBy }: LabeledSelectProps) {
  const id = useId();
  return (
    <div className="team-member__field">
      <label htmlFor={id}>{label}</label>
      <select
        id={id}
        value={value}
        disabled={disabled}
        aria-describedby={describedBy}
        onChange={(event) => {
          onChange(event.target.value);
        }}
      >
        {children}
      </select>
    </div>
  );
}

/** 保存できない理由の文言(メンバー1体の alert にまとめて出す)。 */
function issueMessage(issue: MemberIssue): string {
  switch (issue.kind) {
    case "speciesRequired":
      return teamMemberText.speciesRequiredError;
    case "moveDuplicate":
      return teamMemberText.moveDuplicateError;
    case "sp":
      return issue.issue.kind === "stat"
        ? teamMemberText.spStatError(statLetterJa[issue.issue.stat], MAX_SP_PER_STAT)
        : teamMemberText.spTotalError(issue.issue.total - MAX_SP_TOTAL, MAX_SP_TOTAL);
  }
}

/**
 * 非メガの持ち物欄の選択肢: メガストーンを除いたマスタの持ち物。非メガの現在値がメガストーンのとき
 * (直さない。API が検査しないため)は、名前を引いて選択肢に足す(足さないと ID が出る)。
 */
function pickableItemChoices(
  items: readonly MasterItem[],
  stoneIds: ReadonlySet<string>,
  currentId: string | null,
): readonly Item[] {
  // ADR-0326: 構築は実戦で持たせる持ち物の記録なので役割で絞らない(メガストーンだけ外す)。
  const pickable = itemsForRole(items, TEAM_ITEM_ROLE_FILTER, stoneIds);
  const found = currentId === null ? undefined : items.find((item) => item.id === currentId);
  // 現在値のストーンは、名前を推測せず「メガストーン」と表示する(英語名を出さない)。
  const current =
    found !== undefined && isMegaStoneItem(found, stoneIds)
      ? { ...found, nameJa: itemRoleText.megaStoneUnnamed }
      : undefined;
  return itemOptions(current === undefined ? pickable : [...pickable, current], currentId);
}

export function TeamMemberFields({
  position,
  count,
  draft,
  master,
  masterSearch,
  species,
  abilityPool,
  movePool,
  resolveFailed,
  stoneIds,
  correctionNotice,
  onChange,
  onResolved,
  onRemove,
  onMove,
}: TeamMemberFieldsProps) {
  const itemReasonId = useId();
  const hasSpeciesList = masterCapabilities(master).speciesList;
  const spCheck = checkSpDraft(draft.sp);
  const result = draftToMember(draft);
  const messages = [
    ...(result.ok ? [] : result.issues.map(issueMessage)),
    ...(resolveFailed ? [teamMemberText.speciesResolveError] : []),
  ];
  const abilities = species === null ? [] : selectableAbilities(species, abilityPool);
  // issue 515・ADR-0320: メガ種族の持ち物はメガストーンに固定する(固定は種族から毎回導く)。
  const itemLock = megaItemLock(species, master.items);
  const itemChoices = pickableItemChoices(master.items, stoneIds, draft.itemId);

  function pickSpecies(key: string): void {
    const next = master.species.find((candidate) => candidate.key === key);
    if (next === undefined) {
      onChange({ ...draft, speciesKey: null });
      return;
    }
    onChange(
      changeSpecies(draft, next, selectableAbilities(next, master.abilities), {
        previous: species,
        items: master.items,
      }),
    );
  }

  // マスタのタイプ一覧に無い現在値も選択肢に足す(足さないと「(なし)」と表示され保存値とずれる)。
  const masterTypes = master.typeChart.types.filter(isTypeId);
  const teraOptions =
    draft.teraType === null || masterTypes.includes(draft.teraType)
      ? masterTypes
      : [...masterTypes, draft.teraType];

  function pickMove(slot: number, moveId: string): void {
    onChange({
      ...draft,
      moves: draft.moves.map((current, index) =>
        index === slot ? (moveId === "" ? null : moveId) : current,
      ),
    });
  }

  function pickTera(value: string): void {
    onChange({ ...draft, teraType: isTypeId(value) ? value : null });
  }

  function setSp(stat: StatKey, value: string): void {
    onChange({ ...draft, sp: { ...draft.sp, [stat]: value } });
  }

  return (
    <fieldset className="team-member">
      <legend>{teamMemberText.memberLegend(position)}</legend>

      {hasSpeciesList ? (
        <LabeledSelect
          label={teamMemberText.speciesLabel}
          value={draft.speciesKey ?? ""}
          onChange={pickSpecies}
        >
          <option value="">{teamMemberText.speciesPlaceholder}</option>
          {master.species.map((candidate) => (
            <option key={candidate.key} value={candidate.key}>
              {candidate.nameJa}
            </option>
          ))}
          {draft.speciesKey !== null && species === null && (
            <option value={draft.speciesKey}>{teamMemberText.unknownSpeciesOption(draft.speciesKey)}</option>
          )}
        </LabeledSelect>
      ) : (
        <SpeciesSearchField
          label={teamMemberText.speciesLabel}
          masterSearch={masterSearch}
          onResolved={onResolved}
          selectedNameJa={species?.nameJa ?? null}
        />
      )}

      {Array.from({ length: MAX_MEMBER_MOVES }, (_, slot) => {
        const current = draft.moves[slot] ?? null;
        return (
          <LabeledSelect
            key={slot}
            label={teamMemberText.moveLabel(slot + 1)}
            value={current ?? ""}
            onChange={(value) => {
              pickMove(slot, value);
            }}
          >
            <option value="">{teamMemberText.moveNone}</option>
            {moveOptions(species, movePool, current).map((move) => (
              <option
                key={move.id}
                value={move.id}
                disabled={draft.moves.some((other, index) => index !== slot && other === move.id)}
              >
                {move.nameJa}
              </option>
            ))}
          </LabeledSelect>
        );
      })}

      <LabeledSelect
        label={teamMemberText.itemLabel}
        value={
          itemLock.kind === "locked"
            ? itemLock.item.id
            : itemLock.kind === "missing"
              ? ""
              : (draft.itemId ?? "")
        }
        disabled={itemLock.kind !== "none"}
        describedBy={itemLock.kind === "none" ? undefined : itemReasonId}
        onChange={(value) => {
          onChange({ ...draft, itemId: value === "" ? null : value });
        }}
      >
        <option value="">{teamMemberText.itemNone}</option>
        {itemLock.kind === "locked" && species !== null ? (
          <option value={itemLock.item.id}>{megaStoneLabel(species, itemLock.item.nameJa)}</option>
        ) : (
          itemChoices.map((item) => (
            <option key={item.id} value={item.id}>
              {item.nameJa}
            </option>
          ))
        )}
      </LabeledSelect>
      <MegaItemReason id={itemReasonId} lock={itemLock} className="team-member__reason" />

      <div className="team-member__field">
        <AbilitySelect
          ariaLabel={teamMemberText.abilityLabel}
          labelClassName="team-member__ability-label"
          options={abilityOptions(abilities, draft.abilityId)}
          value={draft.abilityId ?? ""}
          autoOptionLabel={draft.abilityId === null ? teamMemberText.abilityUnset : undefined}
          onChange={(id) => {
            onChange({ ...draft, abilityId: id === "" ? null : id });
          }}
        />
      </div>

      <LabeledSelect
        label={teamMemberText.natureLabel}
        value={draft.natureId}
        onChange={(value) => {
          onChange({ ...draft, natureId: value });
        }}
      >
        {natureOptions(master.natures, draft.natureId).map((nature) => (
          <option key={nature.id} value={nature.id}>
            {nature.nameJa}
          </option>
        ))}
      </LabeledSelect>

      <LabeledSelect label={teamMemberText.teraLabel} value={draft.teraType ?? ""} onChange={pickTera}>
        <option value="">{teamMemberText.teraNone}</option>
        {teraOptions.map((type) => (
          <option key={type} value={type}>
            {typeNameJa[type]}
          </option>
        ))}
      </LabeledSelect>

      <fieldset className="team-member__sp">
        <legend>{teamMemberText.spLegend}</legend>
        <div className="team-member__sp-grid">
          {SP_STATS.map((stat) => (
            <label key={stat} className="team-member__sp-cell">
              <span aria-hidden="true">{statLetterJa[stat]}</span>
              <input
                type="text"
                inputMode="numeric"
                aria-label={teamMemberText.spLabel(statLetterJa[stat])}
                aria-invalid={spCheck.issues.some((issue) => issue.kind === "stat" && issue.stat === stat)}
                value={draft.sp[stat]}
                onChange={(event) => {
                  setSp(stat, event.target.value);
                }}
              />
            </label>
          ))}
        </div>
        <p className="team-member__sp-summary">
          {teamMemberText.spSummary(spCheck.total, MAX_SP_TOTAL, spCheck.remaining)}
        </p>
      </fieldset>

      {correctionNotice !== null && (
        <p role="status" className="team-member__notice">
          {correctionNotice}
        </p>
      )}

      {messages.length > 0 && (
        <div role="alert" className="team-member__error">
          {messages.map((message) => (
            <p key={message}>{message}</p>
          ))}
        </div>
      )}

      <div className="team-member__actions">
        <button
          type="button"
          disabled={position === 1}
          onClick={() => {
            onMove(-1);
          }}
        >
          {teamMemberText.moveUpLabel(position)}
        </button>
        <button
          type="button"
          disabled={position === count}
          onClick={() => {
            onMove(1);
          }}
        >
          {teamMemberText.moveDownLabel(position)}
        </button>
        <button type="button" onClick={onRemove}>
          {teamMemberText.removeLabel(position)}
        </button>
      </div>
    </fieldset>
  );
}
