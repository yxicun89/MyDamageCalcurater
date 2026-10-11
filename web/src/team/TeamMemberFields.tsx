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
import { Icon } from "../ui/Icon";
import { typeAccentStyle } from "../ui/typeAccent";
import { uiText } from "../i18n/ui";
import { AbilitySelect } from "../screens/AbilitySelect";
import { MegaItemReason } from "../screens/MegaItemReason";
import { PokemonIcon } from "../images/PokemonIcon";
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

interface SpCellProps {
  readonly stat: StatKey;
  readonly text: string;
  readonly invalid: boolean;
  readonly onChange: (text: string) => void;
}

/**
 * SP 1 欄: 数値欄(文字のまま持つ。不正な入力は誤りとして出すため数値欄にはしない)と、1 ずつ増減する − / + ボタン(G-05)。
 * 空欄は 0、整数でない文字から押したときは 0〜MAX_SP_PER_STAT に収めた値に直す(計算画面の SpField と同じ)。
 */
function SpCell({ stat, text, invalid, onChange }: SpCellProps) {
  const label = teamMemberText.spLabel(statLetterJa[stat]);
  const parsed = text.trim() === "" ? 0 : Number(text);
  const isInteger = Number.isInteger(parsed);
  const current = isInteger ? parsed : 0;
  const clampedStep = (delta: number): void => {
    onChange(String(Math.min(MAX_SP_PER_STAT, Math.max(0, current + delta))));
  };
  const atMin = isInteger && current <= 0;
  const atMax = isInteger && current >= MAX_SP_PER_STAT;
  return (
    <div className="team-member__sp-cell">
      <span aria-hidden="true" className="team-member__sp-name">
        {statLetterJa[stat]}
      </span>
      <div className="ui-stepper">
        <button
          type="button"
          className="ui-stepper__button"
          aria-label={`${label}${uiText.stepperDecrease}`}
          aria-disabled={atMin ? true : undefined}
          onClick={() => {
            if (!atMin) {
              clampedStep(-1);
            }
          }}
        >
          <Icon name="minus" size={16} />
        </button>
        <input
          type="text"
          inputMode="numeric"
          className="ui-stepper__input"
          aria-label={label}
          aria-invalid={invalid}
          value={text}
          onChange={(event) => {
            onChange(event.target.value);
          }}
        />
        <button
          type="button"
          className="ui-stepper__button"
          aria-label={`${label}${uiText.stepperIncrease}`}
          aria-disabled={atMax ? true : undefined}
          onClick={() => {
            if (!atMax) {
              clampedStep(1);
            }
          }}
        >
          <Icon name="plus" size={16} />
        </button>
      </div>
    </div>
  );
}

/** 保存できない理由の文言(メンバー1体の alert にまとめて出す)。 */
function issueMessage(issue: MemberIssue): string {
  switch (issue.kind) {
    // 空の枠は保存の対象外(slotsToMembers)なので画面には届かない。draftToMember の検査として残す。
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

  const speciesField = hasSpeciesList ? (
    <LabeledSelect label={teamMemberText.speciesLabel} value={draft.speciesKey ?? ""} onChange={pickSpecies}>
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
  );

  // 空の枠(ADR-0332 §2): 種族の欄と案内だけ。種族を選ぶと下の欄が出る。
  if (draft.speciesKey === null) {
    return (
      <fieldset className="team-member ui-card">
        <legend>{teamMemberText.memberLegend(position)}</legend>
        {speciesField}
        <p className="team-member__hint">{teamMemberText.emptySlotHint}</p>
      </fieldset>
    );
  }

  // 種族が決まった枠は、最初のタイプの色でカードを染める(種族を引けないときは染めない)。
  const typeId = species?.types[0];
  return (
    <fieldset
      className={typeId === undefined ? "team-member ui-card" : "team-member ui-card ui-card--typed"}
      style={typeAccentStyle(typeId)}
    >
      <legend>
        {species !== null && <PokemonIcon speciesKey={species.key} typeId={species.types[0]} />}
        {teamMemberText.memberLegend(position)}
      </legend>

      {speciesField}

      <div className="team-member__grid">
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
      </div>

      <div className="team-member__grid">
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
      </div>

      <fieldset className="team-member__sp">
        <legend>{teamMemberText.spLegend}</legend>
        <div className="team-member__sp-grid">
          {SP_STATS.map((stat) => (
            <SpCell
              key={stat}
              stat={stat}
              text={draft.sp[stat]}
              invalid={spCheck.issues.some((issue) => issue.kind === "stat" && issue.stat === stat)}
              onChange={(text) => {
                setSp(stat, text);
              }}
            />
          ))}
        </div>
        <p
          className={
            spCheck.remaining < 0
              ? "team-member__sp-summary team-member__sp-summary--over"
              : "team-member__sp-summary"
          }
        >
          {teamMemberText.spSummary(spCheck.total, MAX_SP_TOTAL, spCheck.remaining)}
        </p>
        <span aria-hidden="true" className="team-member__sp-bar">
          <span
            className="team-member__sp-bar-fill"
            style={{ width: `${String(Math.min(100, (spCheck.total / MAX_SP_TOTAL) * 100))}%` }}
          />
        </span>
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
          className="ui-button ui-button--secondary team-member__icon-button"
          aria-label={teamMemberText.moveUpLabel(position)}
          disabled={position === 1}
          onClick={() => {
            onMove(-1);
          }}
        >
          <Icon name="up" size={20} />
        </button>
        <button
          type="button"
          className="ui-button ui-button--secondary team-member__icon-button"
          aria-label={teamMemberText.moveDownLabel(position)}
          disabled={position === count}
          onClick={() => {
            onMove(1);
          }}
        >
          <Icon name="down" size={20} />
        </button>
        <button
          type="button"
          className="ui-button ui-button--danger team-member__icon-button"
          aria-label={teamMemberText.removeLabel(position)}
          onClick={onRemove}
        >
          <Icon name="trash" size={20} />
        </button>
      </div>
    </fieldset>
  );
}
