# ADR-0807: ポケモン画像はローカル変換+gateway の `/images/*` で配信する(MinIO は入れない)

- 状態: 採用(2026-10-03。P8-1 の範囲確定。spec-writer。実装は後続)
- 関連: requirements.md「ポケモン画像」・ADR-0001(画像: MinIO + gateway /assets)・ADR-0002(公式画像を Git に入れない)・ADR-0202(gateway ルーティング)・issue #286 所見1

## 背景
requirements.md は「MinIO に置き gateway の `/assets/` から配信、`manifest.json` で管理、`make assets` で取込」と書く。
現状は MinIO 未デプロイ・`GATEWAY_ASSETS_URL` 未設定で `/assets/*` は常に 404、`make assets` は終了コード 2 のスタブ、`tools/assets` は空。
Web・iOS にはタイプ色のエンブレムがすでにあり、画像が無くても全機能が動く。個人利用・学習目的で、費用はローカルのみ。

## 決定
1. **入力はユーザーが手元に置く画像**。取得ツールは作らない(権利。ADR-0002)。既定の入力 `data/generated/images/src/{key}.{png|jpg|jpeg|webp}`、
   出力 `data/generated/images/dist/`(どちらも `data/generated/` が .gitignore 済みで Git に載らない)。環境変数 `ASSETS_SRC`・`ASSETS_OUT` で変えられる。
   キーは `{図鑑番号4桁}-{フォルム3桁}`(openapi の `SpeciesKey` と同じ)。形式違反のファイル名・壊れた画像は警告してスキップし、残りを処理する(終了コード 0)。
   入力が空・存在しないときも成功し、空の manifest を書く。
2. **変換**: Node(`tools/assets`。画像ライブラリは sharp を版固定。Docker 不要)。WebP 2サイズ、長辺 128px(≤20KB)と 512px(≤100KB)、拡大しない・縦横比を保つ。
   ファイル名は内容 hash 付き `thumb/{key}.{hash8}.webp`・`detail/{key}.{hash8}.webp`。出力は決定的(同じ入力なら byte 同一。時刻・絶対パスを入れない)。
   入力から消した画像は出力からも消える。
3. **manifest.json**: `{ "version": 1, "images": { "0445-000": { "thumb": "thumb/0445-000.ab12cd34.webp", "detail": "detail/0445-000.ef567890.webp" } } }`。
   キーは昇順。パスは `/images/` からの相対。**manifest に無いキー = 画像なし = エンブレム**(クライアントは manifest が 404・不正でもエンブレムにする)。
4. **配信は MinIO を入れず、gateway が `GATEWAY_IMAGES_DIR`(任意。未設定なら画像なし)のディレクトリを `/images/*` で配信する**。
   - 理由: 画像は静的ファイルで書き込み・認可・バケット管理が要らない。MinIO は常駐の Pod・PVC・認証情報・アップロード手順が増えるだけで、学習したい対象(ADR-0001 のマイクロサービス)の本題ではない。
     ローカルのみ・費用ゼロ。公開時に CDN/オブジェクトストレージへ移す場合も、URL 規約(`/images/` + manifest の相対パス)は変えずに済む。
   - 配信するのは `manifest.json` と `.webp` だけ(他の拡張子・ドットファイル・ディレクトリ一覧・`..`・ディレクトリ外を指すシンボリックリンクは 404)。GET / HEAD のみ。
   - ヘッダ: ハッシュ付き WebP は `Cache-Control: public, max-age=31536000, immutable`、manifest は `no-cache`。`X-Device-Id` は課さない(`<img>` はヘッダを送れない)。CORS は assets と同じ。
   - `images` を予約セグメントに加える(未設定でも Web の SPA フォールバックが HTML を 200 で返さない。manifest として HTML を読ませない)。
   - **`/assets/*` の転送(ADR-0202)は変えない**(既存テストを保つ。将来 MinIO/CDN を使うときの口として残す)。issue #286 所見1の「`/images/` に移す」は、移行ではなく `/images/` の新設で満たす。
   - k3d: 変換出力を gateway Pod へ渡す(local overlay の hostPath を k3d の volume mount 越しに。無ければ空の emptyDir で起動し画像なし)は **実装フェーズの運用タスク**で、`make dev` は `GATEWAY_IMAGES_DIR` を指すだけで済む。
5. **契約**: openapi には載せない(静的規約)。`/images/` の URL 規約と manifest の形は本 ADR と `tools/assets/README.md` が正。型は Web・iOS が各自の小さな decode で持つ。
6. **表示側(Web・iOS)は別依頼**: 「manifest にキーがあれば `<img loading=lazy>`/AsyncImage で表示、無ければ既存のタイプ色エンブレム」。

## 受け入れ条件
- AC-A1: キー名の画像から thumb(長辺128)・detail(長辺512)の WebP と manifest ができる(容量上限・拡大しない・縦横比保持)。
- AC-A2: hash 付きファイル名・決定的な出力・消した入力は出力から消える・入力を変更しない・manifest に絶対パス等が無い。
- AC-A3: 入力が空・存在しなくても成功し、空 manifest。
- AC-A4: 形式違反・壊れた画像・0バイトはスキップ(理由付き・警告)して続行。
- AC-A5: 既定の入出力先は `data/generated/images/{src,dist}` で .gitignore 済み。
- AC-A6: `make assets` が変換を実行し、画像なしでも終了コード 0(スタブの終了コード 2 をやめる)。
- AC-I1〜I6(gateway): manifest/WebP の配信とヘッダ、画像なしは JSON 404(Web に流れない)、危険なパスの拒否、CORS、予約セグメント、`/assets` 不変。
- AC-X(最優先): 画像が無い現状で、gateway・Web・iOS の全機能が従来どおり動く(エンブレム)。

## テスト
`tools/assets/convert.test.mjs`(Node。小さな架空 PNG を生成。Docker・実画像・ネットワーク不要)、
`services/gateway/internal/httpapi/images_test.go`・`services/gateway/cmd/gateway/images_env_test.go`。

## 実装時の追記(2026-10-03。P8-1b)
- **実装**: 変換は `tools/assets/convert.mjs`(sharp 0.35.5 固定)。容量上限は品質を 85→15 に下げながら再エンコードして守り、収まらなければ `invalid_image` でスキップする。
  出力は manifest を最後に原子的に置き、thumb/・detail/ の古い hash や入力から消した画像のファイルは消す(それ以外のファイルには触らない)。
  gateway は `internal/httpapi/images.go`(`Config.ImagesDir`・`GATEWAY_IMAGES_DIR`)。ハッシュ付き(`.{hash8}.webp`)でない WebP は immutable にせず `no-cache` にする。
- **`make assets` の検査の置換**: `scripts/make-targets_test.sh` の「make assets は非0」は、スタブ(終了コード 2)が成功と数えられないための検査だった。
  実装後は「画像なしの一時ディレクトリ(`ASSETS_SRC`・`ASSETS_OUT`)で終了コード 0 になり空の manifest を書く」に置き換える(AC-A6)。検査を緩めたのではなく、守る対象が「未実装を成功と数えない」から「画像なしでも壊れない」に変わった。
- **`make dev`**: `data/generated/images/dist/manifest.json` があるときだけ `GATEWAY_IMAGES_DIR` を渡す(`DEV_IMAGES_DIR` で変更可)。無ければ画像なし。
- **k3d は未対応**: local overlay の gateway に hostPath を足すには k3d クラスタ作成時の volume mount が要り、`make up` の既存クラスタ・手順を壊す。
  今回は何も足さず(画像なしで全機能が動く)、k3d で画像を出す手順だけ `docs/runbooks/images.md` に書く。配線は別タスク。
- `make test-tools` は `tools/assets` で `npm ci` してからテストを走らせる(sharp が必要)。
