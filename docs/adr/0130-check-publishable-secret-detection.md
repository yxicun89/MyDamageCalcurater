# ADR-0130: check-publishable の秘密検査を「値の形」と「追跡禁止ファイル名」で強化する(gitleaks は足さない)

- 状態: 採用
- 日付: 2026-10-01
- レーン: データ(ADR 帯 `0100〜`)
- 関連: issue #300、#74(スクリプト部分)、#238(クローズ済み)、issue #403(パッケージ D24)、ADR-0002、ADR-0408(BE_EXCLUDES)

## 背景

`scripts/check-publishable.sh` の B はキー名(password 等)+8文字以上の値だけを見るため、このリポジトリで実際に出る形
(DSN、`mysql://` の資格情報、`MYSQL_PWD`、Secret の `data:` の base64、`_authToken`、Bearer、`token:`、
`sk-ant-`・`github_pat_`・`xoxb-`・`AIza` の接頭辞、短い値、キーと値が2行に分かれる形)をほぼ検出しない。
C は `.envrc`・`id_rsa`・`credentials.json`・`*.p8`・`*.sql.gz` を禁止していない。`.gitignore` も同様。

## 決定(spec-writer の提案。受け入れ条件はテストが正)

1. gitleaks 等の外部ツールは足さない。依存を増やすと `make doctor`・CI での入手と版固定が要り、誤検知の許可リストも別管理になる。
   bash の正規表現で足りる範囲(接頭辞・DSN/URL 資格情報・Secret の data・鍵ファイル名)を、この脚本に足す。
   将来 gitleaks を併用するなら別 ADR で版固定・doctor・CI を一緒に決める。
2. B は「値の形」(DSN・URL 資格情報・トークン接頭辞・Bearer・Secret の base64)と「キー名=値」の2系統にする。
   キー名=値は、値が8文字未満でも検出する。ただし参照(`os.Getenv`・`process.env`・`${VAR}`・`$VAR`・`cfg.X`)・空文字・
   型注釈(`password: string` など)と Secret 名(`secret-type=repository`・`imagePullSecrets:`・`secretKeyRef:`)は検出しない。
   キーと値が2行に分かれる形は、検出位置をキーの行にする。
3. C は `.envrc`・`id_rsa`・`id_ed25519` 等の拡張子なしの鍵・`credentials.json`・`*.p8`・`*.sql.gz`・`secret*.yaml` と、
   ダンプらしい `*dump*.sql`・`*backup*.sql` を禁止する。**`*.sql` 全体は禁止しない**(`services/*/db/migrations/`・`db/query/`・
   `db/testdata/` に正当な SQL を追跡している)。`*.xcconfig` も禁止しない(`ios/PokeCalc/Config/PokeCalc.xcconfig` は追跡対象)。
4. `.gitignore` に同じ名前を足す。自己テストが実際の `.gitignore` に対し `git check-ignore` で確認する。
5. 許可リスト(`B_KEYVALUE_ALLOW` と新しい許可)には理由を1行ずつ付ける。base リポジトリが許可対象の名前を全部含み、
   許可リストを壊すと自己テストが赤くなる(#74)。

## 影響

- 既存の `make check-publishable`(現 main)は 0 件のまま保つ。実装者は新しい検査で現 main に誤検知が出たら、
  許可リストを足すより先に、検査を狭められないかを確認する。

## 実装での確定事項

- キー名=値は password・secret 系で3文字以上。`token` は値16文字以上かつ数字を含むものだけ(`let token = beginInput()` 等のコード変数を避ける)。
  DSN は値8文字以上(`<pw>` の placeholder を除く)。許可は理由付きで `B_KEYVALUE_ALLOW_EXTRA`・`B_DSN_ALLOW`・`B_BEARER_ALLOW` に集約。
- 現 main で出た誤検知(`automountServiceAccountToken: false`・Swift の `token` 変数・テストの変数代入(Passwd に変数 pass を渡す行)・テストの偽 DSN `@tcp(127.0.0.1:1)`・ゼロ埋め UUID の Bearer・echo の日本語)は、許可を足す前に検査を狭めた。
  残る許可は偽 DSN(閉じたポート)・ゼロ UUID・英字だけ3〜7文字・非 ASCII の説明文・型注釈と環境変数参照だけ。
- 2行に分かれる値は awk(mawk で動くよう区間 `{n}` を使わない)。

## 既知の見逃し(誤検知を避けるための設計上の限界)

- 英字だけの 7 文字以下の値(辞書にある短い単語をそのまま使ったパスワード)。変数名・単語と区別できないため許可している。8 文字以上、または数字・記号を含めば検出する
- 数字を含まない、または 16 文字未満の token の値。コードの変数名(最新の一覧を指す変数への代入など)と区別できないため
- 許可の判定は値の先頭だけで行う(`^[^:=]*[:=]` で最初の区切りに固定)。値の途中の `=é`・`:true` で許可されないことを自己テスト b2_15〜b2_17 で固定した
- これらは「人が書いた本物の秘密」を完全には防げない。コミット前の注意と、公開前の人間の確認(CLAUDE.md)を前提にする
