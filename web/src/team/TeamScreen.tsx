// P5-5 PR-A1: 構築ビルダーの画面(ADR-0309 §4)。
// この段階で扱うのは **一覧・新規作成(名前だけ・メンバーは空)・名前変更・削除** まで。
// メンバー(種族・技・持ち物・特性・性格・SP・テラスタイプ)の編集は PR-A2、
// Showdown 形式の入出力は判定レーンの web/src/team/showdownFormat.ts(別担当)。
//
// 状態の作り(SpeedScreen.tsx・BalanceScreen.tsx と同じ考え方):
//   - list() はマウント時に1回だけ呼び、cancelled フラグで古い応答を捨てる
//   - create()/update()/remove() が成功したら、応答の Team で手元の一覧を書き換える(list を呼び直さない)
//   - 一覧の読み込みに失敗しても、新規作成のフォームは先に使える(ADR-0309 §4)
//
// **このファイルは spec-writer 工程の骨格**(props の型だけ)。中身は implementer が
// team/TeamScreen.test.tsx を緑にする形で実装する。

import type { ReactNode } from "react";
import type { TeamClient } from "./teamClient";

/** パーティの上限(api/openapi.yaml の TeamInput.members の maxItems)。 */
export const MAX_TEAM_MEMBERS = 6;

/** 構築名の長さの上限(api/openapi.yaml の TeamInput.name の maxLength)。 */
export const MAX_TEAM_NAME_LENGTH = 50;

/**
 * 画面の props(App.tsx が app/screens.tsx 経由で注入する。ADR-0309 §2)。
 * PR-A1 の一覧・作成・名前変更・削除は master も engine も使わないが、ルート表では
 * `usesMaster: true` にしてある(PR-A2 のメンバー編集で必要になるため。ADR-0309 §1)。
 */
export interface TeamScreenProps {
  readonly teamClient: TeamClient;
}

/**
 * 構築ビルダーの画面(ADR-0309 §4)。
 *
 * **未実装**: spec-writer 工程では骨格だけを置く(team/TeamScreen.test.tsx が正)。
 * 何も描かない実装にすると「無いことを確かめるテスト」が偽の緑になりうるので、
 * 実装前は描画しようとすると必ず失敗するようにしておく。
 */
export function TeamScreen({ teamClient }: TeamScreenProps): ReactNode {
  // 骨格のみ(implementer が teamClient を使って一覧・作成・名前変更・削除を実装する)。
  throw new Error(
    `TeamScreen は未実装(P5-5 PR-A1 の implementer 工程で実装する。teamClient: ${typeof teamClient})`,
  );
}
