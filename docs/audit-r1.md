# R-1 コーディング規約違反の監査結果

- 日付: 2026-09-21 / 対象: `feat/claude-p1-engine` の追跡ファイル(基準は [coding-rules.md](coding-rules.md) v2)
- 方法: 読み取り専用の監査(critic)。`make gen` の差分ゼロ、`make lint` / `make test` / `make test-golden` 成功を確認
- 結論: **条件付きで公開可**。秘密情報・トークン・鍵・ローカル絶対パス・メールアドレス・端末名は追跡ファイル内に検出されなかった。
  公開前に直すべきものは、fixture の公式日本語名、`tools/golden/package.json` の `^` 指定、LICENSE 未作成の3点。

## 違反一覧

| ID | 重大度 | 内容 | 是正(R-2 の単位) |
|---|---|---|---|
| A-1 | 重要 | Git 履歴の作者情報(実名・メールアドレスが全コミットに残る。値はここに書かない) | ユーザー判断(下記) |
| A-2 | 重要 | テスト fixture・`engine/wasmapi/testdata/vectors.json` に公式の日本語名(ADR-0002「名称をテストベクタに入れない」の範囲外) | R-2-2 |
| A-3 | 重要 | `tools/golden/package.json` が `^0.10.0`(oracle は完全固定の方針) | R-2-3 |
| A-4 | 軽微 | LICENSE / NOTICE が無い | ユーザー判断 |
| A-5 | 軽微 | ADR に内輪の事情の表現(利用制限・レートリミット・「業務で慣れている」) | R-2-7(事実だけの表現に) |
| A-6 | 軽微 | ADR-0002 に第三者データの抜粋(技の威力変更表など) | ユーザー判断 |
| A-7 | 軽微 | `.gitignore` に `*.wasm` `*.pem` `*.key` `*.p12` が無い | R-2-3 |
| A-8 | 情報 | Go の module path に GitHub アカウント名が含まれる | ユーザー判断 |
| B-1 | 重要 | タイプ相性表が engine にあるが、ADR-0002 / requirements.md は DB の `type_chart` を前提。どちらが正かを決めた ADR が無い | R-2-7(ADR で決定) |
| B-2 | 重要 | OpenAPI の `Error.code` が enum でなく、WASM 境界のエラーコードと重複する | P3-1(契約変更) |
| B-3 | 重要 | `DefenderPreset` enum と `PresetKey` の一致を検査するテストが無い | P3-1 |
| B-4 | 軽微 | ドメイン定数の一部が無名リテラル(HP/その他の実数値オフセット 75/20、補正 2048/6144/8192、プリセットの 32) | R-2-4 |
| B-5 | 軽微 | engine が日本語の表示ラベルを持つことと「表示文言はクライアント側」規約の整合が ADR に無い | R-2-7 |
| B-6 | 軽微 | `Version = "0.0.0-dev"` が engine と services の2か所にある | R-2-5 |
| C-1 | 重要 | `engine/reverse.go` の `CalcReverse` が約150行・ネスト5重、ファイルも 638 行 | **P1-12 で作り直す際に規約どおり書く**(先にリファクタしても捨てるため R-2 では行わない) |
| C-2 | 軽微 | `engine/wasmapi/requests.go` の一部関数が 65〜80 行(平坦なので実害は小さい) | 必要になれば |
| C-3 | 軽微 | 末尾が空の `//` コメント | R-2-5 |
| C-4 | 軽微 | `real` が組み込み関数を隠す(`engine/stats.go`) | R-2-5 |
| C-5 | 軽微 | `AllStatKeys` が exported な可変スライス | R-2-5 |
| D-1 | 重要 | `scripts/codex-review.sh` と `scripts/doctor.sh` が `set -u` のみ(`set -euo pipefail` 統一。`doctor.sh` は「不足を列挙して続ける」設計なので個別対応が要り、挙動が変わりうる) | R-2-1(単独コミット) |
| D-2 | 軽微 | `codex-review.sh` が一時ファイルを `mktemp`/`trap` で扱わない(`.reviews/` は .gitignore 済みで公開影響なし) | 現状維持 |

## ADR 等で認められた例外(違反にしていない)
- `testdata/golden/*`(数値と英語識別子のみ。日本語は 0 件を実測): ADR-0002 §追加の回答
- 防御プリセットのカタログが engine にあること: ADR-0009(Label の扱いだけ B-5)
- KO 確率・`MatchScore`・表示用 `Effectiveness` の float: ADR-0006 / ADR-0010(ただし `engine/modifiers.go` の `mult > 1` は制御分岐に float を使っており、整数比較への置換が可能。軽微)
- 生成コード `services/internal/api/openapi.gen.go`(`make gen` で差分ゼロ)

## R-2 の着手順(1コミット = 1つの塊。挙動を変えない)
1. R-2-1 シェルを `set -euo pipefail` に統一(D-1。唯一挙動が変わりうるので単独)
2. R-2-2 fixture の公式日本語名を架空名に置換(A-2。`NameJa` は計算に使わない)
3. R-2-3 oracle の完全固定と `.gitignore` の追加(A-3, A-7)
4. R-2-4 ドメイン定数に名前を付けて集約(B-4)
5. R-2-5 小さな可読性の是正(B-6, C-3, C-4, C-5)
6. R-2-7 ADR の追記・修正(B-1, B-5, A-5)。コードは触らない
7. (R-2-6 = C-1 は P1-12 に統合)

## R-3 `make check-publishable` の検査項目案
- 絶対パス(`/Users/` `/home/` `/var/folders` `/private/tmp`)、メールアドレス(許可リストは `noreply@anthropic.com` と `example.com`)、秘密らしき文字列(パスワード・鍵・トークンのパターン)、端末名・IP。**ヒットしても値は出力せず、ファイル:行と種類だけ表示する**
- 追跡してはいけないファイル: `.env`、鍵ファイル、`*.wasm`、`docs/local/`、`data/generated/`、`.reviews/`、`node_modules/`、大きなファイル(許可リスト以外)、テキスト以外(許可リスト以外)
- 第三者データ: `testdata/golden` に日本語が 1 文字でもあれば失敗(ADR-0002 の例外条件)。テストの `NameJa` は架空名(`テスト` で始まる)だけ
- 再現性: `make gen` 後に差分なし、`tools/golden/package.json` に `^`/`~` が無い、lockfile が追跡されている
- Git メタデータ: 作者情報を許可リストと照合(出力時はマスクする)。履歴全体を見るので、開発ループを遅くしないよう分けて呼ぶ

## ユーザーの判断が要る点
1. **LICENSE の方針**(A-4)。依存(Echo・oapi-codegen 等は MIT/Apache-2.0/BSD、`@smogon/calc` は MIT)は公開に支障なし。ただし `testdata/golden` の元データの権利は LICENSE とは別問題
2. **Go module path の GitHub アカウント名**(A-8)。公開するとアカウント名が出る。残す / 変える。変える場合は 3 つの `go.mod` と import の機械的な置換
3. **Git 履歴の作者名・メール**(A-1)。このまま / 今後だけ公開用にする / 履歴を書き換える(リモート未作成の今なら低コスト)。R-2 では触らない
4. **ADR-0002 の第三者データ抜粋**(A-6)を公開のまま残すか(残すなら「出典付きの調査記録で配布物ではない」と明記)
5. **タイプ相性表の正**(B-1): engine のハードコードを正(ゲーム機構扱い)か、DB の `type_chart` を正として注入するか
6. **engine が日本語の表示ラベルを持ち続けるか**(B-5)
