// I-speed-4b(ADR-0610 §4): 素早さ画面の作り直し。ここは spec-writer のスタブ(空実装)。
// 実装者が SpeedScreen.*.test.tsx を通す形に書く。構成と契約は ADR-0610「Web 画面の具体」を正とする。
// 表(ADR-0608)・周辺パネル(ADR-0609)は画面に出さない(部品も削除済み)。

import type { SpeedClient } from "./speedClient";
import type { components } from "./speed.gen";

type Schemas = components["schemas"];

/**
 * 「この相手で連続技を計算」で計算画面の連続技へ渡す内容(ADR-0610 §4・§6 Q7)。
 * speed 画面はダメージ・確定数を出さない。今の振り(speed)があれば素早さの SP・性格・ランク・スカーフも渡す。
 */
export interface SpeedComboRequest {
  /** 攻撃側 = 自分のポケモン。 */
  readonly attackerPokemonId: string;
  /** 防御側 = 相手のポケモン。 */
  readonly defenderPokemonId: string;
  /** 「今の振り」を入れているときだけ。 */
  readonly speed?: {
    readonly sp: number;
    readonly nature: Schemas["NatureId"];
    readonly rank: number;
    readonly scarf: boolean;
  };
}

/** 画面の props(App.tsx が app/screens.tsx 経由で注入する。ADR-0604 §2)。 */
export interface SpeedScreenProps {
  readonly speedClient: SpeedClient;
  /**
   * 連続技の画面へ移る口。連続技の画面が画面レジストリ(ADR-0323)に載るまでは渡されず、
   * 渡されないときは「この相手で連続技を計算」のリンクを出さない(無効表示もしない)。
   */
  readonly onOpenCombo?: (request: SpeedComboRequest) => void;
}

/** 素早さ比較の画面(スタブ)。 */
// eslint-disable-next-line @typescript-eslint/no-unused-vars -- スタブ。実装者が props を使う形に書く。
export function SpeedScreen(_props: SpeedScreenProps) {
  return <div className="speed-screen" />;
}
