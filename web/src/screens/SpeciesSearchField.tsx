// P4-16b(ADR-0304 A-10): 種族の検索欄(コンボボックス)。capabilities.speciesList が false のとき、
// ドロップダウンの代わりに出す。accessible name は既存のスロットのラベルのまま(役割も combobox のまま)。
// WAI-ARIA Authoring Practices の Combobox パターン(入力欄が aria-expanded・aria-controls を持ち、
// 候補は role="listbox" の中の role="option")に合わせる。CalcScreen.tsx・ReverseScreen.tsx で共有する。

import { useEffect, useId, useRef, useState } from "react";
import { masterOnlineText } from "../i18n/ja";
import {
  SPECIES_SEARCH_DEBOUNCE_MS,
  SPECIES_SEARCH_LIMIT,
  SPECIES_SEARCH_MIN_LENGTH,
} from "../master/onlineSource";
import type { MasterSpeciesResolution, MasterSpeciesSearch, MasterSpeciesSummary } from "../master/types";

export interface SpeciesSearchFieldProps {
  /** accessible name(既存のスロットのラベル。「攻撃側のポケモン」等)。 */
  readonly label: string;
  /** 検索口。省略(undefined)は「検索できない」(組み合わせの誤り。ADR-0304 A-10)。 */
  readonly masterSearch: MasterSpeciesSearch | undefined;
  /** 候補を選んで resolveSpecies が解決したときに呼ぶ。 */
  readonly onResolved: (resolution: MasterSpeciesResolution) => void;
}

/** 検索欄の状態(判別 union)。resolved は候補を選んだ直後(案内も候補も出さない)。 */
type SearchStatus =
  | { readonly kind: "empty" }
  | { readonly kind: "pending" }
  | { readonly kind: "results"; readonly candidates: readonly MasterSpeciesSummary[] }
  | { readonly kind: "no-result" }
  | { readonly kind: "error" }
  | { readonly kind: "resolved" };

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
        setStatus(found.length === 0 ? { kind: "no-result" } : { kind: "results", candidates: found });
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

  const disabled = masterSearch === undefined;
  const showListbox = !disabled && status.kind === "results";

  return (
    <div className="species-search">
      <input
        type="text"
        role="combobox"
        aria-label={label}
        aria-expanded={showListbox}
        aria-controls={listboxId}
        aria-describedby={hintId}
        placeholder={masterOnlineText.speciesSearchLabel}
        value={inputText}
        disabled={disabled}
        onChange={(event) => {
          handleChange(event.target.value);
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
            {status.candidates.map((candidate) => (
              <li
                key={candidate.key}
                role="option"
                aria-selected={false}
                className="species-search__option"
                onClick={() => {
                  selectCandidate(candidate);
                }}
              >
                {candidate.nameJa}
              </li>
            ))}
          </ul>
        </>
      )}
    </div>
  );
}
