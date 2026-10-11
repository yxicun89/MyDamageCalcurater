// 仮想敵・メンバーの入力欄(ポケモン・特性・技1〜4)と、その枠の一覧を操作するハンドラ。
// タイプバランス画面(screens/BalanceScreen.tsx)から取り出し、構築編集の「仮想敵」の節でも使う(ADR-0421 §3)。

import { useId, useRef } from "react";
import type { Dispatch, SetStateAction } from "react";
import { learnsetMoves } from "../domain/moves";
import { balanceScreenText } from "../i18n/ja";
import type { MasterData, MasterSpeciesResolution, MasterSpeciesSearch } from "../master/types";
import { SpeciesSearchField } from "../screens/SpeciesSearchField";
import type { SpeciesResolutions } from "../screens/speciesResolution";
import { Icon } from "../ui/Icon";
import { PokemonCard } from "../ui/PokemonCard";
import { typeAccentStyle } from "../ui/typeAccent";
import { findAbilityName } from "./balanceParts";
import type { BalanceInputMember } from "./types";
import "./BalancePanel.css";

/** 技の枠数(services/balance/api/openapi.yaml の moveIds の maxItems)。 */
export const MOVE_SLOTS = 4;

/** メンバーの最大数(services/balance/api/openapi.yaml の members の maxItems)。 */
export const MAX_MEMBERS = 6;

/** 画面が持つメンバー1件の状態。moveIds は枠(技1〜技4)の順のまま、空欄は ""。 */
export interface MemberState {
  readonly id: number;
  readonly speciesKey: string;
  readonly abilityId: string;
  readonly moveIds: readonly string[];
}

/** 画面の1枠を BalanceInputMember にする(特性が空欄なら null。空欄の技や空の枠は useTeamBalance が無視する)。 */
export function toBalanceInput(member: MemberState): BalanceInputMember {
  return {
    speciesKey: member.speciesKey,
    abilityId: member.abilityId === "" ? null : member.abilityId,
    moveIds: member.moveIds,
  };
}

export function emptyMember(id: number): MemberState {
  return { id, speciesKey: "", abilityId: "", moveIds: Array.from({ length: MOVE_SLOTS }, () => "") };
}

/** メンバーの枠(最大6体)を操作する共通のハンドラの形(P4-12b: 自分のパーティと仮想敵で同じ操作を使う)。 */
export interface MemberListActions {
  readonly add: () => void;
  readonly remove: (index: number) => void;
  readonly selectSpecies: (master: MasterData, index: number, speciesKey: string) => void;
  /**
   * P4-17b(ADR-0304 A-14.2): 検索で種族が解決したとき。既定の特性を `resolution.species.abilities[0]`
   * にし、技の枠は空に戻す(selectSpecies と同じ形)。`movesFor`/`abilitiesFor` はここで呼ばない
   * (register の setState は非同期で、直後はまだ古い覚え書きのまま。resolution の実体をそのまま使う。
   * CalcScreen.tsx の handleAttackerResolved と同じ理由)。
   * index ではなく member.id で引く(critic 指摘): 検索の解決を待つ間に別の枠が削除・並び替わると、
   * index はもう違う枠を指しうる(SpeciesSearchField の解決に中断が無いため)。id は枠の生成時に一度だけ
   * 割り振られ、削除・並び替えで変わらない。
   */
  readonly resolveSpecies: (id: number, resolution: MasterSpeciesResolution) => void;
  readonly selectAbility: (index: number, abilityId: string) => void;
  readonly selectMove: (index: number, slot: number, moveId: string) => void;
}

/**
 * メンバーの枠(最大6体)を操作する共通のハンドラ(P4-12b: 自分のパーティと仮想敵で同じ操作を使う)。
 * ID 発行用の ref はこの中だけで持つ(render 中に ref を外へ渡さない。react-hooks/refs)。
 * 各操作はアロー関数のプロパティにする(オブジェクトのメソッドとして切り離して渡しても this に依存しない。
 * @typescript-eslint/unbound-method)。
 */
export function useMemberListActions(
  setList: Dispatch<SetStateAction<MemberState[]>>,
  minCount = 1,
): MemberListActions {
  const nextIdRef = useRef(1);
  return {
    add: (): void => {
      setList((current) => {
        if (current.length >= MAX_MEMBERS) {
          return current;
        }
        const id = nextIdRef.current;
        nextIdRef.current += 1;
        return [...current, emptyMember(id)];
      });
    },
    remove: (index: number): void => {
      setList((current) => (current.length <= minCount ? current : current.filter((_, i) => i !== index)));
    },
    selectSpecies: (master: MasterData, index: number, speciesKey: string): void => {
      const species = master.species.find((candidate) => candidate.key === speciesKey);
      setList((current) =>
        current.map((member, i) =>
          i === index
            ? {
                ...member,
                speciesKey,
                abilityId: species?.abilities[0] ?? "",
                moveIds: Array.from({ length: MOVE_SLOTS }, () => ""),
              }
            : member,
        ),
      );
    },
    resolveSpecies: (id: number, resolution: MasterSpeciesResolution): void => {
      setList((current) =>
        current.map((member) =>
          member.id === id
            ? {
                ...member,
                speciesKey: resolution.species.key,
                abilityId: resolution.species.abilities[0] ?? "",
                moveIds: Array.from({ length: MOVE_SLOTS }, () => ""),
              }
            : member,
        ),
      );
    },
    selectAbility: (index: number, abilityId: string): void => {
      setList((current) => current.map((member, i) => (i === index ? { ...member, abilityId } : member)));
    },
    selectMove: (index: number, slot: number, moveId: string): void => {
      setList((current) =>
        current.map((member, i) =>
          i === index
            ? { ...member, moveIds: member.moveIds.map((current2, j) => (j === slot ? moveId : current2)) }
            : member,
        ),
      );
    },
  };
}

export interface MemberFieldsProps {
  /** fieldset の legend(role=group の accessible name。「メンバーn」「仮想敵n」)。 */
  readonly groupLabel: string;
  /** 削除ボタンの文言(「メンバーnを削除」「仮想敵nを削除」)。 */
  readonly removeLabel: string;
  readonly member: MemberState;
  readonly master: MasterData;
  readonly removable: boolean;
  /** P4-17b(ADR-0304 A-14.1): 画面が使えないとき(speciesInputAvailable・moveInputAvailable のどちらかが false)、欄を全部 disabled にする。 */
  readonly disabled: boolean;
  /** P4-17b(ADR-0304 A-14.4): 技の一覧が使えるか(capabilities.moves)。枠ごとの技セレクトの有効・無効の判定に使う。 */
  readonly movesListAvailable: boolean;
  /** P4-17b(ADR-0304 A-14.2): 種族の一覧が使えるか。false ならドロップダウンの代わりに検索欄を出す。 */
  readonly speciesListAvailable: boolean;
  /** 検索口(speciesListAvailable が false のときに使う。省略は「検索できない」)。 */
  readonly masterSearch: MasterSpeciesSearch | undefined;
  readonly speciesFor: SpeciesResolutions["speciesFor"];
  readonly abilitiesFor: SpeciesResolutions["abilitiesFor"];
  readonly movesFor: SpeciesResolutions["movesFor"];
  readonly onSelectSpecies: (speciesKey: string) => void;
  /** 検索で種族が解決したとき(speciesListAvailable が false のときに使う)。 */
  readonly onSpeciesResolved: (resolution: MasterSpeciesResolution) => void;
  readonly onSelectAbility: (abilityId: string) => void;
  readonly onSelectMove: (slot: number, moveId: string) => void;
  readonly onRemove: () => void;
}

/**
 * 1枠分の入力(ポケモン・特性・技1〜4)。fieldset + legend が role=group とその accessible name になる。
 * 自分のパーティ(メンバー)と仮想敵の両方で使う共通コンポーネント(ADR-0303 §7)。
 * P4-17b(ADR-0304 A-14.2): 種族・特性・技の参照は master.* を直接見ず、speciesFor/abilitiesFor/movesFor を通す
 * (検索で解決した分が master.species 等に無くても出るように)。
 */
export function MemberFields({
  groupLabel,
  removeLabel,
  member,
  master,
  removable,
  disabled,
  movesListAvailable,
  speciesListAvailable,
  masterSearch,
  speciesFor,
  abilitiesFor,
  movesFor,
  onSelectSpecies,
  onSpeciesResolved,
  onSelectAbility,
  onSelectMove,
  onRemove,
}: MemberFieldsProps) {
  const species = speciesFor(master.species, member.speciesKey);
  const moves = species === null ? [] : learnsetMoves(species, movesFor(master.moves, member.speciesKey));
  // P4-17b(ADR-0304 A-14.4): 枠ごとの技セレクトの有効・無効(A-13.2 をこの画面に当てはめる)。
  const movesAvailable = movesListAvailable || moves.length > 0;
  // 見えるラベル(issue 304)の id。枠(MemberFields)は複数出るので固定文字列にしない。
  const speciesSelectId = useId();
  const abilitySelectId = useId();
  return (
    <fieldset className="ui-card ui-card--typed balance-member" style={typeAccentStyle(species?.types[0])}>
      <legend>{groupLabel}</legend>
      {species !== null && (
        <PokemonCard
          className="balance-member__card"
          speciesKey={species.key}
          name={species.nameJa}
          types={species.types}
        />
      )}
      {speciesListAvailable ? (
        <>
          <label className="balance-member__label" htmlFor={speciesSelectId}>
            {balanceScreenText.speciesLabel}
          </label>
          <select
            id={speciesSelectId}
            aria-label={balanceScreenText.speciesLabel}
            value={member.speciesKey}
            disabled={disabled}
            onChange={(event) => {
              onSelectSpecies(event.target.value);
            }}
          >
            <option value="" hidden>
              {balanceScreenText.speciesPlaceholderOption}
            </option>
            {master.species.map((candidate) => (
              <option key={candidate.key} value={candidate.key}>
                {candidate.nameJa}
              </option>
            ))}
          </select>
        </>
      ) : (
        <SpeciesSearchField
          label={balanceScreenText.speciesLabel}
          masterSearch={masterSearch}
          onResolved={onSpeciesResolved}
        />
      )}
      <label className="balance-member__label" htmlFor={abilitySelectId}>
        {balanceScreenText.abilityLabel}
      </label>
      <select
        id={abilitySelectId}
        aria-label={balanceScreenText.abilityLabel}
        value={member.abilityId}
        disabled={disabled}
        onChange={(event) => {
          onSelectAbility(event.target.value);
        }}
      >
        {/* ポケモンを選ぶまで特性の候補が1件も無い(空の枠)。未選択の option は hidden にして
            一覧(role=option)には出さない(issue 304)。 */}
        <option value="" hidden>
          {balanceScreenText.abilityPlaceholderOption}
        </option>
        {species?.abilities.map((abilityId) => (
          <option key={abilityId} value={abilityId}>
            {findAbilityName(abilitiesFor, master, member.speciesKey, abilityId)}
          </option>
        ))}
      </select>
      {Array.from({ length: MOVE_SLOTS }, (_, slot) => slot).map((slot) => (
        <MoveFieldSelect
          key={slot}
          label={balanceScreenText.moveLabel(slot + 1)}
          value={member.moveIds[slot] ?? ""}
          disabled={disabled || !movesAvailable}
          moves={moves}
          onChange={(moveId) => {
            onSelectMove(slot, moveId);
          }}
        />
      ))}
      {removable && (
        <button
          type="button"
          onClick={onRemove}
          disabled={disabled}
          className="ui-button ui-button--secondary balance-member__remove"
        >
          <Icon name="trash" size={16} />
          {removeLabel}
        </button>
      )}
    </fieldset>
  );
}

interface MoveFieldSelectProps {
  /** 見えるラベルと accessible name の両方に使う語(「技1」〜「技4」)。 */
  readonly label: string;
  readonly value: string;
  readonly disabled: boolean;
  readonly moves: readonly { readonly id: string; readonly nameJa: string }[];
  readonly onChange: (moveId: string) => void;
}

/** 技1〜4の1枠(issue 304: 見えるラベルを持つ)。 */
function MoveFieldSelect({ label, value, disabled, moves, onChange }: MoveFieldSelectProps) {
  const selectId = useId();
  return (
    <>
      <label className="balance-member__label" htmlFor={selectId}>
        {label}
      </label>
      <select
        id={selectId}
        aria-label={label}
        value={value}
        disabled={disabled}
        onChange={(event) => {
          onChange(event.target.value);
        }}
      >
        <option value="">{balanceScreenText.noMoveOption}</option>
        {moves.map((move) => (
          <option key={move.id} value={move.id}>
            {move.nameJa}
          </option>
        ))}
      </select>
    </>
  );
}
