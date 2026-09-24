// P4-16b(ADR-0304 A-10): 種族の検索欄(コンボボックス)。capabilities.speciesList が false のとき、
// ドロップダウンの代わりに出す。accessible name は既存のスロットのラベルのまま(役割も combobox のまま)。
// WAI-ARIA Authoring Practices の Combobox パターン、うち "List Autocomplete with Automatic Selection"
// (候補表示時は先頭を自動ハイライト)に合わせる(ADR-0304 A-12。P4-16c でキーボード操作を追加)。
// 候補は role="listbox" の中の role="option")に合わせる。CalcScreen.tsx・ReverseScreen.tsx で共有する。

import { useEffect, useId, useRef, useState } from "react";
import { masterOnlineText } from "../i18n/ja";
import {
  SPECIES_SEARCH_DEBOUNCE_MS,
  SPECIES_SEARCH_LIMIT,
  SPECIES_SEARCH_MIN_LENGTH,
} from "../master/onlineSource";
import type { MasterSpeciesResolution, MasterSpeciesSearch, MasterSpeciesSummary } from "../master/types";
import "./SpeciesSearchField.css";

export interface SpeciesSearchFieldProps {
  /** accessible name(既存のスロットのラベル。「攻撃側のポケモン」等)。 */
  readonly label: string;
  /** 検索口。省略(undefined)は「検索できない」(組み合わせの誤り。ADR-0304 A-10)。 */
  readonly masterSearch: MasterSpeciesSearch | undefined;
  /** 候補を選んで resolveSpecies が解決したときに呼ぶ。 */
  readonly onResolved: (resolution: MasterSpeciesResolution) => void;
}

/**
 * 検索欄の状態(判別 union)。resolved は候補を選んだ直後(案内も候補も出さない)。
 * closed は Escape で候補を閉じた直後(ADR-0304 A-12): 入力の文字は残すが候補は捨てるので、
 * empty を使い回さず別の kind にする(empty だと「名前を入力してください」の案内が出てしまう)。
 * highlightedIndex は「List Autocomplete with Automatic Selection」のハイライト位置
 * (candidates のうち何番目が選ばれているか。候補を出した直後・入れ替わった直後は必ず 0)。
 */
type SearchStatus =
  | { readonly kind: "empty" }
  | { readonly kind: "pending" }
  | {
      readonly kind: "results";
      readonly candidates: readonly MasterSpeciesSummary[];
      readonly highlightedIndex: number;
    }
  | { readonly kind: "no-result" }
  | { readonly kind: "error" }
  | { readonly kind: "resolved" }
  | { readonly kind: "closed" };

/** 種族の検索欄(ADR-0304 A-4・A-10)。 */
export function SpeciesSearchField({ label, masterSearch, onResolved }: SpeciesSearchFieldProps) {
  const [inputText, setInputText] = useState("");
  const [status, setStatus] = useState<SearchStatus>({ kind: "empty" });
  const debounceTimerRef = useRef<number | null>(null);
  const abortControllerRef = useRef<AbortController | null>(null);
  const hintId = useId();
  const listboxId = useId();

  function clearDebounceTimer(): void {
    if (debounceTimerRef.current !== null) {
      window.clearTimeout(debounceTimerRef.current);
      debounceTimerRef.current = null;
    }
  }

  /** 進行中の検索を取り消す(古い応答で新しい候補を上書きしないため。CalcScreen.tsx の cancelled と同じ考え方)。 */
  function abortPendingSearch(): void {
    abortControllerRef.current?.abort();
    abortControllerRef.current = null;
  }

  // アンマウント時にタイマー・進行中の検索を必ず片付ける(CalcScreen.tsx の clearSwapFallbackTimer と同じ作法)。
  useEffect(() => {
    return () => {
      clearDebounceTimer();
      abortPendingSearch();
    };
  }, []);

  function runSearch(search: MasterSpeciesSearch, query: string): void {
    abortPendingSearch();
    const controller = new AbortController();
    abortControllerRef.current = controller;
    search.searchSpecies(query, controller.signal).then(
      (found) => {
        if (controller.signal.aborted) {
          return;
        }
        setStatus(
          found.length === 0
            ? { kind: "no-result" }
            : { kind: "results", candidates: found, highlightedIndex: 0 },
        );
      },
      () => {
        if (controller.signal.aborted) {
          return;
        }
        setStatus({ kind: "error" });
      },
    );
  }

  function handleChange(nextText: string): void {
    setInputText(nextText);
    clearDebounceTimer();
    abortPendingSearch();
    const trimmed = nextText.trim();
    if (trimmed.length < SPECIES_SEARCH_MIN_LENGTH || masterSearch === undefined) {
      setStatus({ kind: "empty" });
      return;
    }
    setStatus({ kind: "pending" });
    debounceTimerRef.current = window.setTimeout(() => {
      runSearch(masterSearch, trimmed);
    }, SPECIES_SEARCH_DEBOUNCE_MS);
  }

  function selectCandidate(candidate: MasterSpeciesSummary): void {
    if (masterSearch === undefined) {
      return;
    }
    clearDebounceTimer();
    abortPendingSearch();
    setInputText(candidate.nameJa);
    setStatus({ kind: "resolved" });
    masterSearch.resolveSpecies(candidate.key).then(
      (resolution) => {
        onResolved(resolution);
      },
      () => {
        setStatus({ kind: "error" });
      },
    );
  }

  /** 候補の id(aria-activedescendant が指す先。key を使うので一意で安定する)。 */
  function optionId(candidateKey: string): string {
    return `${listboxId}-option-${candidateKey}`;
  }

  /**
   * ArrowDown(delta=1)/ArrowUp(delta=-1)。端で止まる(ループしない。ADR-0304 A-12)。
   * 戻り値は「実際に処理したか」(呼び出し側が処理したときだけ preventDefault するため)。
   */
  function moveHighlight(delta: number): boolean {
    if (status.kind !== "results") {
      return false;
    }
    const maxIndex = status.candidates.length - 1;
    const nextIndex = Math.min(maxIndex, Math.max(0, status.highlightedIndex + delta));
    if (nextIndex === status.highlightedIndex) {
      return false;
    }
    setStatus({ ...status, highlightedIndex: nextIndex });
    return true;
  }

  /** Enter: ハイライト中の候補を確定する。候補が出ていなければ何もしない(戻り値は処理したか)。 */
  function confirmHighlighted(): boolean {
    if (status.kind !== "results") {
      return false;
    }
    const candidate = status.candidates[status.highlightedIndex];
    if (candidate === undefined) {
      return false;
    }
    selectCandidate(candidate);
    return true;
  }

  /**
   * Escape: 候補を閉じる(入力の文字は残す)。閉じたら候補は捨て、ArrowDown では戻さない。
   * 候補が出ていなければ何もしない(戻り値は処理したか)。
   */
  function closeCandidates(): boolean {
    if (status.kind !== "results") {
      return false;
    }
    setStatus({ kind: "closed" });
    return true;
  }

  const disabled = masterSearch === undefined;
  const showListbox = !disabled && status.kind === "results";
  const highlightedCandidate =
    status.kind === "results" ? status.candidates[status.highlightedIndex] : undefined;

  return (
    <div className="species-search">
      <input
        type="text"
        role="combobox"
        aria-label={label}
        aria-expanded={showListbox}
        aria-controls={showListbox ? listboxId : undefined}
        aria-activedescendant={
          showListbox && highlightedCandidate !== undefined ? optionId(highlightedCandidate.key) : undefined
        }
        aria-describedby={hintId}
        placeholder={masterOnlineText.speciesSearchLabel}
        value={inputText}
        disabled={disabled}
        onChange={(event) => {
          handleChange(event.target.value);
        }}
        onKeyDown={(event) => {
          // IME 変換中の Enter・矢印キーは変換の操作であって候補選択ではない。素通しする
          // (ADR-0304 A-12。変換確定の Enter でハイライト中の候補を誤って選んでしまう退行を避ける)。
          if (event.nativeEvent.isComposing) {
            return;
          }
          let handled = false;
          switch (event.key) {
            case "ArrowDown":
              handled = moveHighlight(1);
              break;
            case "ArrowUp":
              handled = moveHighlight(-1);
              break;
            case "Enter":
              handled = confirmHighlighted();
              break;
            case "Escape":
              handled = closeCandidates();
              break;
            default:
              break;
          }
          // 実際に処理したときだけ既定動作を止める(候補が無いときのキャレット移動等を邪魔しない)。
          if (handled) {
            event.preventDefault();
          }
        }}
      />
      <p id={hintId} className="species-search__hint">
        {masterOnlineText.speciesSearchHint}
      </p>
      {disabled && <p className="species-search__status">{masterOnlineText.speciesSearchFailed}</p>}
      {!disabled && status.kind === "empty" && (
        <p className="species-search__status">{masterOnlineText.speciesSearchEmpty}</p>
      )}
      {!disabled && status.kind === "no-result" && (
        <p className="species-search__status">{masterOnlineText.speciesSearchNoResult}</p>
      )}
      {!disabled && status.kind === "error" && (
        <p className="species-search__status">{masterOnlineText.speciesSearchFailed}</p>
      )}
      {!disabled && status.kind === "results" && (
        <>
          {status.candidates.length >= SPECIES_SEARCH_LIMIT && (
            <p className="species-search__status">{masterOnlineText.speciesSearchTruncated}</p>
          )}
          <ul role="listbox" id={listboxId} aria-label={label} className="species-search__list">
            {status.candidates.map((candidate, index) => {
              const isActive = index === status.highlightedIndex;
              return (
                <li
                  key={candidate.key}
                  id={optionId(candidate.key)}
                  role="option"
                  aria-selected={isActive}
                  className={
                    isActive
                      ? "species-search__option species-search__option--active"
                      : "species-search__option"
                  }
                  onClick={() => {
                    selectCandidate(candidate);
                  }}
                >
                  {candidate.nameJa}
                </li>
              );
            })}
          </ul>
        </>
      )}
    </div>
  );
}
