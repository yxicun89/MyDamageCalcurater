// 画面 ID → 描画するコンポーネントの対応(P4-10。ADR-0300 §1)。
// 画面を増やすレーン(タイプバランスの P4-12・素早さの SP3)は、app/routes.ts の SCREEN_ROUTES に1件足したうえで
// ここに1件足す。Record<ScreenId, ...> なので、どちらかの足し忘れは型エラーになる(App.tsx は触らない)。

import type { ComponentType } from "react";
import type { BalanceClient } from "../api/balanceClient";
import type { CalcEngine } from "../engine/types";
import type { MasterData, MasterSpeciesSearch } from "../master/types";
import { BalanceScreen } from "../screens/BalanceScreen";
import { CalcScreen } from "../screens/CalcScreen";
import { ReverseScreen } from "../screens/ReverseScreen";
import { SpeedScreen } from "../speed/SpeedScreen";
import type { SpeedClient } from "../speed/speedClient";
import type { ScreenId } from "./routes";

/**
 * どの画面にも App が渡すもの(計算の差し替え口・マスタ・balance API のクライアント。
 * ADR-0300 §2・§3、P4-12a: ADR-0303 §2)。計算画面・逆算画面は client を使わない(構造的に無視する)。
 * SP3(ADR-0604 §2): 素早さの画面は client(balance 専用)とは別のフィールド speedClient を使う。
 */
export interface ScreenProps {
  readonly engine: CalcEngine;
  readonly master: MasterData;
  readonly client: BalanceClient;
  readonly speedClient: SpeedClient;
  /**
   * P4-16b(ADR-0304 A-10): 種族を都度引く口。App は今選ばれているマスタの取得口が検索付きのとき
   * (`isSearchableMasterSource`)だけ渡す。`master.capabilities.speciesList` が false の画面は、
   * ポケモンのドロップダウンの代わりにこの口で検索する。素早さの画面は master 自体を使わない。
   */
  readonly masterSearch?: MasterSpeciesSearch;
}

/** 画面 ID ごとのコンポーネント。 */
export const SCREEN_COMPONENTS: Record<ScreenId, ComponentType<ScreenProps>> = {
  calc: CalcScreen,
  reverse: ReverseScreen,
  balance: BalanceScreen,
  // SP3(ADR-0604 §2): 素早さ比較。画面の中身は web/src/speed/ にある(レーンの境界)。
  speed: SpeedScreen,
};
