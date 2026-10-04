## 2026-10-04: お気に入りに計算の入力全体(calc)・構築名の省略(API レーンから Web〈web-cf〉・iOS〈ios-6f〉・ダメージ計算〈ec〉の取りまとめへ)
Decision: usability-round2 の F-09・F-08 の API 側を、後方互換の契約変更として spec にした(実装済み(record と team は別コミット。2つの PR に分けてよい))。
- F-09(ADR-0228): `FavoriteInput`・`Favorite` に省略可の `calc`(`CalcRequest` への `$ref`。生成型も `CalcRequest` そのもの)を足した。`individual` は必須のまま。
  `calc` の無いお気に入り(旧い行・旧クライアントの作成)は応答に `calc` キーが無い。保存内容(既定値を補った JSON)が 4096 バイトを超える作成は 400 `invalid_input`。
- F-08(ADR-0229): `TeamInput.name` を省略可・nullable にした。省略・null・空・空白だけなら、サーバーが既定名「名称未設定」を入れて保存する(`createTeam`・`updateTeam` とも。置換で省けば前の名前は残らない)。応答の `Team.name` は必須の string のまま。
Reason: 利用者の要望「お気に入りをクリックしたらそのときのダメージ計算がすぐ出る」「構築名はいらない」。既存クライアントを壊さないよう、契約は任意項目の追加・必須の解除だけにした。
Impact:
- Web(web-cf): お気に入りのピン留めは、今の計算要求(計算 API に送った `CalcRequest` と同じ値)を `calc` に、一覧に出す個体(通常 `calc.attacker`)を `individual` に入れて `createFavorite` を呼ぶ。
  一覧で `calc` のあるお気に入りを選んだら、攻撃側・防御側・技・場・形式・急所を画面に戻し、そのまま `calcDamage`(`POST /api/calc`)を呼んで結果を出す。`calc` が無いものは従来どおり `individual` を読み込むだけ。
  計算が 400(`unknown_move` 等。マスタ・レギュレーションの変更で起こりうる)のときは、入力を戻したうえで計算の失敗として表示する(お気に入りは消さない)。
  構築名の入力欄を廃止する画面は `createTeam`/`updateTeam` で `name` を送らない(空文字や自前の既定名を作らない)。表示は応答の `Team.name`。
  生成型の変化: `TeamInput.name` が `string | null | undefined`。既存テストの偽クライアント2か所(`TeamScreen.mega.test.tsx`・`TeamScreen.members.test.tsx`)を `name ?? "名称未設定"` に追従済み(spec の PR に含む)。型を固定するテストは `web/src/record/favoritesContract.test.ts`(追記)・`web/src/team/teamNameContract.test.ts`(新規)。
- iOS(ios-6f): お気に入りの読み込み(計算画面の導線)で、`Favorite.calc`(`Components.Schemas.CalcRequest?`)があれば計算の入力全体を復元し、そのまま `calcDamage` を呼ぶ。無ければ従来どおり `individual` を読み込む。
  ピン留めは `FavoriteInput(label:individual:calc:)` で今の計算要求を `calc` に入れる。iOS の構築はローカル保存で team-svc の `TeamInput` を使っていないので、構築名の変更で直すコードは無い(同期を作るときは ADR-0229 に従う)。
  型を固定するテストは `FavoritesContractTests`(2件追記)・`TeamNameContractTests`(新規)。
- ダメージ計算(ec)の取りまとめ(ec レーンへ): 契約・実装は PR で統合される。docs/plan に F-08・F-09 の行は無く、`docs/usability-round2.md` の状態列の更新は ec レーンが行う(API レーンは編集しない)。record-svc の実装は ADR-0228、team-svc の実装は ADR-0229 の受け入れ条件。
