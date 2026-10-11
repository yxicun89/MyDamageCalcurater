// ADR-0338: 計算履歴の一覧(お気に入りの画面の第2の節)。種族・技は key のまま出す(マスタを使わない)。
// 取得: recordClient / reloadToken が変わるたびに先頭ページを1回。古い取得は abort し、古い応答は捨てる(ポーリングなし)。
// 失敗は節の中の alert だけに閉じ込める(絶対ルール5)。オフライン(recordClient なし)は何も出さない。

import { useEffect, useRef, useState, type ReactNode } from "react";
import type { components } from "../api/openapi.gen";
import { PokemonIcon } from "../images/PokemonIcon";
import { Icon } from "../ui/Icon";
import { calcHistoryText } from "../i18n/favorites";
import type { RecordClient, RecordError } from "../record/recordClient";
import { historyEntryAsFavorite } from "./calcHistoryFavorite";

type Schemas = components["schemas"];
type Entry = Schemas["CalcHistoryEntry"];

export interface CalcHistorySectionProps {
  readonly recordClient?: RecordClient;
  /** 値が変わったら先頭から取り直す(端末データの削除の後など)。 */
  readonly reloadToken?: number;
  /** 渡したときだけ各行に「この計算を使う」を出す。 */
  readonly onUse?: (favorite: Schemas["Favorite"]) => void;
}

type HistoryState =
  | { readonly status: "loading" }
  | { readonly status: "error"; readonly error: RecordError }
  | {
      readonly status: "loaded";
      readonly items: readonly Entry[];
      readonly nextCursor: string | null;
      readonly moreError: RecordError | null;
    };

const INVALID_INPUT_CODE = "invalid_input";
/** ダメージバーの満タン(計算画面の DAMAGE_BAR_MAX_PERCENT と同じ。100% を超えても枠で止める)。 */
const BAR_MAX_PERCENT = 100;

const dateFormat = new Intl.DateTimeFormat("ja-JP", { dateStyle: "medium", timeStyle: "short" });

function formatOccurredAt(value: string): string {
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? value : dateFormat.format(date);
}

export function CalcHistorySection({ recordClient, reloadToken, onUse }: CalcHistorySectionProps): ReactNode {
  const [state, setState] = useState<HistoryState>({ status: "loading" });
  const [loadingMore, setLoadingMore] = useState(false);
  // 今の取得の世代ごとの AbortController(先頭ページ・続きとも。世代が変わる/アンマウントで abort)。
  const controllerRef = useRef<AbortController | null>(null);
  const loadingMoreRef = useRef(false);

  const [prevSource, setPrevSource] = useState({ recordClient, reloadToken });
  if (prevSource.recordClient !== recordClient || prevSource.reloadToken !== reloadToken) {
    setPrevSource({ recordClient, reloadToken });
    setState({ status: "loading" });
    setLoadingMore(false);
  }

  /** 先頭ページを読む(世代の最初・400 の読み直し)。応答が古ければ捨てる。 */
  function loadFirst(client: RecordClient, controller: AbortController): void {
    void client.listCalcHistory(undefined, controller.signal).then((result) => {
      if (controller.signal.aborted) {
        return;
      }
      loadingMoreRef.current = false;
      setLoadingMore(false);
      setState(
        result.ok
          ? {
              status: "loaded",
              items: result.value.items,
              nextCursor: result.value.nextCursor,
              moreError: null,
            }
          : { status: "error", error: result.error },
      );
    });
  }

  useEffect(() => {
    if (recordClient === undefined) {
      return;
    }
    const controller = new AbortController();
    controllerRef.current = controller;
    loadingMoreRef.current = false;
    loadFirst(recordClient, controller);
    return () => {
      controller.abort();
    };
  }, [recordClient, reloadToken]);

  if (recordClient === undefined) {
    return null;
  }
  const client = recordClient;

  function handleMore(): void {
    if (state.status !== "loaded" || state.nextCursor === null || loadingMoreRef.current) {
      return;
    }
    const cursor = state.nextCursor;
    const generation = controllerRef.current;
    if (generation === null || generation.signal.aborted) {
      return;
    }
    loadingMoreRef.current = true;
    setLoadingMore(true);
    void client.listCalcHistory(cursor, generation.signal).then((result) => {
      if (generation.signal.aborted) {
        return;
      }
      if (result.ok) {
        loadingMoreRef.current = false;
        setLoadingMore(false);
        setState((current) =>
          current.status === "loaded"
            ? {
                status: "loaded",
                items: [...current.items, ...result.value.items],
                nextCursor: result.value.nextCursor,
                moreError: null,
              }
            : current,
        );
        return;
      }
      if (result.error.code === INVALID_INPUT_CODE) {
        // 古い cursor。先頭から1回だけ読み直す(これも失敗したら loadFirst がエラー表示で止める)。
        loadFirst(client, generation);
        return;
      }
      loadingMoreRef.current = false;
      setLoadingMore(false);
      setState((current) =>
        current.status === "loaded" ? { ...current, moreError: result.error } : current,
      );
    });
  }

  return (
    <section className="calc-history" aria-label={calcHistoryText.regionLabel}>
      <h3 className="calc-history__heading">
        <Icon name="history" size={24} />
        {calcHistoryText.regionLabel}
      </h3>
      {state.status === "loading" && (
        <p className="ui-notice ui-notice--loading calc-history__notice">
          <Icon name="history" size={24} />
          {calcHistoryText.loadingNotice}
        </p>
      )}
      {state.status === "error" && (
        <div role="alert" className="ui-notice ui-notice--error calc-history__error">
          <p className="calc-history__error-heading">
            <Icon name="alert" size={20} />
            {calcHistoryText.errorHeading}
          </p>
          <p>{state.error.message}</p>
        </div>
      )}
      {state.status === "loaded" && state.items.length === 0 && (
        <p className="ui-notice ui-notice--empty calc-history__notice calc-history__empty">
          <Icon name="history" size={48} />
          {calcHistoryText.emptyNotice}
        </p>
      )}
      {state.status === "loaded" && state.items.length > 0 && (
        <ul aria-label={calcHistoryText.listLabel} className="calc-history__list">
          {state.items.map((entry, index) => {
            const label = calcHistoryText.pairLabel(
              entry.calc.attacker.speciesKey,
              entry.calc.defender.speciesKey,
            );
            return (
              // 同じ内容の行が並びうるので、キーは配列の位置(一覧は先頭から足すだけで並べ替えない)。
              <li
                key={index}
                className={`ui-card calc-history__item${onUse !== undefined ? " calc-history__item--pressable" : ""}`}
              >
                <span className="calc-history__icons">
                  <PokemonIcon speciesKey={entry.calc.attacker.speciesKey} />
                  <Icon name="down" size={16} className="calc-history__arrow" />
                  <PokemonIcon speciesKey={entry.calc.defender.speciesKey} />
                </span>
                <span className="calc-history__pair">{label}</span>
                <span className="calc-history__move">{calcHistoryText.moveLabel(entry.calc.moveId)}</span>
                <span className="calc-history__percent">
                  {calcHistoryText.rangeLabel(entry.result.minPercent, entry.result.maxPercent)}
                </span>
                <div aria-hidden="true" data-testid="history-bar" className="calc-history__bar">
                  <div
                    data-testid="history-bar-fill"
                    className="calc-history__bar-fill"
                    style={{ width: `${String(Math.min(entry.result.maxPercent, BAR_MAX_PERCENT))}%` }}
                  />
                </div>
                <time className="calc-history__time" dateTime={entry.occurredAt}>
                  {formatOccurredAt(entry.occurredAt)}
                </time>
                {onUse !== undefined && (
                  <button
                    type="button"
                    className="ui-button ui-button--primary calc-history__use"
                    aria-label={calcHistoryText.useLabel}
                    onClick={() => {
                      onUse(historyEntryAsFavorite(entry, label));
                    }}
                  >
                    <Icon name="calc" size={20} />
                    {calcHistoryText.useShort}
                  </button>
                )}
              </li>
            );
          })}
        </ul>
      )}
      {state.status === "loaded" && state.moreError !== null && (
        <div role="alert" className="ui-notice ui-notice--error calc-history__error">
          <p className="calc-history__error-heading">
            <Icon name="alert" size={20} />
            {calcHistoryText.errorHeading}
          </p>
          <p>{state.moreError.message}</p>
        </div>
      )}
      {state.status === "loaded" && state.nextCursor !== null && (
        <button
          type="button"
          className="ui-button ui-button--secondary calc-history__more"
          disabled={loadingMore}
          onClick={handleMore}
        >
          <Icon name="open" size={20} />
          {calcHistoryText.moreLabel}
        </button>
      )}
    </section>
  );
}
