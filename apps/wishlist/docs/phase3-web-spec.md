# wishlist フェーズ3 Web(PWA)の受け入れ条件: 目安価格の表示

対象: `apps/wishlist/web/`。仕様は [../CLAUDE.md](../CLAUDE.md) §3(詳細シートの 3〜5)・§6(取得失敗でもエラー画面にせず「最終取得: ○日前」・リンクは常に出す)・§10(オフライン)、
API は [../api/openapi.yaml](../api/openapi.yaml) と [phase3-api-spec.md](phase3-api-spec.md)。書き方・画面の契約は [phase1-web-spec.md](phase1-web-spec.md) に合わせる。
テストは `cd apps/wishlist/web && npm test`。実装前は失敗する(`src/lib/price.ts` と `api.ts` の 2 メソッドはスタブ、Sheet は未対応)。

## 決めたこと(仕様に書かれていなかった部分)

- 取得: 詳細シートを開いたとき `GET estimates` を 1 回。サイト行のリンクは estimates の成否に関係なく(今までどおり `resolveSiteLinks` から)出す。
- **サマリ**(`role="status"`。どれも 1 つの要素の文字):
  - 値あり: `だいたい ¥3,000〜¥4,500 で買えそう(10/3 時点)`。`summary_mid` が null なら `だいたい ¥3,000〜 で買えそう(10/3 時点)`。
    日付は `summary_fetched_at` を **JST の M/D**(ゼロ詰めなし)。`summary_fetched_at` が無ければ `(… 時点)` を付けない
  - 目安なし(`sites` が空、または `summary_low` が null): `まだ価格情報はありません`(時点なし)
  - 通信できない(estimates が `ApiError.code === "network"`、または `navigator.onLine === false`): `オフライン`(既存)
  - `refreshing: true` の間は、上のどれにも `更新中…` を添える(値が無くても出す)。表示は文字だけ(スピナー・`progressbar`・アニメーションなし)
- **サイト行**: `ul[aria-label="サイト"]`(`role=list` name「サイト」)の各 `<a target="_blank" rel="noopener">`。ジャンルの `site_ids` 順(enabled=false 除外は今までどおり)。
  estimates に出てこないサイト(link_only など)・ジャンルに無いサイトの目安は今までどおりリンクだけ
  - `status=ok`: サイト名・`¥low〜¥mid`(mid が null なら `¥low〜`)・`N件`(`count`)・在庫(`in_stock_count > 0` → `在庫あり`、0 → `在庫なし`)
  - `status=failed`: 前回の値(`low` があれば上と同じ表示)に `最終取得: N日前`(`fetched_at` と現在の **JST の暦日の差**。同じ日なら `最終取得: 今日`。月単位にしない)。
    前回値が無い(`low` が null)なら `取得できませんでした`(`最終取得` は出さない)。エラー表示(`alert`)にはしない
  - `status=no_result`: `見つかりません`(価格は出さない)
- **再取得(ポーリング)**: `refreshing: true` の応答を受けたら、`setTimeout` で **5000ms 後**にもう一度 GET する(連鎖する `setTimeout`。`setInterval` は不可)。
  再取得は最大 6 回(最初と合わせて GET は最大 7 回)。`refreshing: false` で止まる。打ち切ったときは `更新中…` を消し、取得済みの値は残す。
  シートを閉じたら予約を `clearTimeout` する(閉じたあとの GET なし)。通信失敗・offline のときは予約しない
- **更新ボタン**: シートに `button`「更新」。押すと `POST estimates/refresh`(202。本文は GET と同じ形)を呼び、本文を反映して(`refreshing: true` なので `更新中…`)上の再取得に入る。
  オフライン(サマリが「オフライン」)のあいだは `disabled`
- **参考外**: `suspicious_count` の合計が 0 のときは「参考外」の文字を出さず、listings も取らない。1 以上なら折りたたみ(`<details><summary>参考外 N件</summary>` を推奨。`aria-expanded` でも可。N は合計)。
  **開いたときに** `GET listings`(`site_id` なし)を呼び、`suspicious_reasons` が空でない出品だけを `ul[aria-label="参考外の出品"]` に並べる(API の並びのまま)。
  各 `<li>`: 画像(`<img alt=タイトル>`)・タイトル・価格(`¥300`)・理由(`商品名が一致しない` / `安すぎる` / `下限価格未満`)・出品へのリンク(`<a href target="_blank" rel="noopener">`)
  - `url` が http(s) でなければリンクにしない。`image_url` が http(s) でなければ `<img>` を出さない(`javascript:`・`data:` を DOM に入れない)
- **金額・日付・理由の整形は `src/lib/price.ts` の純粋関数**(`formatYen`・`formatRange`・`formatJstDate`・`formatAge`・`reasonLabel`・`isHttpUrl`)。現在時刻は `formatAge` の引数で受ける(Sheet は `new Date()` を渡す)。
- 常時動くアニメーションは入れない(更新中は文字だけ。既存の AC-PWA-02 が CSS の `infinite` を検査する)。
- 参考外の listings の取得に失敗したとき(未テストの既定案): 折りたたみの中に短い文を出す(`role="alert"` は使わない。シート全体は壊さない)。

## テスト支援(spec-writer が追加)

- `src/test/fakeApi.ts`: `api.estimates(itemId, nth)`(GET の本文。既定は目安なし)・`api.listings`・`api.failEstimates`(GET estimates だけ通信失敗)。
  `POST .../estimates/refresh` は 202 で `{...api.estimates(...), refreshing: true}`、`GET .../listings` は `site_id` で絞る。既存の振る舞いは変えていない。
- `src/test/pollClock.ts`: 偽の時計。**2000ms 以上の `setTimeout` だけ**を横取りして手動で進める(`advance(ms)`・`pendingCount()`)。
  RTL は vitest の `vi.useFakeTimers` だと内部の drain で固まるため使わない。長押し(500ms)・waitFor(1000ms)・user-event の 0ms は本物のまま。

## 受け入れ条件とテスト

| ID | 条件 | テスト |
|---|---|---|
| AC-EST-01 | シートを開くと `GET estimates` を 1 回(Bearer 付き)。サイト行のリンクは出る | `src/App.estimates.test.tsx` 「AC-EST-01 …」 |
| AC-EST-02 | サマリ: 値あり(JST の日付)/ mid が null / 目安なし(sites 空・summary_low null)/ refreshing で「更新中…」・文字だけ | 同 「AC-EST-02 …」4 件(オフラインは既存 AC-OFF-02) |
| AC-EST-03 | サイト行: ジャンル順・名前・low〜mid・件数・在庫あり/なし・`<a target=_blank rel=noopener>`・ジャンル外のサイトは出さない / 目安なしはリンクだけ / failed は前回値と最終取得(JST の暦日・今日)/ failed で前回値なし / no_result | 同 「AC-EST-03 …」5 件 |
| AC-EST-04 | refreshing:true は 5 秒後に GET し直し、false で止まる(4999ms では GET しない)/ false なら予約なし / 最大 6 回で打ち切り「更新中…」を消す / 閉じたら予約を取り消す | 同 「AC-EST-04 …」4 件 |
| AC-EST-05 | 「更新」で POST refresh(202)→「更新中…」→ 5 秒後に GET → 止まる | 同 「AC-EST-05 …」 |
| AC-EST-06 | 参考外: 合計 0 なら出さない・listings も取らない / 「参考外 N件」は折りたたみで開くまで取らない / 開くと GET listings・理由のある出品だけ・画像・タイトル・価格・理由・リンク / http(s) 以外はリンク・画像にしない | 同 「AC-EST-06 …」4 件 |
| AC-EST-07 | estimates が通信失敗: 「オフライン」・リンクは出る・alert なし・「更新」は disabled・再取得の予約なし | 同 「AC-EST-07 …」 |
| AC-EST-08 | 整形の純粋関数(金額の桁区切り・range・JST の M/D(年またぎ・UTC との日付差)・N日前(JST の暦日・未来)・理由の文言・http(s) 判定) | `src/lib/price.test.ts`(テーブル駆動) |
| AC-API-08 | `refreshEstimates`(POST・202 でも本文を返す)・`listListings`(GET・配列だけ・`site_id` クエリ)・失敗は ApiError(通信失敗は network) | `src/lib/api.test.ts` 「createApiClient(フェーズ3)」 |
| AC-EST-09 | 常時アニメーションなし | 既存 AC-PWA-02(`src/pwa/pwaFiles.test.ts`)と AC-EST-02 の `progressbar` なし |

既存テストの期待は変えていない(AC-SHEET-05 の「まだ価格情報はありません」・AC-OFF-02 の「オフライン」はそのまま通る)。

## implementer への注意

- テストを変えて通さない。契約が不都合なら spec-writer へ戻す。
- スタブ: `src/lib/price.ts`(`notImplemented` ごと消す)と `src/lib/api.ts` の `refreshEstimates`・`listListings`(`throw`)。型は `src/api/types.ts` に `SiteEstimate`・`Listing`・`SuspiciousReason` を足した(別名)。`schema.d.ts` は生成物。
- ポーリングは **連鎖する `setTimeout`(5000)**。`setInterval`・`requestAnimationFrame` では偽の時計が効かずテストが落ちる。`useEffect` のクリーンアップで `clearTimeout` し、閉じたあと・アンマウント後に `setState` しない。
- 「更新」の POST の応答と GET の応答が前後しても、古い応答で新しい状態を上書きしない(世代番号などで)。
- 在庫の表示は `in_stock_count` から決める(`status` と独立)。`failed` でも前回の件数・在庫は出す。
- 既存の Sheet の `<ul className="site-list">` に `aria-label="サイト"` を付ける。既存テストは `getAllByRole("link")` でシート全体のリンクを数えるため、参考外のリンクは開いたときだけ DOM に出すこと(今のテストは閉じた状態でしか数えない)。
- 参考外の画像・リンクは `isHttpUrl` を通した場合だけ出す。CSS は `src/` 内の `.css` に書き、`@keyframes` は常時動かさない。
- 検証: `npm run typecheck && npm run lint && npm test`(web/ 内)。`npm test` は偽の時計のテストを含むが実時間では数秒。
