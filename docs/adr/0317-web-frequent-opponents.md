# ADR-0317: 計算画面の「よく計算する相手」チップ(P5-5c。Web)

- 状態: 採用
- 日付: 2026-10-02
- レーン: Web
- 関連: ADR-0209(record の保持・集計)、ADR-0212(計算イベントの非同期発行)、ADR-0210(端末 ID 境界)、
  ADR-0309(teamClient の流儀)、ADR-0304(オンライン/オフラインのマスタ)、docs/design.md「画面: ダメージ計算」、
  requirements.md §2、api/openapi.yaml `GET /api/record/frequent-opponents`
- 番号: 0316 は別 PR 予約済み

## 背景

record-svc は端末の計算イベントを集計し、`GET /api/record/frequent-opponents` が防御側の種族 key を
スコア降順で返す(名前・タイプは返さない)。計算イベントは非同期保存で、record が落ちても計算は成功する
(絶対ルール5)。よって Web は、この取得の失敗を計算画面の失敗にしてはならない。

## 決定(テストが具体形を固定する)

1. **recordClient**(`web/src/record/recordClient.ts`): teamClient と同じ流儀(`createRecordClient({baseUrl, fetch, ids})`)。
   例外を投げず `RecordResult<T>`(ok/error の判別 union)で返す。`listFrequentOpponents(signal?)` は
   `GET ${baseUrl}api/record/frequent-opponents?limit=5` を `X-Device-Id`・`X-Session-Id` 付きで呼ぶ(端末 ID はパスに含めない)。
   limit は定数 `FREQUENT_OPPONENTS_LIMIT`(5)。通信失敗・JSON でない・本文の形が不正は `record_unavailable`。
   サーバーの Error 封筒(503 `store_unavailable` 等)は code・message をそのまま運ぶ。応答は並べ替えない。
2. **表示条件**: CalcScreen の任意 prop `recordClient`。App は**計算モードがオンラインのときだけ**渡す
   (オフラインは /api に一切触れない。offline.spec.ts の前提を守る)。省略・失敗・0 件・マスタで引けない key だけのとき、
   チップの group を出さず、`role="alert"` も出さない(黙って非表示)。計算・入力・結果には影響しない。
3. **取得は画面の表示時に 1 回**(マウント時。入力・選択のたびには呼ばない)。アンマウントで AbortSignal を abort する。
   取得の完了を計算は待たない。
4. **名前解決**: 種族 key はマスタで引く。一覧マスタ(speciesList)なら `master.species`、検索マスタなら
   `masterSearch.resolveSpecies(key)`(全件を最初に解決。上限 5 件)。引けない key は黙って落とす。表示名は `nameJa`。
5. **UI**: `role="group"` の名前「よく計算する相手」の中に `button` を並べる(名前 = 種族の日本語名。サーバーの順)。
   押すと防御側にその種族をセットする(攻撃側・技・持ち物の既存の扱いは、防御側セレクトで選んだときと同じ)。
   配置は design.md のとおり結果の下。色・余白はデザイントークンのみ。文言は `i18n/ja.ts`。
6. **端末の境界**: 他端末のデータはサーバー側で混ざらない。Web は端末 ID をヘッダーで付けるだけ(ADR-0210)。

## 対象外

- 履歴一覧(API 無し)、端末データの削除 UI(P5-5d)、逆算画面のチップ(必要になったら別 PR)、スコア・件数の表示。
