// P4-12a/b: タイプバランスの画面(docs/adr/0303-web-balance-screen.md、docs/type-balance-design.md §6 TB1〜TB5、
// ADR-0400、ADR-0401)。balance API のクライアント(props の client、ADR-0303 §1)へ、メンバー(最大6体。
// ポケモン・特性・技4つまで)を送り、防御相性(analyze)・攻撃範囲(coverage)を出す(P4-12a)。加えて、
// 仮想敵(threats。最大6体、メンバーと同じ入力)・おすすめタイプ(recommendations)を出す(P4-12b)。
// 倍率・集計・穴の判定は応答をそのまま出し、Web で計算し直さない(ADR-0303 §1・§7)。
// ADR-0421: 呼び出しの状態管理は balance/useTeamBalance、結果の表示は balance/BalancePanel に取り出した。
// この画面に残るのは入力欄(メンバー・仮想敵の選択)とその組み合わせだけ。

import { useId, useRef, useState } from "react";
import type { Dispatch, SetStateAction } from "react";
import type { BalanceClient } from "../api/balanceClient";
import { BalancePanel } from "../balance/BalancePanel";
import { findAbilityName } from "../balance/balanceParts";
import type { BalanceInputMember } from "../balance/types";
import { isBalanceAvailable, useTeamBalance } from "../balance/useTeamBalance";
import { learnsetMoves } from "../domain/moves";
import { balanceScreenText, masterOnlineText } from "../i18n/ja";
import { masterCapabilities } from "../master/capabilities";
import type { MasterData, MasterSpeciesResolution, MasterSpeciesSearch } from "../master/types";
import { Icon } from "../ui/Icon";
import { PokemonCard } from "../ui/PokemonCard";
import { Tile } from "../ui/Tile";
import { typeAccentStyle } from "../ui/typeAccent";
import { SpeciesSearchField } from "./SpeciesSearchField";
import { useSpeciesResolutions, type SpeciesResolutions } from "./speciesResolution";
import "./BalanceScreen.css";

/** 画面の props(App.tsx が注入する。マスタは既存の画面と同じ MasterData。ADR-0303 §2・§3)。 */
export interface BalanceScreenProps {
  readonly master: MasterData;
  readonly client: BalanceClient;
  /**
   * P4-17b(ADR-0304 A-14): 種族を都度引く口。`capabilities.speciesList` が false のマスタでは、
   * 各枠(メンバー・仮想敵)のポケモン選択をドロップダウンから検索欄に替え、技・特性も
   * `resolveSpecies` と一緒に解決する(A-14.1・A-14.2)。省略は「検索できない」
   * (capabilities を省いたマスタ = 今までどおりドロップダウン)。
   */
  readonly masterSearch?: MasterSpeciesSearch;
}

/** 技の枠数(services/balance/api/openapi.yaml の moveIds の maxItems)。 */
const MOVE_SLOTS = 4;

/** メンバーの最大数(services/balance/api/openapi.yaml の members の maxItems)。 */
const MAX_MEMBERS = 6;

/** 画面が持つメンバー1件の状態。moveIds は枠(技1〜技4)の順のまま、空欄は ""。 */
interface MemberState {
  readonly id: number;
  readonly speciesKey: string;
  readonly abilityId: string;
  readonly moveIds: readonly string[];
}

/** 画面の1枠を BalanceInputMember にする(特性が空欄なら null。空欄の技や空の枠は useTeamBalance が無視する)。 */
function toBalanceInput(member: MemberState): BalanceInputMember {
  return {
    speciesKey: member.speciesKey,
    abilityId: member.abilityId === "" ? null : member.abilityId,
    moveIds: member.moveIds,
  };
}

function emptyMember(id: number): MemberState {
  return { id, speciesKey: "", abilityId: "", moveIds: Array.from({ length: MOVE_SLOTS }, () => "") };
}

/** メンバーの枠(最大6体)を操作する共通のハンドラの形(P4-12b: 自分のパーティと仮想敵で同じ操作を使う)。 */
interface MemberListActions {
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
function useMemberListActions(setList: Dispatch<SetStateAction<MemberState[]>>): MemberListActions {
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
      setList((current) => (current.length <= 1 ? current : current.filter((_, i) => i !== index)));
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

/** タイプバランスの画面(ADR-0303 §2・§7)。 */
export function BalanceScreen({ master, client, masterSearch }: BalanceScreenProps) {
  // P4-17b(ADR-0304 A-14.1): 画面の可否は「入力の口があるか」で決める(判定は useTeamBalance と共有)。
  const capabilities = masterCapabilities(master);
  const balanceAvailable = isBalanceAvailable(master, masterSearch);

  // P4-17b(ADR-0304 A-14.2): 検索で解決した種族・特性・技の覚え書き。Map なので12枠(メンバー6・
  // 仮想敵6)で1つを共有できる(speciesResolution.ts は変更しない)。
  const { speciesFor, abilitiesFor, movesFor, register: registerSpeciesResolution } = useSpeciesResolutions();

  const [members, setMembers] = useState<MemberState[]>(() => [emptyMember(0)]);
  const memberActions = useMemberListActions(setMembers);

  // P4-12b: 仮想敵の枠(メンバーと同じ構造だが独立した状態。ADR-0303 §7)。
  const [threats, setThreats] = useState<MemberState[]>(() => [emptyMember(0)]);
  const threatActions = useMemberListActions(setThreats);

  /**
   * P4-17b(ADR-0304 A-14.2): 検索でメンバーの種族が解決したとき。枠は id で引く(critic 指摘):
   * 解決を待つ間に他の枠が削除されると index はもう違う枠を指しうるため。
   */
  function handleMemberResolved(id: number, resolution: MasterSpeciesResolution): void {
    registerSpeciesResolution(resolution);
    memberActions.resolveSpecies(id, resolution);
  }

  /** P4-17b(ADR-0304 A-14.2): 検索で仮想敵の種族が解決したとき(同上、id で引く)。 */
  function handleThreatResolved(id: number, resolution: MasterSpeciesResolution): void {
    registerSpeciesResolution(resolution);
    threatActions.resolveSpecies(id, resolution);
  }

  // ADR-0421: balance API の呼び出し(analyze・coverage・threats・recommendations)は useTeamBalance に任せる。
  const balance = useTeamBalance({
    members: members.map(toBalanceInput),
    threats: threats.map(toBalanceInput),
    client,
    master,
    masterSearch,
    movesFor,
  });

  return (
    <div className="balance-screen">
      {/* P4-17b(ADR-0304 A-14.1): ポケモン・技のどちらかを選ぶ口が無いときだけ画面ごと使えない案内を出し、
          入力は残すが全部 disabled にする(balance API は1本も呼ばない)。 */}
      {!balanceAvailable && (
        <p className="ui-notice ui-notice--info balance-screen__notice">
          {masterOnlineText.balanceUnavailable}
        </p>
      )}
      <div className="balance-screen__members">
        {members.map((member, index) => (
          <MemberFields
            key={member.id}
            groupLabel={balanceScreenText.memberGroupLabel(index + 1)}
            removeLabel={balanceScreenText.removeMemberLabel(index + 1)}
            member={member}
            master={master}
            removable={members.length > 1}
            disabled={!balanceAvailable}
            movesListAvailable={capabilities.moves}
            speciesListAvailable={capabilities.speciesList}
            masterSearch={masterSearch}
            speciesFor={speciesFor}
            abilitiesFor={abilitiesFor}
            movesFor={movesFor}
            onSelectSpecies={(speciesKey) => {
              memberActions.selectSpecies(master, index, speciesKey);
            }}
            onSpeciesResolved={(resolution) => {
              handleMemberResolved(member.id, resolution);
            }}
            onSelectAbility={(abilityId) => {
              memberActions.selectAbility(index, abilityId);
            }}
            onSelectMove={(slot, moveId) => {
              memberActions.selectMove(index, slot, moveId);
            }}
            onRemove={() => {
              memberActions.remove(index);
            }}
          />
        ))}
      </div>
      <Tile
        empty
        className="balance-screen__add"
        label={balanceScreenText.addMemberLabel}
        onClick={memberActions.add}
        disabled={!balanceAvailable || members.length >= MAX_MEMBERS}
      />

      <div className="balance-screen__members">
        {threats.map((threat, index) => (
          <MemberFields
            key={threat.id}
            groupLabel={balanceScreenText.threatGroupLabel(index + 1)}
            removeLabel={balanceScreenText.removeThreatLabel(index + 1)}
            member={threat}
            master={master}
            removable={threats.length > 1}
            disabled={!balanceAvailable}
            movesListAvailable={capabilities.moves}
            speciesListAvailable={capabilities.speciesList}
            masterSearch={masterSearch}
            speciesFor={speciesFor}
            abilitiesFor={abilitiesFor}
            movesFor={movesFor}
            onSelectSpecies={(speciesKey) => {
              threatActions.selectSpecies(master, index, speciesKey);
            }}
            onSpeciesResolved={(resolution) => {
              handleThreatResolved(threat.id, resolution);
            }}
            onSelectAbility={(abilityId) => {
              threatActions.selectAbility(index, abilityId);
            }}
            onSelectMove={(slot, moveId) => {
              threatActions.selectMove(index, slot, moveId);
            }}
            onRemove={() => {
              threatActions.remove(index);
            }}
          />
        ))}
      </div>
      <Tile
        empty
        className="balance-screen__add"
        label={balanceScreenText.addThreatLabel}
        onClick={threatActions.add}
        disabled={!balanceAvailable || threats.length >= MAX_MEMBERS}
      />

      <BalancePanel state={balance} master={master} speciesFor={speciesFor} abilitiesFor={abilitiesFor} />
    </div>
  );
}

interface MemberFieldsProps {
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
function MemberFields({
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
