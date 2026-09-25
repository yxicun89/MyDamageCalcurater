// 画面 ID → 描画するコンポーネントの対応(P4-10。ADR-0300 §1)。
// 画面を増やすレーン(タイプバランスの P4-12・素早さの SP3)は、app/routes.ts の SCREEN_ROUTES に1件足したうえで
// ここに1件足す。Record<ScreenId, ...> なので、どちらかの足し忘れは型エラーになる(App.tsx は触らない)。

import type { ComponentType } from "react";
import type { BalanceClient } from "../api/balanceClient";
import type { CalcEngine } from "../engine/types";
import { JudgeScreen } from "../judge/JudgeScreen";
import type { JudgeClient } from "../judge/judgeClient";
import type { MasterData, MasterSpeciesSearch } from "../master/types";
import { BalanceScreen } from "../screens/BalanceScreen";
import { CalcScreen } from "../screens/CalcScreen";
import { ReverseScreen } from "../screens/ReverseScreen";
import { SpeedScreen } from "../speed/SpeedScreen";
import type { SpeedClient } from "../speed/speedClient";
import { TeamScreen } from "../team/TeamScreen";
import type { TeamClient } from "../team/teamClient";
import type { MasterlessScreenId, ScreenId } from "./routes";

/**
 * どの画面にも App が渡すもの(計算の差し替え口・マスタ・balance API のクライアント。
 * ADR-0300 §2・§3、P4-12a: ADR-0303 §2)。計算画面・逆算画面は client を使わない(構造的に無視する)。
 * SP3(ADR-0604 §2): 素早さの画面は client(balance 専用)とは別のフィールド speedClient を使う。
 * JD5(ADR-0705 §1): 判定の画面も同じく judgeClient を専用のフィールドで受け取る。
 */
export interface ScreenProps {
  readonly engine: CalcEngine;
  readonly master: MasterData;
  readonly client: BalanceClient;
  readonly speedClient: SpeedClient;
  readonly judgeClient: JudgeClient;
  /** P5-5 PR-A1(ADR-0309 §2): 構築ビルダーの画面も同じく専用のフィールドで受け取る。 */
  readonly teamClient: TeamClient;
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
  // JD5(ADR-0705 §1): 判定。画面の中身は web/src/judge/ にある(レーンの境界)。
  judge: JudgeScreen,
  // P5-5 PR-A1(ADR-0309 §2): 構築ビルダー。画面の中身は web/src/team/ にある(レーンの境界)。
  team: TeamScreen,
};

/**
 * issue 308(ADR-0304 追記6): マスタを使わない画面(app/routes.ts の usesMaster: false)に渡す props。
 * ScreenProps から master と engine を取り除いただけ(SpeedScreenProps はどちらも要らないので、この型は
 * 元から両方を渡さない SpeedScreen にそのまま代入できる)。engine も除くのは、マスタの読み込みに
 * 失敗している間(critic指摘)は `resolvedEngines.online` が使えず offline へ静かに落ちるため
 * (ADR-0301 §4 が禁じる自動フォールバックに見えかねない)。マスタ不要の画面は engine も要らない
 * はずなので、そもそも渡さない。
 */
export type MasterlessScreenProps = Omit<ScreenProps, "master" | "engine">;

/**
 * マスタを使わない画面のコンポーネント(app/routes.ts の `MasterlessScreenId` = usesMaster: false の
 * 画面 ID だけを key に持つ。今は素早さだけ)。マスタの読み込みに失敗していても、この対応表にある画面は
 * master を渡さずに描画できる(App.tsx が `isMasterlessScreen` で選ぶ)。
 */
export const MASTERLESS_SCREEN_COMPONENTS: Record<
  MasterlessScreenId,
  ComponentType<MasterlessScreenProps>
> = {
  speed: SpeedScreen,
};
