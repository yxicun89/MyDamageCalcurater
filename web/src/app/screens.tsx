// 画面 ID → 描画するコンポーネントの対応(P4-10。ADR-0300 §1)。
// 画面を増やすレーン(タイプバランスの P4-12・素早さの SP3)は、app/routes.ts の SCREEN_ROUTES に1件足したうえで
// ここに1件足す。Record<ScreenId, ...> なので、どちらかの足し忘れは型エラーになる(App.tsx は触らない)。

import type { ComponentType } from "react";
import type { CalcEngine } from "../engine/types";
import type { MasterData } from "../master/types";
import { CalcScreen } from "../screens/CalcScreen";
import { ReverseScreen } from "../screens/ReverseScreen";
import type { ScreenId } from "./routes";

/** どの画面にも App が渡すもの(計算の差し替え口とマスタ。ADR-0300 §2・§3)。 */
export interface ScreenProps {
  readonly engine: CalcEngine;
  readonly master: MasterData;
}

/** 画面 ID ごとのコンポーネント。 */
export const SCREEN_COMPONENTS: Record<ScreenId, ComponentType<ScreenProps>> = {
  calc: CalcScreen,
  reverse: ReverseScreen,
};
