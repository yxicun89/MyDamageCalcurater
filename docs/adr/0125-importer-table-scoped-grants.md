# ADR-0125: importer の書き込み権限を表単位にし、schema_migrations を書き換えられないようにする(issue #312)

- 状態: 採用
- 日付: 2026-09-25
- レーン: データ(ADR 帯 `0100〜`)
- 関連: issue #312、ADR-0110(用途別の最小権限。決定1の importer の行を置き換える)、issue #221(dirty からの復旧)

## 背景

ADR-0110 は importer(`pokedex_importer`)に `pokedex.*` への `SELECT, INSERT, UPDATE, DELETE` を付けた。
DB 全体への権限なので migration の管理表 `schema_migrations` も含み、importer の資格情報で
`UPDATE schema_migrations SET dirty = 1` が通って migrate の状態を壊せた(誤操作・資格情報の漏えい)。
MySQL には「DB 全体から特定の表を除く」GRANT が無いので、除くには表ごとに付けるしかない。

## 検討した案

- (A) 表ごとの GRANT。表の一覧はコードに持たず、Provision のときに DB の `information_schema.tables`
  (= 適用済みの migration が作った表)から引く。新しい表は migrate up の後に付け直す。**採用**。
- (B) 表の一覧をコードに列挙する。migration を足すたびに一覧の更新が要り、忘れると importer が新しい表に書けない。
- (C) 現状維持(issue の既定案の1つ)。変更は無いが、migrate の状態を importer から壊せるまま。

## 決定

1. `RoleGrant` に `Scope` を足す。ゼロ値 `ScopeDatabase` は従来どおり `db`.* に付ける(reader・migrator・
   record/team の app は変えない)。`ScopeDataTables` は `SELECT` を `db`.* に(`schema_migrations` も読める)、
   それ以外(`INSERT, UPDATE, DELETE`)を `schema_migrations` を除くいまある表ごとに付ける。
   `ScopeDataTables` に DML 以外(DDL)を渡すと接続前に拒否する(表単位で DDL を配らない)。
2. importer だけを `ScopeDataTables` にする(`cmd/migrate` の `rolesFromEnv`)。
3. `cmd/migrate up`(`POKEDEX_PROVISION_DSN` があるとき)は「全ロールをプロビジョニング → Up → importer だけ
   もう一度プロビジョニング」の順にする。Up で増えた表にも importer が書けるようにするため。
   付け直しが失敗したら終了コード 1(新しい表に書けない importer を黙って残さない)。
4. 管理表の名前は `dbmigrate.MigrationsTable`(golang-migrate の既定名と同じ値)を migrate と権限の両方で使う。
5. 表の名前は識別子として GRANT 文に埋め込むので、英数字と `_` 以外を含む名前があれば拒否する(ADR-0110 の
   DB 名の検査と同じ文字種)。

## 影響

- 付け直しは `POKEDEX_PROVISION_DSN` がある up(k8s の `pokedex-migrate` Job、`make up` が流す)だけで起きる。
  `make deploy-latest` の migrate-up は migrator の DSN だけで流すため、**権限は付け直さない**。
  - 既存の k3d の DB では、この変更を入れた後に一度付け直すまで importer は DB 全体への権限のまま。
  - `make deploy-latest` で表を足す migration を入れたときは、付け直すまで importer がその表に書けない。
  - どちらも `docs/runbooks/data.md`「importer の権限を付け直す」の手順で直す。deploy-latest に付け直しを
    組み込むかは運用レーンに任せる(root の DSN を扱う経路が増えるため、ここでは足さない)。
- `make dev`・`make test-db` はプロビジョニングしない(ADR-0110 決定7)ので影響なし。
- 回帰テスト(`-tags mysql`): `grants_mysql_test.go` の `TestImporterCannotWriteMigrationsTable`
  (importer は schema_migrations を読めるが INSERT/UPDATE/DELETE は拒否、マスタの全表に DML ができる)、
  `TestImporterGrantsFollowNewTablesAfterReprovision`(付け直すまで新しい表に書けず、付け直すと書ける)。
