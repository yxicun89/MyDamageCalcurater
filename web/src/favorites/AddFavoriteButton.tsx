// P5-3c: 計算画面の「攻撃側をお気に入りに追加」(ADR-0327 §2)。計算とは独立(絶対ルール5):
// 追加の成否・pending・reject は、このボタン周りの表示だけに出し、計算結果には触れない。

import { useEffect, useRef, useState, type ReactNode } from "react";
import type { components } from "../api/openapi.gen";
import { favoritesCalcText } from "../i18n/favorites";
import type { RecordClient, RecordError } from "../record/recordClient";

type FavoriteInput = components["schemas"]["FavoriteInput"];

export interface AddFavoriteButtonProps {
  readonly recordClient: RecordClient;
  /** 追加する内容。攻撃側の種族が決まるまで(または性格を決められないとき)は null(ボタンは disabled)。 */
  readonly input: FavoriteInput | null;
  /** 追加に成功した(201 でも 200 でも)ことを知らせる。 */
  readonly onAdded?: () => void;
}

type AddState =
  | { readonly status: "idle" }
  | { readonly status: "submitting" }
  | { readonly status: "added" }
  | { readonly status: "already" }
  | { readonly status: "error"; readonly error: RecordError };

export function AddFavoriteButton({ recordClient, input, onAdded }: AddFavoriteButtonProps): ReactNode {
  const [state, setState] = useState<AddState>({ status: "idle" });
  // 追加する個体が変わったら、前の個体への結果表示(追加しました等)を消す(送信中の応答は届いたときに表示する)。
  const inputKey = input === null ? "" : JSON.stringify(input);
  const [prevInputKey, setPrevInputKey] = useState(inputKey);
  if (inputKey !== prevInputKey) {
    setPrevInputKey(inputKey);
    if (state.status !== "submitting") {
      setState({ status: "idle" });
    }
  }
  // 同じ描画内の二重押しも弾く(state の disabled だけでは次の描画まで間がある)。
  const submittingRef = useRef(false);
  const mountedRef = useRef(true);
  useEffect(() => {
    mountedRef.current = true;
    return () => {
      mountedRef.current = false;
    };
  }, []);

  async function handleAdd(): Promise<void> {
    if (input === null || submittingRef.current) {
      return;
    }
    submittingRef.current = true;
    setState({ status: "submitting" });
    let next: AddState;
    try {
      const result = await recordClient.createFavorite(input);
      if (result.ok) {
        next = { status: result.value.created ? "added" : "already" };
        onAdded?.();
      } else {
        next = { status: "error", error: result.error };
      }
    } catch {
      // RecordClient は例外を投げない契約だが、計算を巻き込まないよう握りつぶして失敗表示にする。
      next = { status: "error", error: { code: "record_unavailable", message: "" } };
    } finally {
      submittingRef.current = false;
    }
    if (mountedRef.current) {
      setState(next);
    }
  }

  return (
    <div className="calc-screen__favorite">
      <button
        type="button"
        disabled={input === null || state.status === "submitting"}
        onClick={() => {
          void handleAdd();
        }}
      >
        {favoritesCalcText.addLabel}
      </button>
      {(state.status === "added" || state.status === "already") && (
        <p role="status" className="calc-screen__favorite-notice">
          {state.status === "added" ? favoritesCalcText.addedNotice : favoritesCalcText.alreadyNotice}
        </p>
      )}
      {state.status === "error" && (
        <div role="alert" className="calc-screen__favorite-error">
          <p>{favoritesCalcText.addErrorHeading}</p>
          {state.error.message !== "" && <p>{state.error.message}</p>}
        </div>
      )}
    </div>
  );
}
