// G-01(ADR-0341、docs/design.md「見た目の作り直し方針(G-05)」): 技ピッカー。<select> の代わりに、
// トリガー(select-only combobox)+ 検索つきの listbox パネルで出す。行は 1 行(タイプバッジ・技名・分類アイコン・威力)。
// 並びはタイプ順だけ(domain/moveOrder.ts)。計算画面・逆算画面で共有する。

import { useEffect, useId, useMemo, useRef, useState, type KeyboardEvent, type ReactElement } from "react";
import { formatMovePickerCategory } from "../domain/format";
import { orderMoves } from "../domain/moveOrder";
import type { Move } from "../engine/types";
import { favoritesRestoreText } from "../i18n/favorites";
import { calcScreenText, isTypeId, typeNameJa } from "../i18n/ja";
import { movePickerText } from "../i18n/movePicker";
import { Icon, type IconName } from "../ui/Icon";
import "./MovePicker.css";

const CATEGORY_ICON: Record<Move["category"], IconName> = {
  physical: "movePhysical",
  special: "moveSpecial",
  status: "moveStatus",
};

/** 検索用に揃える: 全角半角・大小は同じ字、カタカナはひらがなにする(濁点は別の字のまま)。 */
function normalize(text: string): string {
  return text
    .normalize("NFKC")
    .toLowerCase()
    .replace(/[ァ-ヶ]/g, (ch) => String.fromCharCode(ch.charCodeAt(0) - 0x60));
}

function typeName(type: string): string {
  return isTypeId(type) ? typeNameJa[type] : type;
}

function TypeBadge({ type }: { readonly type: string }): ReactElement {
  return (
    <span
      className="ui-badge move-picker__type"
      style={{
        backgroundColor: `var(--type-${type}, var(--border-hairline))`,
        color: `var(--type-${type}-ink, var(--text-primary))`,
      }}
    >
      {typeName(type)}
    </span>
  );
}

function rowLabel(move: Move): string {
  const parts = [move.nameJa, typeName(move.type), formatMovePickerCategory(move.category)];
  if (move.category !== "status") {
    parts.push(`${calcScreenText.movePowerLabel} ${String(move.power)}`);
  }
  return parts.join("、");
}

export interface MovePickerProps {
  /** 名前(見えるラベルと同じ。「技」)。 */
  readonly label: string;
  readonly moves: readonly Move[];
  /** マスタのタイプ表の並び(タイプ順の群の並び)。 */
  readonly types: readonly string[];
  /** 選択中の技 ID(未選択は空文字)。 */
  readonly value: string;
  readonly onChange: (moveId: string) => void;
  readonly disabled?: boolean;
  /** 技が空のとき、先頭に押せない「技を選んでください」行を出すか(計算画面。ADR-0333 §4)。 */
  readonly showUnselected?: boolean;
  /** 見えるラベルのクラス(画面ごと)。 */
  readonly labelClassName?: string;
  /** トリガーに足すクラス(画面ごとの入力の形。例: calc-screen__move)。 */
  readonly triggerClassName?: string;
}

export function MovePicker({
  label,
  moves,
  types,
  value,
  onChange,
  disabled = false,
  showUnselected = false,
  labelClassName,
  triggerClassName,
}: MovePickerProps): ReactElement {
  const baseId = useId();
  const triggerId = `${baseId}-trigger`;
  const listId = `${baseId}-list`;
  const rootRef = useRef<HTMLDivElement>(null);
  const triggerRef = useRef<HTMLButtonElement>(null);
  const searchRef = useRef<HTMLInputElement>(null);
  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState("");
  const [activeId, setActiveId] = useState<string | null>(null);

  const ordered = useMemo(() => orderMoves(moves, types), [moves, types]);
  const selected = ordered.find((move) => move.id === value) ?? null;
  const shown = useMemo(() => {
    const needle = normalize(query.trim());
    return needle === "" ? ordered : ordered.filter((move) => normalize(move.nameJa).includes(needle));
  }, [ordered, query]);
  const isOpen = open && !disabled;
  const showUnselectedRow = showUnselected && value === "" && ordered.length > 0 && query.trim() === "";
  const active = shown.find((move) => move.id === activeId) ?? shown[0] ?? null;

  function openPanel(): void {
    setQuery("");
    setActiveId(selected?.id ?? ordered[0]?.id ?? null);
    setOpen(true);
  }

  function closePanel(): void {
    setOpen(false);
  }

  function choose(id: string): void {
    onChange(id);
    setOpen(false);
    triggerRef.current?.focus();
  }

  // 開いた直後は検索欄へフォーカス。
  useEffect(() => {
    if (isOpen) {
      searchRef.current?.focus();
    }
  }, [isOpen]);

  // 活性の行が見える位置へ。
  useEffect(() => {
    if (isOpen && active !== null) {
      const row = document.getElementById(`${listId}-${active.id}`);
      // jsdom には scrollIntoView が無い。
      // eslint-disable-next-line @typescript-eslint/no-unnecessary-condition
      row?.scrollIntoView?.({ block: "nearest" });
    }
  }, [isOpen, active, listId]);

  // 外側の押下で閉じる(選ばない)。
  useEffect(() => {
    if (!isOpen) {
      return;
    }
    function onPointerDown(event: Event): void {
      if (
        rootRef.current !== null &&
        event.target instanceof Node &&
        !rootRef.current.contains(event.target)
      ) {
        setOpen(false);
      }
    }
    document.addEventListener("pointerdown", onPointerDown);
    return () => {
      document.removeEventListener("pointerdown", onPointerDown);
    };
  }, [isOpen]);

  function moveActive(index: number): void {
    const next = shown[Math.max(0, Math.min(shown.length - 1, index))];
    if (next !== undefined) {
      setActiveId(next.id);
    }
  }

  function onSearchKeyDown(event: KeyboardEvent<HTMLInputElement>): void {
    // 日本語入力の変換中(確定の Enter・候補を選ぶ矢印)は何もしない。
    if (event.nativeEvent.isComposing) {
      return;
    }
    const index = active === null ? 0 : shown.indexOf(active);
    switch (event.key) {
      case "ArrowDown":
        event.preventDefault();
        moveActive(index + 1);
        break;
      case "ArrowUp":
        event.preventDefault();
        moveActive(index - 1);
        break;
      case "Home":
        event.preventDefault();
        moveActive(0);
        break;
      case "End":
        event.preventDefault();
        moveActive(shown.length - 1);
        break;
      case "Enter":
        event.preventDefault();
        if (active !== null) {
          choose(active.id);
        }
        break;
      case "Escape":
        event.preventDefault();
        closePanel();
        triggerRef.current?.focus();
        break;
      case "Tab":
        // トリガーに戻してから閉じる(次に進む位置をトリガー基準にする)。
        triggerRef.current?.focus();
        closePanel();
        break;
    }
  }

  return (
    <div className="move-picker" ref={rootRef}>
      <label className={labelClassName} htmlFor={triggerId}>
        {label}
      </label>
      <button
        type="button"
        id={triggerId}
        ref={triggerRef}
        className={
          triggerClassName === undefined ? "move-picker__trigger" : `move-picker__trigger ${triggerClassName}`
        }
        role="combobox"
        aria-haspopup="listbox"
        aria-expanded={isOpen}
        aria-controls={isOpen && shown.length > 0 ? listId : undefined}
        aria-label={label}
        data-value={value}
        disabled={disabled}
        onClick={() => {
          if (isOpen) {
            closePanel();
          } else {
            openPanel();
          }
        }}
        onKeyDown={(event) => {
          if (event.key === "ArrowDown" || event.key === "ArrowUp") {
            event.preventDefault();
            if (!isOpen) {
              openPanel();
            }
          }
        }}
      >
        {selected === null ? (
          <span className="move-picker__name move-picker__name--placeholder">
            {favoritesRestoreText.moveUnselectedOption}
          </span>
        ) : (
          <>
            <TypeBadge type={selected.type} />
            <span className="move-picker__name">{selected.nameJa}</span>
            <Icon
              name={CATEGORY_ICON[selected.category]}
              label={formatMovePickerCategory(selected.category)}
              size={16}
              className="move-picker__category"
            />
            {selected.category !== "status" && <span className="move-picker__power">{selected.power}</span>}
          </>
        )}
        <span className="move-picker__chevron" aria-hidden="true" />
      </button>
      {isOpen && (
        <div
          className="move-picker__panel"
          onMouseDown={(event) => {
            // 行を押しても検索欄のフォーカスを外さない。
            if (event.target !== searchRef.current) {
              event.preventDefault();
            }
          }}
        >
          <input
            ref={searchRef}
            type="search"
            className="move-picker__search"
            role="searchbox"
            aria-label={movePickerText.searchLabel}
            aria-controls={shown.length > 0 ? listId : undefined}
            aria-activedescendant={active === null ? undefined : `${listId}-${active.id}`}
            autoComplete="off"
            placeholder={movePickerText.searchLabel}
            value={query}
            onChange={(event) => {
              setQuery(event.target.value);
              setActiveId(null);
            }}
            onKeyDown={onSearchKeyDown}
          />
          {shown.length === 0 ? (
            <p role="status" className="move-picker__empty">
              {movePickerText.noResults}
            </p>
          ) : (
            <ul id={listId} role="listbox" aria-label={label} className="move-picker__list">
              {showUnselectedRow && (
                <li
                  role="option"
                  aria-selected="false"
                  aria-disabled="true"
                  aria-label={favoritesRestoreText.moveUnselectedOption}
                  data-move-id=""
                  className="move-picker__row move-picker__row--unselected"
                >
                  <span className="move-picker__name move-picker__name--placeholder">
                    {favoritesRestoreText.moveUnselectedOption}
                  </span>
                </li>
              )}
              {shown.map((move) => (
                <li
                  key={move.id}
                  id={`${listId}-${move.id}`}
                  role="option"
                  aria-selected={move.id === value}
                  aria-label={rowLabel(move)}
                  data-move-id={move.id}
                  className={`move-picker__row${move.id === active?.id ? " move-picker__row--active" : ""}`}
                  onClick={() => {
                    choose(move.id);
                  }}
                >
                  <TypeBadge type={move.type} />
                  <span className="move-picker__name">{move.nameJa}</span>
                  <Icon
                    name={CATEGORY_ICON[move.category]}
                    label={formatMovePickerCategory(move.category)}
                    size={16}
                    className="move-picker__category"
                  />
                  <span className="move-picker__power">{move.category === "status" ? "" : move.power}</span>
                </li>
              ))}
            </ul>
          )}
        </div>
      )}
    </div>
  );
}
