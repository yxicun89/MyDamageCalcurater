## Judge
Lane: 判定(素早さ×ダメージ連動。`services/judge/`・`web/src/judge/`。どの AI が進めてもよい)
Active: issue 271(判定画面の未対応の印。ADR-0713。実装・テスト完了、critic・PR 待ち)。issue 235 追加分(status・まひ。ADR-0712。実装完了、critic・PR 待ち)。issue 309(判定画面の技 select・調整プリセット・「詳細」・欄ごとの検証エラー。ADR-0711。実装・テスト完了、critic・PR 待ち)
Branch: 次は main から feat/judge-<名前> を切る(作業ディレクトリ ~/MyDamageCalcurater-judge)
Status: JD0(基盤。PR #92)・JD1(判定API本体。PR #118)・JD2(場の効果。PR #127)・JD3(複数の相手候補。PR #143)・
JD4(返り討ち判定。PR #169)・JD5(Web の画面。PR #182。ADR-0705)まで全段階が完了。`POST /api/judge/v1/outspeed-and-ko`
は自分1体対相手1〜6体の素早さ判定・場の効果(トリックルーム・追い風)・返り討ち判定まで対応し、`web/src/judge/`
(`/judge` タブ)から呼べる。技はID自由入力(ADR-0304 §3の技一覧APIの欠落を踏襲)、相手側の追い風は全候補共通の
1チェックボックス(ADR-0703 §5)、送信ボタンでのみ呼ぶ(1回で上流最大27回)。
Status(追記): 2026-10-02 issue 309 `web/src/judge/` を変更: 技は ID 自由入力から種族の learnset の select へ(ADR-0705 §5 を置き換え)、調整プリセット(無振り・最速・攻撃特化・HB/HD特化)、SP6欄・ランク5欄は「詳細」に畳む、検証エラーは欄ごとに aria-invalid+文言。ADR-0711。
Status(追記): 2026-10-01 issue #258 judge の GitOps(gitops overlay・Argo CD Application・image 公開スクリプト。ADR-0709)を実装。
実クラスタへの適用(`judge-argocd-app`・registry push・sync)は人間確認待ちで未実施。
Status(追記): 2026-10-03 issue #288(ダブル)は、データレーン(PR #534)・API レーン(PR #536・#510)で技の対象と engine のダブル補正が main に入ったため、判定画面に「ダブル」を戻した。
Status(追記): 2026-10-02 issue #235 第1段(素早さに反映した補正・反映していない入力を応答と判定画面に出す。ADR-0710)を実装。
第2段(特性・持ち物の素早さ補正のデータ駆動)はデータレーンへの依頼(DECISIONS.md)待ち。#258 は PR #419・#449 で overlay まで統合、Argo CD への登録・sync は未実施。
Status(追記): 2026-10-03 issue #235 追加分: 判定に `status`(状態異常)を足し、まひを素早さに反映(ADR-0712。連結・丸めのあと floor(x×50/100)、`*SpeedApplied` の末尾に `paralysis`、全 status を calc-svc へ転送)。Web に「状態異常」select。
Next: issue 271 判定画面の未対応の印(ADR-0713)の critic と PR。issue #235 追加分の critic と PR。issue 309 の critic レビューと PR(共通部品化〈MoveSelect・プリセット選択〉と SpeciesSearchField の aria-invalid 対応は別タスク提案。ADR-0711)。以降は新規要望待ち。軽微な積み残しは解消済み(2026-09-25。`attacker`単数の`Individual`にも`defenders`候補と
同じ大文字小文字厳密なキー検査〈`individualWireKeys`〉を適用。PR #342 main 統合済み)。
issue #234(moveId/natureId の形式検証。ADR-0706)も解消(2026-09-25。critic 2ラウンド。PR #365 main 統合済み):
名前付きスキーマ `MoveId`/`NatureId`(pattern `^[a-z0-9]+(-[a-z0-9]+)*$`・maxLength 64)を契約に追加し、
`outspeed.go` の3箇所(attacker moveId・natureId共有・候補moveId)で上流呼び出し前に検査、
`pokedex.go` は `url.PathEscape` で二重の守り。`web/src/judge/judge.gen.ts` も手動再生成(ADR-0705 §2)。
issue #213(重大度 high。リクエスト全体の期限。ADR-0707)も解消(2026-09-25。critic PASS〈1回目〉。PR #370 main 統合済み):
`JUDGE_REQUEST_TIMEOUT`(既定12秒。`writeTimeout`=15秒未満を起動時検証)を新設し、`outspeedAndKo` の
先頭で ctx を1回だけ `context.WithTimeout` でラップして以降の上流呼び出しに使い回す(呼び出し順序・
逐次打ち切り規約〈ADR-0703 §3〉は無変更)。`internal/client` は無変更(`http.NewRequestWithContext` の
既存の context 統合だけで「進行中呼び出しの中断」「未着手呼び出しの即時失敗」の両方が成立)。
上流が遅くても期限内に503 JSONを返すようになり、クライアントが空応答(HTTP 000)を受け取ることが無くなった。
issue #329(重大度 low。SP合計67の境界値テスト欠落)も解消(2026-09-25。テストのみ・実装無変更。PR #371 main 統合済み):
`validateSP` の合計超過検査の既存テストが境界〈67〉から遠い(96)ため、
`> engine.MaxSPTotal` を `+1` する退行を検出できなかった。境界値(合計66は受け付け・67は拒否)の
テストを `internal/judge`・`internal/httpapi` 両方に追加し、mutation test で実際に検出できることを確認。
issue #257(重大度 low。smoke.sh が healthz のみ)も解消(2026-09-25。テスト用スクリプトのみ。PR で main へ):
gateway smoke の ID取得部分を流用し `POST /api/judge/v1/outspeed-and-ko` の 200(hits含む)・
ヘッダなし400・未知speciesKey 422・7候補400 を実クラスタ(k3d-pokecalc、実データ)で確認済み。
`Makefile` に `API_URL` を追加、README の古い「JD0完了」表記も修正。
iOS版JD5は要望が出たら判断(ADR-0705 却下案)
Status(追記): 2026-10-03、P5-5 の Showdown 形式の変換部を `web/src/team/showdownFormat.ts` に実装(ADR-0310。ブランチ `feat/web-team-showdown-format`)。`parseShowdownTeam(text, master)` / `exportShowdownTeam(members, master)` の純粋関数。`EVs:` は SP をそのまま読み書き。画面への配線は Web レーンの P5-5b(`parse` の members を構築へ、issues を一覧表示)。
