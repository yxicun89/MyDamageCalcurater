// P5-3c: お気に入り(手動ピン留め)の画面(ADR-0327)。一覧と削除だけを持つ(追加は計算画面)。
// 種族名の解決にマスタを使わない(行の見出しは label、null なら speciesKey)。API 専用の画面なので recordClient が無い
// (オフライン)ときは何も呼ばず案内だけを出す。
//
// 状態の作り(TeamScreen.tsx と同じ考え方):
//   - listFavorites は recordClient / reloadToken が変わるたびに1回。effect の cleanup で abort し、古い応答は捨てる
//   - 削除は2段階(確認 → 実行)。成功(404 の「もう無い」を含む)は手元の一覧から外し、一覧は取り直さない

import { useEffect, useRef, useState, type ReactNode } from "react";
import type { components } from "../api/openapi.gen";
import { CalcHistorySection } from "./CalcHistorySection";
import { PokemonIcon } from "../images/PokemonIcon";
import { favoritesScreenText } from "../i18n/favorites";
import { MAX_FAVORITES_PER_DEVICE, type RecordClient, type RecordError } from "../record/recordClient";
import "./FavoritesScreen.css";

type Favorite = components["schemas"]["Favorite"];

export interface FavoritesScreenProps {
  /** オンラインのときだけ App が渡す(ADR-0317 §2)。省略は「オフライン」。 */
  readonly recordClient?: RecordClient;
  /** 値が変わったら一覧を取り直す(計算画面での追加・端末データの削除の後。ADR-0327 §5)。 */
  readonly reloadToken?: number;
  /** ADR-0333: 「計算に使う」を押したとき。渡したときだけ各行にボタンを出す(省略は従来どおり)。 */
  readonly onUse?: (favorite: Favorite) => void;
}

type ListState =
  | { readonly status: "loading" }
  | { readonly status: "error"; readonly error: RecordError }
  | { readonly status: "loaded"; readonly favorites: readonly Favorite[] };

/** 削除の確認・送信・失敗の状態(操作中の1件だけ)。失敗後は確認を閉じ、行に失敗を出して再度削除できる。 */
interface DeleteState {
  readonly id: string;
  readonly phase: "confirming" | "submitting" | "failed";
  readonly error: RecordError | null;
}

const NOT_FOUND_CODE = "not_found";

function titleOf(favorite: Favorite): string {
  return favorite.label ?? favorite.individual.speciesKey;
}

export function FavoritesScreen({ recordClient, reloadToken, onUse }: FavoritesScreenProps): ReactNode {
  const [list, setList] = useState<ListState>({ status: "loading" });
  const [deleteState, setDeleteState] = useState<DeleteState | null>(null);

  // 取得し直す合図(口の切り替え・reloadToken)が変わったら、古い一覧を出し続けず読み込み中に戻す
  // (レンダー中の state 調整。react-hooks/set-state-in-effect を避ける)。
  const [prevSource, setPrevSource] = useState({ recordClient, reloadToken });
  if (prevSource.recordClient !== recordClient || prevSource.reloadToken !== reloadToken) {
    setPrevSource({ recordClient, reloadToken });
    setList({ status: "loading" });
    setDeleteState(null);
  }

  useEffect(() => {
    if (recordClient === undefined) {
      return;
    }
    const controller = new AbortController();
    void recordClient.listFavorites(controller.signal).then((result) => {
      if (controller.signal.aborted) {
        return;
      }
      setList(
        result.ok ? { status: "loaded", favorites: result.value } : { status: "error", error: result.error },
      );
    });
    return () => {
      controller.abort();
    };
  }, [recordClient, reloadToken]);

  // 削除の応答待ちの間にアンマウントされたら、応答で setState しない。
  const mountedRef = useRef(true);
  useEffect(() => {
    mountedRef.current = true;
    return () => {
      mountedRef.current = false;
    };
  }, []);

  if (recordClient === undefined) {
    return (
      <section className="favorites-screen" aria-label={favoritesScreenText.regionLabel}>
        <h2>{favoritesScreenText.regionLabel}</h2>
        <p role="status" className="ui-notice ui-notice--info favorites-screen__notice">
          {favoritesScreenText.offlineNotice}
        </p>
      </section>
    );
  }

  async function handleDelete(id: string): Promise<void> {
    if (recordClient === undefined || deleteState?.phase === "submitting") {
      return;
    }
    setDeleteState({ id, phase: "submitting", error: null });
    const result = await recordClient.deleteFavorite(id);
    if (!mountedRef.current) {
      return;
    }
    // 404 は「もう無い」= 目的は達成済みなので成功と同じに扱う(冪等)。
    if (result.ok || result.error.code === NOT_FOUND_CODE) {
      setList((current) =>
        current.status === "loaded"
          ? { status: "loaded", favorites: current.favorites.filter((favorite) => favorite.id !== id) }
          : current,
      );
      setDeleteState(null);
    } else {
      setDeleteState({ id, phase: "failed", error: result.error });
    }
  }

  const count = list.status === "loaded" ? list.favorites.length : null;

  return (
    <section className="favorites-screen" aria-label={favoritesScreenText.regionLabel}>
      <h2>{favoritesScreenText.regionLabel}</h2>
      {count !== null && (
        <p className="ui-badge favorites-screen__count">
          {favoritesScreenText.countLabel(count, MAX_FAVORITES_PER_DEVICE)}
        </p>
      )}
      {list.status === "loading" && (
        <p className="ui-notice ui-notice--loading favorites-screen__notice">
          {favoritesScreenText.loadingNotice}
        </p>
      )}
      {list.status === "error" && (
        <div role="alert" className="ui-notice ui-notice--error favorites-screen__error">
          <p>{favoritesScreenText.listErrorHeading}</p>
          <p>{list.error.message}</p>
        </div>
      )}
      {list.status === "loaded" && list.favorites.length === 0 && (
        <p className="ui-notice ui-notice--empty favorites-screen__notice">
          {favoritesScreenText.emptyNotice}
        </p>
      )}
      {list.status === "loaded" && list.favorites.length > 0 && (
        <ul aria-label={favoritesScreenText.listLabel} className="favorites-screen__list">
          {list.favorites.map((favorite) => {
            const title = titleOf(favorite);
            const rowState = deleteState?.id === favorite.id ? deleteState : null;
            const confirming = rowState?.phase === "confirming" || rowState?.phase === "submitting";
            const rowError = rowState?.error ?? null;
            const submitting = rowState?.phase === "submitting";
            return (
              <li key={favorite.id} className="ui-card favorites-screen__item">
                <div className="favorites-screen__heading">
                  <span className="favorites-screen__who">
                    <PokemonIcon speciesKey={favorite.individual.speciesKey} />
                    <span className="favorites-screen__title">{title}</span>
                  </span>
                  {favorite.calc === undefined && onUse !== undefined && (
                    <span className="ui-badge favorites-screen__hint">
                      {favoritesScreenText.attackerOnlyHint}
                    </span>
                  )}
                </div>
                <div className="favorites-screen__actions">
                  {onUse !== undefined && (
                    <button
                      type="button"
                      className="ui-button ui-button--primary favorites-screen__use"
                      onClick={() => {
                        onUse(favorite);
                      }}
                    >
                      {favoritesScreenText.useLabel(title)}
                    </button>
                  )}
                  {!confirming ? (
                    <button
                      type="button"
                      className="ui-button ui-button--secondary"
                      disabled={deleteState?.phase === "submitting"}
                      onClick={() => {
                        setDeleteState({ id: favorite.id, phase: "confirming", error: null });
                      }}
                    >
                      {favoritesScreenText.deleteLabel(title)}
                    </button>
                  ) : (
                    <div className="favorites-screen__confirm">
                      <p className="ui-notice ui-notice--error">
                        {favoritesScreenText.deleteConfirmNotice(title)}
                      </p>
                      <button
                        type="button"
                        className="ui-button ui-button--danger"
                        disabled={submitting}
                        onClick={() => {
                          void handleDelete(favorite.id);
                        }}
                      >
                        {favoritesScreenText.deleteConfirmLabel(title)}
                      </button>
                      <button
                        type="button"
                        className="ui-button ui-button--secondary"
                        disabled={submitting}
                        onClick={() => {
                          setDeleteState(null);
                        }}
                      >
                        {favoritesScreenText.deleteCancelLabel(title)}
                      </button>
                    </div>
                  )}
                </div>
                {rowError !== null && (
                  <div role="alert" className="ui-notice ui-notice--error favorites-screen__error">
                    <p>{favoritesScreenText.deleteErrorHeading}</p>
                    <p>{rowError.message}</p>
                  </div>
                )}
              </li>
            );
          })}
        </ul>
      )}
      <CalcHistorySection recordClient={recordClient} reloadToken={reloadToken} onUse={onUse} />
    </section>
  );
}
