# M1 の動作確認手順(ドラフト)

- 状態: **ドラフト**(2026-09-22、Web レーン。P4-7)。M1 の残り(P2-2c・P2-2d・P2-3 pokedex-svc・P3-3 契約テストと k3d スモーク)が
  main に入ったら、§4 の「まだ確認できないこと」を手順に置き換えて完成版にする。
- 対象: ダメージ計算・一括計算・逆算(engine / WASM / calc-svc / gateway / Web)。マスタは架空の例データ(実マスタは Git に置かない。ADR-0002)。
- 所要時間の目安: §1 の自動テストが数分、§2・§3 の画面確認が 10〜15 分。

## 0. 準備(初回だけ)

```sh
make doctor            # 前提ツール(Go 1.27.1・Node 26.9.0 など)
make web-install       # Web の依存(npm ci)
make web-e2e-install   # E2E 用の chromium
make wasm              # engine.wasm と wasm_exec.js を web/public/ へ
```

## 1. 自動テスト(すべて成功すること)

| コマンド | 見ていること |
|---|---|
| `make test` | engine・services・tools の Go テスト、balance、Web の単体・画面テスト(Vitest) |
| `make lint` | gofmt / vet、Web の型検査・ESLint・Prettier、公開前の検査 |
| `make build` | Go のビルド、Web の本番ビルドと JS の配信サイズ予算(≤ 300KB gzip) |
| `make test-golden` | engine と @smogon/calc 0.12.0 の照合(全件一致) |
| `make test-wasm` | ネイティブ Go と WASM の出力のバイト一致、逆算 1 秒以内 |
| `make web-test-wasm` | Web が組み立てたリクエストを本物の engine.wasm に通す |
| `make web-e2e` | ブラウザ(chromium)でオフラインの主な流れ。`/api` を遮断しても計算できる、engine.wasm は初回の計算まで読まない |
| `make web-e2e-online` | 例データで calc-svc を起動し、オンラインで同じ操作の結果がオフラインと一致する |

## 2. 画面の確認(オフライン = WASM。バックエンド不要)

```sh
make web-dev           # http://localhost:5173 を開く
```

ブラウザの開発者ツールの Network タブを開いた状態で、次を確かめる。**Chrome と Safari の両方**で行う(自動テストは chromium だけ)。

1. **開いた直後**: 「計算」タブが選ばれ、ヘッダーの計算モードは「オフライン(WASM)」。この時点では `engine.wasm` を読んでいない。
2. **計算**: 攻撃側と防御側のポケモンを選ぶ。
   - 技は自動で選ばれる。
   - 結果が5行出る(無振り / H振り / H振り+B補正 / HB振り / HB特化。特殊技なら D 系)。
   - 各行に %の幅(小数第1位)・ダメージバー・確定数(確定n発 / 乱数n発(x.x%))が出る。
   - Network に `engine.wasm` が1回だけ現れ、**Content-Type が `application/wasm`** であること(違うと `instantiateStreaming` が失敗し、フォールバックで読み込む)。
   - 体感: 初回の計算までの待ち(約4.6MB / gzip 1.3MB)と、2回目以降の即時性(入力から 100ms 以内が目標。design.md)。
3. **自分側の調整**: 「A特化」にすると各行の % が上がる。特殊技を選ぶと表示が「C特化」に変わる。
4. **持ち物の候補も比較**: オンにすると行が増え、各行に持ち物の名前(または「持ち物なし」)が出る。
5. **攻守入れ替え**: 左右のポケモンと持ち物が入れ替わり、結果が更新される。OS の「視差効果を減らす」をオンにすると入れ替えの動きが無くなる。
6. **逆算**: 「逆算」タブで、自分・相手・技を選び、観測1に整数%(例: 計算タブで見た H振りの行の範囲の値)を入れる。
   - 「H32 を仮定」の注記と、候補(補正なし / B上昇 × 持ち物)が engine の順で出る。各候補に B の SP 範囲・目安の名前・想定ダメージ幅が出る。
   - 「観測を追加」で2回目を入れると、観測を説明できる候補が増えない。
   - 101 や小数を入れると入力欄にエラーが出て、候補は出ない。
7. **キーボード**: タブ(計算 / 逆算)は ← → / Home / End で切り替えられる。ラジオはフォーカス位置が見える。
8. **メモリ**: 計算と逆算を何度か繰り返しても、タブのメモリ使用量が増え続けない(開発者ツールの Memory / タスクマネージャー)。

## 3. 画面の確認(オンライン = API。calc-svc と gateway をローカルで起動)

pokedex-svc がまだ無いので、Web の例データを calc-svc のマスタとして書き出して使う(ADR-0301 §5)。

```sh
# 1. 例データを calc-svc のスナップショットに書き出す(data/generated/ は Git 管理外)
node web/scripts/export-example-master.mjs

# 2. calc-svc(別ターミナル)
cd services && CALC_ADDR=127.0.0.1:18080 \
  CALC_MASTER_PATH=../data/generated/web-example-master.json \
  CALC_TYPECHART_PATH=../testdata/golden/typechart.json \
  go run ./calc/cmd/calc

# 3. gateway(別ターミナル)
cd services && GATEWAY_ADDR=127.0.0.1:18081 GATEWAY_CALC_URL=http://127.0.0.1:18080 \
  go run ./gateway/cmd/gateway

# 4. Web(/api を gateway へ転送)
cd web && API_PROXY_TARGET=http://127.0.0.1:18081 npm run dev
```

1. ヘッダーの計算モードを「オンライン(API)」にする。
2. §2 の 2〜6 と同じ操作をする。Network に `POST /api/calc/bulk`(逆算は `/api/calc/reverse`)が 200 で出て、`engine.wasm` は読まない。
   リクエストヘッダーに `X-Device-Id` / `X-Session-Id`(UUID)が付いている。
3. 同じ操作の結果が、オフラインのときと同じ数値になる。
4. gateway を止めると、オンラインのままではエラーが出る(自動でオフラインに切り替わらない。ADR-0301 §4)。オフラインに戻すと計算できる。

## 4. まだ確認できないこと(M1 の残り。完成版で手順に置き換える)

| 項目 | 待っているタスク |
|---|---|
| 実マスタ(チャンピオンズ M-C)での計算・名前検索 | P2-2c・P2-2d(取込)、P2-3 pokedex-svc、Web がオンラインのときにマスタを API から読む `MasterSource` |
| k3d 上の全体(`make up` → gateway 経由で Web と API) | P3-3(契約テストと k3d スモーク)、`make e2e` への Web の E2E のつなぎ込み(DECISIONS.md で提案) |
| Tailscale 経由の外出先からの利用 | M1 の範囲外(運用) |

## 結果の記録

確認した日付・ブラウザ(Chrome / Safari の版)・気づいた点を、`docs/plan.md` の P4-5 / P4-7 の行か、改善要望(`/improve`)として残す。
