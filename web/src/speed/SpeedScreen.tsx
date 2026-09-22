// SP3: 素早さ比較の画面(ADR-0604 §4、docs/speed-design.md §5・§6)。
// 左 = 速い順の全体の表(speed-svc の tiers をそのまま描画)、右 = 自分のポケモン(preset / custom / raw)。
// 自分の実数値が左の表のどこに入るかを、同速の段の強調か、段の間の境界線で示す(ADR-0604 §1・§4)。
// 素早さの計算・並び・位置はすべて speed-svc が決める。Web は engine(WASM)・pokedex のマスタを使わない
// (ADR-0604 §5。ScreenProps.engine・master は構造的に無視する)。
//
// ここは SP3 のスタブ。画面の受け入れ条件は SpeedScreen.test.tsx が定める(失敗するテストが先。CLAUDE.md)。

import type { SpeedClient } from "./speedClient";
import "./SpeedScreen.css";

/** 画面の props(App.tsx が app/screens.tsx 経由で注入する。ADR-0604 §2)。 */
export interface SpeedScreenProps {
  readonly speedClient: SpeedClient;
}

/**
 * 素早さ比較の画面(ADR-0604 §4)。
 *
 * SP3 のスタブ: まだ何も描画しない。実装(implementer)は SpeedScreen.test.tsx が定める形
 * (マウント時の pokemon()・table()、段の描画と「同速」、タイプ色のエンブレム、
 * モード切り替えと position() の呼び直し、古い応答の無視、強調・境界線、エラー表示)を満たすように埋める。
 */
export function SpeedScreen(props: SpeedScreenProps): never {
  throw new Error(`not implemented: SpeedScreen (speedClient: ${typeof props.speedClient})`);
}
