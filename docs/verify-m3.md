# M3(iOS)の動作確認

上から順に実行する。各コマンドの下の「→」が成功の見え方。M1 の確認([verify-m1.md](verify-m1.md))が済んでいて、k3d が動いていることが前提。
`xcrun simctl` が使えない環境(Xcode 未選択・CI など)では、§3〜§5 の目視と `make ios-test` は人間の作業になる。

## 1. 自動テスト(シミュレータ不要)

```sh
cd "$(git rev-parse --show-toplevel)/ios/PokeCalcKit"
DEVELOPER_DIR=/Applications/Xcode.app/Contents/Developer swift test
cd "$(git rev-parse --show-toplevel)"
make ios-gen-check
```
→ `swift test` が約 980 件(2026-10-03 は 984 件)で失敗 0。`ios-gen-check` が成功する(生成物が各 `openapi.yaml`(api・balance・speed)と一致)。

## 2. ビルド

```sh
cd "$(git rev-parse --show-toplevel)"
export DEVELOPER_DIR=/Applications/Xcode.app/Contents/Developer
xcodebuild build -project ios/PokeCalc.xcodeproj -scheme PokeCalc \
  -destination "generic/platform=iOS Simulator" -sdk iphonesimulator -derivedDataPath ios/build/DerivedData \
  CODE_SIGNING_ALLOWED=NO | tail -3
```
→ 最後の行が `** BUILD SUCCEEDED **`。ビルド中の `warning:` は生成コードのもので無視してよい。

## 3. シミュレータで k3d の API を使って確認する(人間の作業。simctl が要る)

接続・起動は [verify-m1.md](verify-m1.md) §7-A・§7-B のとおり実行する(`POKECALC_API_BASE_URL=http://localhost:8080` でビルドしてインストール・起動)。
→ 計算画面に「APIに接続中(localhost)」のバッジと、実データのポケモンでの計算結果が出る。

実機(署名チームの設定とインストールは人間の作業)は [verify-m1.md](verify-m1.md) §7-C と [runbooks/ios-device-install.md](runbooks/ios-device-install.md)。

## 4. タイプバランス画面(ADR-0415。第1〜3段。人間の作業)

§3 と同じビルドを、起動引数なしでシミュレータに起動し、ホームの「タイプバランス」を押す
(この画面を直接開く起動引数は無い。入口はホームのボタンだけ)。k3d の balance に read model が入っていること
(verify-m1.md §3 の `make pokedex-export-k3d` と §4 の `make deploy-latest`)。

1. メンバーを2体選ぶ(検索欄に日本語名の先頭の文字 → 候補から選ぶ) → 防御相性が 18 タイプぶん縦に並び、各行に「×2 弱点」「×1/2 耐性」のような倍率と語が出る。チームの集計も出る(第1段)。
2. メンバーの技を選ぶ → 攻撃範囲に「×2 抜群」「×1/2 いまひとつ」などが出る(第2段)。
3. 仮想敵を1体追加する → 仮想敵への結果が出る。「おすすめタイプ」は操作が落ち着いてから1回だけ計算されて出る。失敗したときは「再計算」ボタンが出る(第3段)。
4. 技範囲チェッカーで技を1〜4つ選ぶ → 受けられるポケモンが出る(長いときは「ほか N件」)。
5. 文字サイズをアクセシビリティの最大にして開き直す(シミュレータの Settings アプリ → アクセシビリティ → 表示とテキストサイズ) → 行が崩れず縦に読める。
6. ダークモードにする(シミュレータの Features → Appearance) → 文字が読める。
7. 弱点・抜群が、色だけでなく文字(「弱点」「抜群」)でも分かる。

モック構成(`POKECALC_USE_MOCK=1`・接続先なし)では架空データを出さず、各セクションに「タイプバランスの API に接続できません」が出る。これは正常(ADR-0415 §4)。

## 5. シミュレータのテスト(人間の作業。simctl が要る)

```sh
cd "$(git rev-parse --show-toplevel)"
make ios-test
```
→ lint・生成物の一致・XCTest・XCUITest・Info.plist 検査がすべて成功する。

## 6. 結果の記録

確かめた日付・端末・うまくいかなかった番号を `docs/plan.md` の M3 の該当行に書く。
