# ADR-0124: 適用済みの migration 000005 の down を書き換え、slot 4 の行を先に消す(issue #278)

- 状態: 採用
- 日付: 2026-09-25
- レーン: データ(ADR 帯 `0100〜`)
- 関連: issue #278、issue #221(dirty からの復旧)、ADR-0100(スキーマと migrate)、ADR-0103 §12(slot 4)

## 背景

migration 000005 は `species_abilities.slot` の CHECK を 1..3 から 1..4 に広げた。down は CHECK を 1..3 に
付け直すだけで、slot = 4 の行が残っていると `Error 3819: Check constraint ... is violated` で失敗する。
実データには slot 4 がある(ADR-0103 §12)ので、実データの入った DB では `make migrate-down` が必ず
000005 で止まり、`version=4 dirty=true` の中途半端な状態になっていた(000007・000006 の表は消え、それ以外は残る)。
000005 は main に取り込み済みで、k3d の DB にも適用されている。

## 検討した案

- (A) 000005 の down の先頭で `DELETE FROM species_abilities WHERE slot = 4` を流す。**採用**。
- (B) DownAll の前にアプリ側(dbmigrate)でデータを消す。スキーマ固有の知識が共通パッケージ(record/team も使う)に
  入り、migration だけを見て down の挙動が分からなくなる。
- (C) 新しい migration 000009 を足して直す。down の失敗は 000005 自体の down の中身の問題で、新しい版では直せない。

## 決定

1. 適用済みの migration でも、**down だけ**なら書き換えてよい。golang-migrate は適用済みの migration の
   checksum を持たず、DB に残るのは版と dirty だけなので、適用済みの DB との不整合は生じない。
   up は書き換えない(適用済みの DB と新しい DB でスキーマが変わるため。000002 を書き換えない方針は変えない)。
2. 000005 の down は、CHECK を付け直す前に slot = 4 の行を消す。down は全削除(`migrate-down`、`CONFIRM_DESTROY` 必須)
   の途中でだけ流れ、`species_abilities` は 000002 の down で表ごと消えるので、データの損失は増えない。
3. 回帰テスト: `services/pokedex/db/mysql_test.go` の `TestDownAllWithSlot4Abilities`(`-tags mysql`)で、
   slot 4 の行が入った DB の DownAll が最後まで通り、版が未適用・テーブルが残らないことを確かめる。

## 影響

- 既に `version=4 dirty=true` で止まった DB は、この変更だけでは戻らない。issue #221 の手順
  (`docs/runbooks/data.md`「migration が途中で失敗して dirty になったとき」)で版 5 に force してから、もう一度
  `make migrate-down` を流す(旧 down は ALTER の1文だけで、失敗したときは何も変わっていない。
  スキーマは版 5 の状態のままなので、版 5 として戻し直せる)。
- 今後、行の値の範囲を広げる migration を足すときは、down で範囲外の行を先に消す(または表ごと消す)こと。
