## JD: 判定(判定レーン。設計は docs/judge-design.md。2026-09-22 ユーザー要望)
「ニトチャ+メイン技で素早さ抜ける+そのポケモンを倒せるか」を1回の入力で確認する。engine を直接呼び、pokedex-svc と calc-svc の公開 API だけに依存する(speed-svc には依存しない)。
- [x] JD0 基盤(ディレクトリ構成・pokedex-svc/calc-svc への HTTP クライアント・ヘルスチェック)
- [x] JD1 抜けるか+倒せるかの最小構成
- [x] JD2〜JD5 の範囲・順序をユーザーに確認
- [x] JD2 場の効果(トリックルーム・追い風)
- [x] JD3 複数の相手候補を一度に判定
- [x] JD4 相手の技を含めた返り討ち判定
- [x] JD5 Web の画面
- [x] issue #234 moveId/natureId の形式検証が無く、制御文字などが上流 URL にそのまま埋め込まれ、503
- [x] issue #213(重大度 high)上流(pokedex-svc/calc-svc)が遅いと judge は1リクエスト全体の期限を持たず
- [x] issue #329(重大度 low)SP 合計超過(67)の拒否を確かめる回帰テストが無く、`validateSP` の
- [x] issue #257(重大度 low)`services/judge/scripts/smoke.sh` が healthz しか叩かず、k3d 上で
- [x] issue #260
- [x] issue #260 のタイプバランス分
- [x] 判定の応答に calc-svc の「未対応」の印を中継する
- [x] issue 309 判定画面の技を select(種族の learnset)に、調整をプリセット(無振り・最速・攻撃特化・HB/HD特化)に、SP6欄・ランク5欄を「詳細」に畳み、検証エラーを欄ごとに aria-invalid+文言で出す(ADR-0711。ADR-0705 §5 を置き換え)。critic PASS・PR #480
- [x] issue #235 追加分 判定に status(状態異常)を足し、まひを素早さに反映(ADR-0712。契約・judge コア・Web の select・`*SpeedApplied` の paralysis)。特性・持ち物のデータ駆動(第2段)はデータレーン待ち
