// JD5: 判定の画面(ADR-0705 §4〜§8、docs/judge-design.md §3 JD5)。
// 自分のポケモン1体(使う技込み)と相手候補1〜6件を入力し、judge-svc の
// POST /api/judge/v1/outspeed-and-ko を「判定する」で1回だけ呼ぶ(ADR-0705 §7)。
// 判定(素早さ・行動順・双方向の確定数)は judge-svc が決める。Web は engine(WASM)で計算し直さず、
// 「勝ち / 負け」にも丸めない(ADR-0700 §6-1・ADR-0704 §3 の立場を画面でも保つ)。
// master / masterSearch は**入力補助にだけ**使う(種族・性格・特性・持ち物の選択肢。ADR-0705 §5)。
// 技は ID の自由入力(ADR-0304 §3: ID から技を引く公開 API がまだ無い)。
//
// **この時点では未実装のスタブ**(JD5 は spec-writer が受け入れ条件とテストを先に書く段階)。
// 実装は JudgeScreen.test.tsx を通す形で implementer が入れる。
// props・定数は契約(ADR-0705 §4・受け入れ条件2)の正として先に置く。

import type { MasterData, MasterSpeciesSearch } from "../master/types";
import type { JudgeClient } from "./judgeClient";

/** 画面の props(App.tsx が app/screens.tsx 経由で注入する。ADR-0705 §1)。 */
export interface JudgeScreenProps {
  readonly judgeClient: JudgeClient;
  /** 入力補助のマスタ(性格・特性・持ち物の選択肢、種族の一覧。判定の計算には使わない)。 */
  readonly master: MasterData;
  /**
   * 種族を都度引く口(ADR-0304 §1)。`master.capabilities.speciesList` が false のとき、
   * ポケモンのドロップダウンの代わりに SpeciesSearchField でこの口を使う。
   */
  readonly masterSearch?: MasterSpeciesSearch;
}

/** 相手候補の上限(契約の defenders は 1〜6 件。ADR-0703 §1)。 */
export const MAX_DEFENDERS = 6;

/**
 * 判定の画面(ADR-0705 §4)。
 * **未実装**: JudgeScreen.test.tsx の受け入れ条件を満たす実装を implementer が入れる。
 */
// eslint-disable-next-line @typescript-eslint/no-unused-vars -- 未実装のスタブ(implementer が props を使う)
export function JudgeScreen(props: JudgeScreenProps): never {
  throw new Error("JudgeScreen is not implemented yet (JD5)");
}
