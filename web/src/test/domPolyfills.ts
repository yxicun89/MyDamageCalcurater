// テスト専用: jsdom に足りない DOM API の最小限のポリフィル。
// jsdom は window.AnimationEvent を実装していない。react-dom はこれを起動時(モジュール読込時)に
// 一度だけ機能検出し、無ければ animationend の合成イベントを「未対応」として扱ってベンダープレフィックス
// 名を探しにいくため、テストの fireEvent.animationEnd(要素) を onAnimationEnd ハンドラが受け取れなくなる
// (P4-8「動き」: 確定数バッジの弾み・攻守入れ替え・逆算の絞り込みはどれも animationend で終える)。
// この検出は react-dom の import 時に実行されるため、setupFiles の中でも import 文を持たない
// (import 文を持つファイルは、他の import を先に評価してしまい手遅れになる)この専用ファイルを、
// react-dom を import する前(vitest.config.ts の setupFiles の先頭)に読み込む必要がある。
if (typeof window !== "undefined" && typeof window.AnimationEvent === "undefined") {
  // Event で十分(react-dom はコンストラクタの有無だけを見る。type や bubbles は fireEvent 側が渡す)。
  window.AnimationEvent = window.Event as unknown as typeof AnimationEvent;
}
