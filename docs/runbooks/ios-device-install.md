# iOS 実機インストールの手順書(Tailscale serve・人間の作業)

自宅 Mac の k3d 上の gateway を Tailscale 経由で HTTPS 公開し、実機の PokeCalc アプリから使えるようにする。
署名チームの設定・実機へのインストールは Apple ID・Tailscale アカウントを扱うため人間が行う(CLAUDE.md「人間の確認が必要なこと」)。
構成の説明は [ios/README.md](../../ios/README.md)、k3d の起動は [docs/runbooks/api.md](api.md)。

## 1. k3d の gateway を起動しておく

```sh
cd "$(git rev-parse --show-toplevel)"
make up
```
確認: 最後の行が `job.batch/pokedex-migrate condition met`(すでに起動済みならスキップしてよい。`http://localhost:8080/healthz` が `{"status":"ok"}` を返せば起動済み)。

## 2. Tailscale で gateway を HTTPS 公開する(人間の作業)

Tailscale にログイン済みで、この Mac がその tailnet に参加していること。

```sh
tailscale serve https / http://localhost:8080
```
確認: `tailscale serve status` の出力に Tailscale が発行した HTTPS URL(マシン名・tailnet 名を含む、Tailscale 管理下のドメイン)が `http://127.0.0.1:8080` へ転送されていると出る。表示された URL を控える(次の手順で使う。tailnet 固有の値なのでこの手順書には書かない)。

## 3. アプリの接続先を Tailscale の URL にする

Xcode で `ios/PokeCalc.xcodeproj` を開き、`ios/PokeCalc/Config/PokeCalc.xcconfig` の `POKECALC_API_BASE_URL` を手順2の URL に書き換える。`//` はコメント扱いになるため空の変数参照で分断する(ADR-0500 §5)。

```
POKECALC_API_BASE_URL = https:/$()/<手順2で控えた Tailscale の URL のホスト部分>
```
`/api` は付けない(生成クライアントのパスが `/api/...` から始まるため。付けると二重になる)。確認: ビルド設定を保存したら `make ios-check-infoplist` が成功する(この設定はシミュレータ向けなので実機の値では通らないが、`/api` を付けていないか・`//` が消えていないかは目で確認する)。

## 4. 署名チームを設定する(人間の作業)

1. Xcode で `PokeCalc` ターゲット → 「Signing & Capabilities」を開く。
2. 「Automatically manage signing」を有効にし、「Team」に自分の Apple ID(無料アカウント可)を選ぶ。
確認: 「Signing Certificate」にエラーの赤字が出ていない。

## 5. 実機にインストールする(人間の作業)

1. iPhone を Mac に USB または同じ Wi-Fi で接続し、Xcode の実行先(スキームの隣のデバイス選択)で実機を選ぶ。
2. `Cmd-R` でビルド・実行する。
3. 初回は iPhone 側で「設定 → 一般 → VPNとデバイス管理」から開発者を信頼する。
確認: iPhone にアプリが起動し、ルート画面に「モックデータで動作中」ではなく実際の gateway からの表示(接続先設定に応じたバッジ、またはエラー画面ならその内容)が出る。

無料 Apple ID の証明書は 7 日で失効する。切れたら手順5をやり直す(再署名・再インストール)。

## 6. 外出先から確認する(人間の作業)

iPhone のモバイル回線・別の Wi-Fi で、Tailscale アプリにログインした状態でアプリを開く。
確認: 計算画面で一括計算の結果が表示される(Mac がスリープ・シャットダウンしていると失敗する。要件どおり「Mac 起動中は」の制約)。

## 7. 後片付け(必要なとき)

```sh
tailscale serve off
```
確認: `tailscale serve status` に公開の表示が無くなる。ローカル(シミュレータ)での確認に戻すときは、手順3の `POKECALC_API_BASE_URL` を空に戻す(モックで動く)。
