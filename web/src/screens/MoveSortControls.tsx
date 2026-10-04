// I-web-9 = F-02(ADR-0335): 計算画面・逆算画面の技欄で共有する、並びのチップ群と並べた選択肢。

import { useId } from "react";
import { useMoveSort } from "../app/useMoveSort";
import { formatMoveCategory } from "../domain/format";
import { MOVE_SORT_ORDERS, moveTypeGroups, sortMoves, type MoveSortOrder } from "../domain/moveSort";
import type { Move } from "../engine/types";
import { favoritesRestoreText } from "../i18n/favorites";
import { calcScreenText, isTypeId, typeNameJa } from "../i18n/ja";
import { moveSortText } from "../i18n/moveSort";

/** チップ群の見た目のクラス(計算画面は calc-preset、逆算画面は reverse-preset。どちらも ui-chip を併用)。 */
export type MoveSortChipsVariant = "calc" | "reverse";

export function MoveSortChips({ variant }: { readonly variant: MoveSortChipsVariant }) {
  const [order, setOrder] = useMoveSort();
  const name = useId();
  const prefix = variant === "calc" ? "calc-preset" : "reverse-preset";
  return (
    <div role="radiogroup" aria-label={moveSortText.groupLabel} className={prefix}>
      {MOVE_SORT_ORDERS.map((key) => {
        const selected = key === order;
        return (
          <label
            key={key}
            className={`ui-chip${selected ? " ui-chip--selected" : ""} ${prefix}__option${selected ? ` ${prefix}__option--selected` : ""}`}
          >
            <input
              type="radio"
              name={name}
              className={`${prefix}__input`}
              checked={selected}
              onChange={() => {
                setOrder(key);
              }}
            />
            {moveSortText.options[key]}
          </label>
        );
      })}
    </div>
  );
}

function MoveOption({ move }: { readonly move: Move }) {
  return (
    <option value={move.id}>
      {move.nameJa}
      {calcScreenText.moveOptionSeparator}
      {formatMoveCategory(move.category)}
      {move.category === "status"
        ? ""
        : `${calcScreenText.moveOptionSeparator}${calcScreenText.movePowerLabel}${String(move.power)}`}
    </option>
  );
}

interface MoveOptionsProps {
  readonly moves: readonly Move[];
  /** マスタのタイプ表の並び(master.typeChart.types)。 */
  readonly types: readonly string[];
  /** 並び。`<select>` を持つ側が useMoveSort で読んで渡す(select 自身を再描画し、選択中の値を保つため)。 */
  readonly order: MoveSortOrder;
  /** 選択中の技 ID。 */
  readonly value: string;
  /** 技が空のとき、未選択の選択肢(お気に入りの技を戻せなかったとき。ADR-0333 §4)を先頭に出すか(計算画面だけ)。 */
  readonly showUnselected?: boolean;
}

/** `<select>` の中身。並びはチップ群で選んだもの。未選択の選択肢は常に先頭(optgroup の外)。 */
export function MoveOptions({ moves, types, order, value, showUnselected = false }: MoveOptionsProps) {
  return (
    <>
      {showUnselected && value === "" && moves.length > 0 && (
        <option value="" disabled>
          {favoritesRestoreText.moveUnselectedOption}
        </option>
      )}
      {order === "type"
        ? moveTypeGroups(moves, types).map((group) => (
            <optgroup key={group.type} label={isTypeId(group.type) ? typeNameJa[group.type] : group.type}>
              {group.moves.map((move) => (
                <MoveOption key={move.id} move={move} />
              ))}
            </optgroup>
          ))
        : sortMoves(moves, order, types).map((move) => <MoveOption key={move.id} move={move} />)}
    </>
  );
}
