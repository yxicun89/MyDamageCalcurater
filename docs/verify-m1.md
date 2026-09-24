# M1 の動作確認

上から順に実行する。各コマンドの下の「→」が成功の見え方。

入口は1つ: **ブラウザ・iOS とも `http://localhost:8080`**(k3d → Traefik → gateway。gateway が `/api/*` をバックエンド
〈calc・pokedex〉へ、それ以外を画面〈web〉へ転送する。ADR-0205)。構成の図は [impl/k8s-local.md](impl/k8s-local.md)。
画面の確認はオフライン(WASM。ブラウザ内で計算)とオンライン(API。k3d のバックエンドで計算)の両方で行う。

## 1. 準備(初回だけ)

```sh
cd "$(git rev-parse --show-toplevel)"
make doctor
make web-install
make web-e2e-install
```
→ doctor が不足ツールを報告しない。

## 2. 自動テスト

```sh
cd "$(git rev-parse --show-toplevel)"
make test && make lint && make build
make test-golden
make test-wasm
make web-test-wasm
make web-e2e
make web-e2e-container
make web-e2e-online
make web-e2e-balance
```
→ すべて最後まで成功する(失敗したらそこで止めて plan.md に記録)。

## 3. k3d を用意する(初回だけ)

```sh
cd "$(git rev-parse --show-toplevel)"
make up
```
→ 最後に `完了。` が出る。`kubectl -n pokecalc get pods` で `mysql-0`・`pokedex` が `Running`。

マスタ(実データ)を入れる。バックエンドはマスタが無いと計算できないので、次の §4 より先に行う。

```sh
cd "$(git rev-parse --show-toplevel)"
make import-fetch
make import-dry-run
make import-k8s
```
→ `import-dry-run` の最後の行が `blockers: none`。`import-k8s` の Job が `condition met` で終わる。

タイプバランス・素早さが読む read model を書き出す(`mysql` へ一時的に port-forward する)。

```sh
cd "$(git rev-parse --show-toplevel)"
kubectl -n pokecalc port-forward svc/mysql 3306:3306 >/dev/null 2>&1 &
PF_PID=$!
sleep 2
export POKEDEX_DATABASE_DSN=$(kubectl -n pokecalc get secret mysql-auth -o jsonpath='{.data.pokedex-dsn}' | base64 -d | sed 's/@tcp(mysql:/@tcp(127.0.0.1:/')
make pokedex-export
kill $PF_PID
```
→ `export: ../data/generated/readmodel に書いた`。

## 4. 最新のコードを k3d に入れる(動作確認の前に毎回)

```sh
cd "$(git rev-parse --show-toplevel)"
git switch main && git pull --ff-only
make deploy-latest
```
→ 最後に `deploy-latest: 全サービスを <コミット> の内容で入れ替えた` と、`balance`・`calc`・`gateway`・`judge`・`pokedex`・`speed`・`web` の
7 行(READY が 1)が出る。古いイメージが残ると、画面は開くのに API が 404 になることがある(例: 技の一括取得)。

## 5. 自動の動作確認(ブラウザで開く入口 8080 を通る)

```sh
cd "$(git rev-parse --show-toplevel)"
make web-k3d-smoke
make api-smoke
make web-k3d-e2e
```
→ `web smoke: すべて成功(http://localhost:8080)`。`api-smoke` の最終行に `calc=200 bulk=200 reverse=200 pokedex=200 web=200 balance=200`
(`missing_header=400`・`invalid_header=400`・`internal=404` は異常系を意図して確かめた結果で、この値が正常)。
`web-k3d-e2e` は `2 passed`(実ブラウザで 8080 を開き、オフラインとオンライン〈実マスタ〉の両方で計算結果が出ることを確かめる)。

## 6. ブラウザで確認する(Chrome と Safari)

`http://localhost:8080` を開く。開発者ツールの Network タブを開いておく(赤い行 = 失敗した通信)。

### 6-1. オフライン(WASM)

画面上部の「オフライン(WASM)」を選ぶ(既定)。計算はブラウザ内の engine.wasm で行い、通信しない。
**マスタは架空の例データ**(「テストほのお」など。実データのオフライン計算は未対応: issue #210)。

1. ポケモンを攻撃側・防御側とも選ぶ → 結果が5行出る。
2. 自分の調整を「A特化」にする → 各行の%が上がる。
3. 「持ち物の候補も比較」をオンにする → 行が増え、各行に持ち物の名前が出る。
4. 「攻守入れ替え」を押す → 左右が入れ替わり、結果が変わる。
5. 「逆算」タブを押す → URL が `/reverse` になる。
6. 自分・相手を選び、観測1に計算タブの「H振り」の行の範囲の整数%を入れる → 「H32 を仮定」と候補が出る。
7. 「観測を追加」で同じ値を入れる → 「近い候補」でない候補の数が増えない。
8. ブラウザの戻るを押す → 前のタブに戻る。`http://localhost:8080/reverse` を直接開く → 逆算タブが選ばれている。
9. タブで ← → キーを押す → タブが切り替わる。

### 6-2. オンライン(API・実データ)

画面上部の「オンライン(API)」を選ぶ。計算は k3d の calc-svc、マスタは pokedex-svc(実データ)。

1. 攻撃側の欄に日本語名の先頭の文字を入れる → 候補が出る。1つ選ぶ → カード(名前・タイプ)が出て、技の欄に技が並ぶ。
2. 防御側も同じように選ぶ → 結果が5行出る(Network に `POST /api/calc/bulk` が 200)。
3. 技を変える → 結果が変わる。「A特化」にする → 各行の%が上がる。
4. 「逆算」タブで自分・相手を選び、観測に%を入れる → 候補が出る(`POST /api/calc/reverse` が 200)。
5. 「タイプバランス」タブ → メンバーを2体選ぶ → 防御相性の表(18タイプ)とチームの集計、「おすすめタイプ」が出る。
   技を選ぶ → 攻撃範囲の表が更新される。「仮想敵を追加」で1体選ぶ → 相性の表と人数の集計が出る。
6. 「素早さ」タブ → ポケモンを選ぶ → 自分より速い・同速・遅いポケモンの一覧が出る。
7. 「判定」タブ → 自分と相手のポケモン・性格を選び、両方の「技の ID」に技の ID(英小文字。例: 計算タブの技の欄の技を英語表記の小文字・記号なしにしたもの)を入れて「判定する」 → 素早さの比較と「自分の技で相手を 乱数N発」が出る(技は ID の手入力: issue #309)。
8. Network に赤い行(4xx・5xx)が無い。

オンラインでは「持ち物の候補も比較」は使えない(持ち物の効果データが公開 API に無い: issue #211。画面にもその旨が出る)。

## 7. iOS で確認する

k3d で動いているバックエンド(`http://localhost:8080` → Traefik → gateway)に iOS アプリをつないで計算する。
`make ios-sim-run` は常にモックで起動する(`POKECALC_USE_MOCK=1` を付ける)ので、ここでは使わず、接続先を渡してビルドする。
接続先はビルド設定 `POKECALC_API_BASE_URL` → Info.plist の `PokeCalcAPIBaseURL` で決まる(ADR-0500 §5)。空ならモック、`http(s)://` の URL ならその API。`/api` は付けない(生成クライアントのパスが `/api/...` から始まるため)。

検証状況(2026-09-25、Xcode 27 / iPhone 18 Pro シミュレータ iOS 27.0):
- (A)(B): 実行して確認済み。`http://localhost:8080` でも、Mac の LAN IP(`http://192.168.x.x:8080`)でも、実データのポケモンで計算結果が出た。Info.plist に ATS の例外は無いが、http のまま通った。
- (C): 実機が無いため**未検証**。LAN IP で gateway に届くこと(Mac 上の curl で 200)と、LAN IP を接続先にしたアプリがシミュレータで動くことまでは確認済み。

#### 7-A. 準備: Xcode を選ぶ

```sh
cd "$(git rev-parse --show-toplevel)"
xcode-select -p
```
→ `/Applications/Xcode.app/Contents/Developer` なら次へ。`/Library/Developer/CommandLineTools` なら、次のどちらかにする。

- 恒久的に切り替える: `sudo xcode-select -s /Applications/Xcode.app`
- sudo を使わない: このターミナルでだけ Xcode を使う(下のコマンドはすべてこのターミナルで実行する)

```sh
export DEVELOPER_DIR=/Applications/Xcode.app/Contents/Developer
xcrun simctl list devices | grep "iPhone 18 Pro"
```
→ `iPhone 18 Pro (...)` の行が出る(`(Booted)` でなくてもよい。次の節で起動する)。

k3d のバックエンドが動いていることを確認する。

```sh
curl -s -o /dev/null -w "%{http_code}\n" http://localhost:8080/api/pokedex/natures \
  -H 'X-Device-Id: 00000000-0000-4000-8000-00000000d001' -H 'X-Session-Id: 00000000-0000-4000-8000-00000000d002'
```
→ `200`(動いていなければ §3・§4)。

#### 7-B. シミュレータで k3d の API を使って計算する(確認済み)

接続先を `http://localhost:8080` にしてビルドする(コマンドラインで渡すので xcconfig を書き換えなくてよい)。

```sh
cd "$(git rev-parse --show-toplevel)"
xcrun simctl boot "iPhone 18 Pro" 2>/dev/null; open -b com.apple.iphonesimulator
xcodebuild build -quiet -project ios/PokeCalc.xcodeproj -scheme PokeCalc \
  -destination "platform=iOS Simulator,name=iPhone 18 Pro" -derivedDataPath ios/build/DerivedData \
  POKECALC_API_BASE_URL=http://localhost:8080
plutil -extract PokeCalcAPIBaseURL raw -o - ios/build/DerivedData/Build/Products/Debug-iphonesimulator/PokeCalc.app/Info.plist
```
→ 最後の行が `http://localhost:8080`(ビルド中の `warning:` は生成コードのもので無視してよい)。

インストールして、計算画面を開いた状態で起動する。

```sh
xcrun simctl install "iPhone 18 Pro" ios/build/DerivedData/Build/Products/Debug-iphonesimulator/PokeCalc.app
xcrun simctl terminate "iPhone 18 Pro" com.example.pokecalc 2>/dev/null
SIMCTL_CHILD_POKECALC_OPEN_CALC_SCREEN_AT_LAUNCH=1 xcrun simctl launch "iPhone 18 Pro" com.example.pokecalc
```
→ シミュレータの計算画面の上に「APIに接続中(localhost)」のバッジ。攻撃側・防御側に実データのポケモン、技の行、「無振り 12.3〜14.5%」のような結果の行が並ぶ(名前が「テスト」で始まっていたらモックで動いている)。

シミュレータで防御側のカードをタップし、検索欄に日本語名の先頭の文字を入れて別のポケモンを選ぶ。
→ 防御側のカードと、技の行の相性(「いまひとつ」等)・結果の%が変わる。

補足: gateway はアクセスログを出さない(エラー時だけ出す)ので、`kubectl -n pokecalc logs deploy/gateway` では iOS からのリクエストは見えない。実データの名前が出ていることで API から取れていると判断する。

モックに戻すときは `make ios-sim-run IOS_SCREEN=calc`(モックで作り直して起動する)。

#### 7-C. 自分の iPhone で k3d の API に接続する(未検証。署名・インストールは人間の作業)

iPhone と Mac を同じ Wi-Fi につなぐ。k3d はホストの 8080 を全インターフェース(`0.0.0.0`)で公開している(deploy/k3d.yaml の `8080:80`)。そのため Mac の LAN IP で届く。

```sh
cd "$(git rev-parse --show-toplevel)"
ipconfig getifaddr en0
curl -s http://$(ipconfig getifaddr en0):8080/healthz
```
→ 1行目に `192.168.x.x` のような IP(以下 `<Mac の IP>`)、2行目に `{"status":"ok"}`。Wi-Fi が en0 でない Mac では `ipconfig getifaddr en1` を試す。

iPhone の Safari で `http://<Mac の IP>:8080/healthz` を開く。
→ `{"status":"ok"}` が表示される(表示されなければ、アプリより先に 7-D のネットワークの項を見る)。

`ios/PokeCalc/Config/PokeCalc.xcconfig` の `POKECALC_API_BASE_URL` を書き換える。xcconfig では `//` 以降がコメントになるので、`$()` で分断して書く(ADR-0500 §5)。

```
POKECALC_API_BASE_URL = http:/$()/<Mac の IP>:8080
```
→ この変更はコミットしない(確認が済んだら空に戻す)。

Xcode で `ios/PokeCalc.xcodeproj` を開き、署名チームを設定して実機で実行する。手順は [runbooks/ios-device-install.md](runbooks/ios-device-install.md) の「4. 署名チームを設定する」「5. 実機にインストールする」と同じ。
→ iPhone でアプリが起動する。

初回の通信で「"PokeCalc"がローカルネットワーク上のデバイスの検出および接続を求めています」が出たら「許可」を押す。
→ 計算画面に「APIに接続中(<Mac の IP>)」のバッジと、実データのポケモン名での計算結果が出る。

終わったら xcconfig の `POKECALC_API_BASE_URL =` を空に戻す。
→ `git status` に `PokeCalc.xcconfig` が出ない。

補足:
- ATS: Info.plist に http の例外は無い。IP アドレス宛の http は ATS の対象外で、シミュレータでは `http://<Mac の IP>:8080` で動くことを確認済み。実機では未検証。
- Mac の LAN IP は DHCP で変わることがある。変わったら xcconfig を直して再ビルドする。Mac がスリープすると届かない。
- 外出先から使う場合(HTTPS・Tailscale)は [runbooks/ios-device-install.md](runbooks/ios-device-install.md) を使う。

#### 7-D. うまくいかないとき

| 症状 | 原因 | 対処 |
|---|---|---|
| `xcodebuild` / `xcrun simctl` が `requires Xcode` 等で失敗する | xcode-select が CommandLineTools を指している | 7-A の `export DEVELOPER_DIR=...` か `sudo xcode-select -s /Applications/Xcode.app` |
| 画面に「モックデータで動作中」、ポケモン名が「テストモン…」 | `POKECALC_API_BASE_URL` を渡さずにビルドした、または `make ios-sim-run` で起動した(モック強制) | 7-B のビルドをやり直し、`plutil` で URL が入っているか確かめる |
| 「設定エラー: … 不正な URL」 | URL にスキームが無い、または xcconfig で `//` 以降が消えて `http:` だけになった | xcconfig では `http:/$()/<ホスト>:8080` と書く。コマンドラインで渡すときは `http://…` のままでよい |
| バッジは「APIに接続中」、赤字で「通信に失敗しました。接続を確認してもう一度お試しください。」、カードが「-」 | 接続先に届かない(k3d が止まっている・ポート違い・IP が変わった) | 7-A の curl で `200` を確認(止まっていれば `make up`)。実機なら `ipconfig getifaddr en0` の値と xcconfig を照合 |
| 結果が 404 になる・何も出ない | URL の末尾に `/api` を付けた(`/api/api/...` になる) | `http://<ホスト>:8080` までにする |
| 実機だけ通信に失敗し、Safari では `/healthz` が開ける | ローカルネットワークの許可を拒否した | iPhone の「設定 → プライバシーとセキュリティ → ローカルネットワーク」で PokeCalc をオンにする(未検証) |
| 実機の Safari でも `/healthz` が開けない | 別のネットワーク(ゲスト Wi-Fi・端末間通信の遮断・VPN)、または Mac のファイアウォールが受信を拒否 | 同じ Wi-Fi か確認。「システム設定 → ネットワーク → ファイアウォール」が有効なら Docker(k3d)の受信を許可する(確認時の Mac はファイアウォール無効) |
| 実機で許可ダイアログが出ず、通信にも失敗する | Info.plist に `NSLocalNetworkUsageDescription` が無い(未検証。下の「必要な修正の候補」) | 下の候補の修正を入れて再ビルド |

#### 7-E. 実機で問題が出たときの修正候補(未検証)

シミュレータでの API 接続(localhost・LAN IP とも)は、コードを変えずに動いた。必須の修正は無い。実機で確認するときに問題が出たら、次を検討する。

- `ios/PokeCalc-Info.plist` に `NSLocalNetworkUsageDescription`(例: 「同じ Wi-Fi の Mac で動く計算サーバーに接続します」)を追加する。いまは無い。iOS はキーが無くてもローカルネットワークの許可ダイアログを既定の文言で出すと考えられるが、実機では未検証。ダイアログが出ずに失敗する場合の対処。
- (任意)gateway にアクセスログが無いので、iOS からのリクエストを `kubectl logs` で追えない。追いたい場合は `services/gateway` にリクエストログのミドルウェアを入れる(別タスク・ADR)。

## 8. 結果の記録

確かめた日付・ブラウザ/端末・うまくいかなかった番号を、docs/plan.md の P4-5 の行に書く(または AI に伝える)。

## うまくいかないとき

| 症状 | 原因 | 対処 |
|---|---|---|
| 8080 で画面が真っ白 | 古い web イメージ(JS が 404。issue #268) | §4 `make deploy-latest` |
| 「マスタデータの読み込みに失敗しました」 | 5173(Web 直。API が無い)で開いている / pokedex にマスタが無い | 8080 で開く / §3 `make import-k8s` |
| ポケモンを選ぶと「ポケモンの検索に失敗しました」 | pokedex が古い(`/api/pokedex/moves/batch` が 404) | §4 `make deploy-latest` |
| タイプバランス・素早さが 503 | read model が入っていない | §3 `make pokedex-export` → §4 |
| `curl localhost:8080` が接続できない | k3d が止まっている・8080 を別のプロセスが使用 | `k3d cluster list`、`lsof -nP -iTCP:8080 -sTCP:LISTEN` |

詳しい切り分けは [impl/verify-mapping.md](impl/verify-mapping.md) §4。`make web-k3d-open`(localhost:5173)は gateway を通さず画面だけを
配信する**診断用**で、API を使うオンラインモードはそこでは動かない。
