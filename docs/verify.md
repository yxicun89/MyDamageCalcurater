# 動作確認の手順(Web・iOS)

上から順に実行する。各コマンドの下の「→」が成功の見え方。
入口は 1 つ: **ブラウザ・iOS とも `http://localhost:8080`**(k3d → Traefik → gateway)。
1-1(クラスタ作成とマスタ投入)は破壊的な操作を含むので、人間が自分のターミナルで実行する(Claude Code では先頭に `!` を付ける)。

## 0. 前提(初回だけ)

```sh
cd "$(git rev-parse --show-toplevel)"
make doctor
make web-install
make web-e2e-install
xcode-select -p
```
→ doctor が不足ツールを報告しない。iOS も確認するなら、最後の行が `/Applications/Xcode.app/Contents/Developer`。
`/Library/Developer/CommandLineTools` なら、`sudo xcode-select -s /Applications/Xcode.app` を実行するか、iOS のコマンドの前に `export DEVELOPER_DIR=/Applications/Xcode.app/Contents/Developer` を実行する。

## 1. 起動

### 1-1. 初回: クラスタとマスタ(人間が実行)

```sh
cd "$(git rev-parse --show-toplevel)"
make up
```
→ 出力に `job.batch/pokedex-migrate condition met` が含まれ、最後に `完了。http://localhost:8080 ...` が出る。この時点で `pokedex` はマスタが無いので `0/1`(異常ではない)。

```sh
cd "$(git rev-parse --show-toplevel)"
make import-fetch
make import-dry-run
created=$(make import-k8s)
echo "$created"
job_name=$(echo "$created" | grep -o 'pokedex-import-manual-[0-9]*' | tail -1)
kubectl -n pokecalc wait --for=condition=complete "job/$job_name" --timeout=600s
make pokedex-export-k3d
```
→ `import-dry-run` の出力に `blockers: none`。`wait` が `condition met` で終わる。最後に `export: ../data/generated/readmodel に書いた`。
`import-fetch` はネットワークが要る。失敗したらそこで止め、ネットワークを戻して再実行する(DB は変わっていない)。

### 1-2. 毎回: 最新のコードを入れる(動作確認の前に必ず)

```sh
cd "$(git rev-parse --show-toplevel)"
git switch main && git pull --ff-only
make deploy-latest
```
→ 最後に `deploy-latest: 全サービスを <コミット> の内容で入れ替えた` と、各サービスの READY が 1 の行が出る。
古いイメージが残ると、画面は開くのに API が 404 になる。

### 1-3. 自動の確認

```sh
cd "$(git rev-parse --show-toplevel)"
make web-k3d-smoke
API_SMOKE_STRICT=1 make api-smoke
make web-k3d-e2e
```
→ `web smoke: すべて成功(http://localhost:8080)`。`api-smoke` の最終行が `calc=200 bulk=200 reverse=200 missing_header=400 invalid_header=400 pokedex=200 internal=404 balance=200 web=200`(400・404 は異常系を意図して確かめた値で、これが正常)。`web-k3d-e2e` が全件成功する。

## 2. Web の確認

`http://localhost:8080` を開く。開発者ツールの Network タブも開く(赤い行 = 失敗した通信)。計算モードは画面上部の「オンライン(API)」(既定)。

### 2-1. 計算

1. 攻撃側の欄に日本語名の先頭の文字を入れる → 候補が出る。選ぶ → カード(名前・タイプ)と技の欄が出る。
2. 防御側も選ぶ → 結果が 5 行出る(`POST /api/calc/bulk` が 200)。
3. 技を変える → 結果が変わる。「A特化」にする → 各行の%が上がる。
4. 「攻守入れ替え」→ 左右が入れ替わり、結果が変わる。「詳細」で急所・天候・壁・ランクを変える → %が変わる。

### 2-2. 逆算・タイプバランス・素早さ・判定

1. 「逆算」タブで自分・相手を選び、観測に%を入れる → 候補が出る(`POST /api/calc/reverse` が 200)。
2. 「タイプバランス」タブでメンバーを 2 体選ぶ → 防御相性の表(18 タイプ)・チームの集計・「おすすめタイプ」が出る。技を選ぶ → 攻撃範囲の表が更新される。
3. 「素早さ」タブでポケモンを選ぶ → 自分より速い・同速・遅いポケモンの一覧が出る。
4. 「判定」タブで自分と相手のポケモン・性格を選び、両方の技の ID(英小文字・記号なし)を入れて「判定する」 → 素早さの比較と「乱数N発」が出る。
5. Network に赤い行(4xx・5xx)が無い。

### 2-3. 構築・よく計算する相手・計算履歴・端末データの削除

1. 「構築」タブで構築名を入れて「作成」 → 一覧に出る(`POST /api/team/teams` が 201)。
2. 「メンバーを編集」で種族・技(4 つまで)・持ち物・特性・性格・テラスタイプを選び、SP を 1 つ 32・合計 66 まで入れる → 保存できる。1 つに 33、または合計 67 → エラーで保存できない。
3. 「メンバーを保存」→ 「保存しました」。再読み込みしても内容が残る。別タブへ移って戻っても未保存の下書きが残る。
4. 「計算」タブで同じ防御側で計算を数回行い、再読み込み → 防御側の下に「よく計算する相手」のチップが出る。押す → 防御側に入って結果が出る。
5. 「お気に入り」タブ → 「計算履歴」に新しい順で行が並ぶ(`GET /api/record/calc-history?limit=20` が 200)。「この計算を使う」→ 計算タブに戻って結果がすぐ出る。
6. ページ下部「このアプリについて」→「この端末のデータを削除」→ 確認で「削除する」 → 「削除しました。」。構築の一覧・チップ・履歴が空になり、計算は使える。

### 2-4. オフライン(WASM)

先に 2-1 でオンラインのまま種族を 2 つ引き(マスタのキャッシュを温める)、画面上部で「オフライン(WASM)」を選ぶ。

1. 引いた種族を攻撃側・防御側に選ぶ → 結果が 5 行出る(通信しない)。
2. 「逆算」タブ → URL が `/reverse` になる。戻るで前のタブに戻る。`http://localhost:8080/reverse` を直接開く → 逆算タブが選ばれている。
3. ページ下部「このアプリについて」→ `/about` に非公式の注記とデータ出典が出る。「計算に戻る」で入力が残っている。

## 3. iOS の確認

シミュレータは `iPhone 17e` を使う(`IOS_SIMULATOR` で指定。未指定の既定は `iPhone 18 Pro`)。

```sh
cd "$(git rev-parse --show-toplevel)"
export DEVELOPER_DIR=/Applications/Xcode.app/Contents/Developer
xcrun simctl list devices | grep "iPhone 17e"
```
→ `iPhone 17e (...)` の行が出る(`Booted` でなくてよい)。無ければ Xcode の Settings → Components で iOS シミュレータを入れる。

### 3-1. コマンドで起動する(最短)

```sh
cd "$(git rev-parse --show-toplevel)"
export DEVELOPER_DIR=/Applications/Xcode.app/Contents/Developer
make ios-run IOS_SIMULATOR="iPhone 17e" POKECALC_API_BASE_URL=http://localhost:8080
```
→ 数分(初回は長め)でシミュレータが開き、アプリが起動する。計算画面に「APIに接続中(localhost)」のバッジと、実データのポケモンでの結果が出る(1-1 と 1-2 が済んでいることが前提。URL に `/api` は付けない)。
防御側のカードをタップして別のポケモンを選ぶ → カードと技の行の相性・結果の%が変わる。
`POKECALC_API_BASE_URL` を付けない、またはモック強制の `IOS_RUN_MOCK=1` を付けると、「モックデータで動作中」とテスト用の名前(「テスト…」)で動く。

### 3-2. Xcode で開いて実行する

```sh
cd "$(git rev-parse --show-toplevel)"
make ios-gen
open ios/PokeCalc.xcodeproj
```

Xcode で次の順に操作する。

1. 画面上部のスキームが `PokeCalc`、実行先が `iPhone 17e`(シミュレータ)であることを確認する。
2. 実 API につなぐ場合のみ、スキーム名 `PokeCalc` をクリック → Edit Scheme → Run → Arguments を開き、Environment Variables の `POKECALC_API_BASE_URL` に `http://localhost:8080` を入れる。入れなければモックで動く。
3. Run(▶、または Cmd+R)を押す。

→ シミュレータでアプリが起動し、3-1 と同じ画面になる(実 API なら「APIに接続中(localhost)」のバッジ)。ビルド中の `warning:` は生成コードのもので無視してよい。

ビルド設定で URL を固定したいときは、`ios/PokeCalc/Config/PokeCalc.xcconfig` の `POKECALC_API_BASE_URL` に `http:/$()/localhost:8080` と書く(`//` 以降がコメントになるため `$()` で分断する)。確認が済んだら空に戻し、コミットしない。

### 3-3. iOS の自動テスト

```sh
cd "$(git rev-parse --show-toplevel)"
export DEVELOPER_DIR=/Applications/Xcode.app/Contents/Developer
make ios-test IOS_SIMULATOR="iPhone 17e"
```
→ lint・生成物の一致・XCTest・XCUITest・Info.plist 検査がすべて成功する。UI テストが約 190 件あり 20〜50 分かかる(他のレーンがテスト中なら順番待ちで延びる)。速く見るだけなら `make ios-swift-test`(macOS で XCTest のみ)。

### 3-4. タイプバランス画面

3-1 の実 API で起動し、ホームの「タイプバランス」を押す(この画面を直接開く起動引数は無い)。read model が入っていること(1-1 の `make pokedex-export-k3d` と 1-2 の `make deploy-latest`)。

1. メンバーを 2 体選ぶ → 防御相性が 18 タイプぶん縦に並び、「×2 弱点」「×1/2 耐性」のように倍率と語が出る。チームの集計も出る。
2. メンバーの技を選ぶ → 攻撃範囲に「×2 抜群」などが出る。
3. 仮想敵を 1 体追加する → 結果と「おすすめタイプ」が出る。
4. 弱点・抜群が、色だけでなく文字でも分かる。

モック起動ではこの画面に「タイプバランスの API に接続できません」が出る。これは正常。

### 3-5. 実機

署名チームの設定と実機インストールは人間の作業。手順は [runbooks/ios-device-install.md](runbooks/ios-device-install.md)。

## 4. うまくいかないとき

| 症状 | 原因 | 対処 |
|---|---|---|
| 8080 で画面が真っ白 | 古い web イメージ | 1-2 `make deploy-latest` |
| 「マスタデータの読み込みに失敗しました」 | 5173(Web 直。API が無い)で開いている / pokedex にマスタが無い | 8080 で開く / 1-1 のマスタ投入 |
| ポケモンを選ぶと「ポケモンの検索に失敗しました」 | pokedex が古い | 1-2 `make deploy-latest` |
| タイプバランス・素早さが 503 | read model が入っていない | 1-1 の `make pokedex-export-k3d` → 1-2 |
| 構築・履歴・よく計算する相手が出ない | record・team・TiDB が起動していない | `kubectl -n pokecalc get pods` で確認し、1-2 `make deploy-latest` |
| `curl localhost:8080` が接続できない | k3d が止まっている・8080 を別プロセスが使用 | `k3d cluster list`、`lsof -nP -iTCP:8080 -sTCP:LISTEN` |
| `xcodebuild` / `xcrun simctl` が `requires Xcode` 等で失敗 | xcode-select が CommandLineTools を指している | 0 の手順(`DEVELOPER_DIR` を設定) |
| iOS が「モックデータで動作中」/ ポケモン名が「テスト…」 | `POKECALC_API_BASE_URL` を渡していない、またはモック強制で起動した | 3-1 を `POKECALC_API_BASE_URL=http://localhost:8080` 付きでやり直す |
| iOS が「設定エラー: … 不正な URL」 | URL にスキームが無い、または xcconfig の `//` 以降が消えた | `http://localhost:8080` と書く(xcconfig では `http:/$()/localhost:8080`) |
| iOS が赤字「通信に失敗しました」・カードが「-」 | 接続先に届かない | `curl -s -o /dev/null -w "%{http_code}\n" http://localhost:8080/healthz` が 200 か確認。止まっていれば `make up` |
| iOS の結果が 404 | URL の末尾に `/api` を付けた | `http://localhost:8080` までにする |

詳しい切り分けは [impl/verify-mapping.md](impl/verify-mapping.md)。他の手順書は [runbooks/](runbooks/)(balance・speed・observability・ios-device-install)。
