# ADR-0131: 配った種族 key の台帳を持ち、ID の消滅と消滅後の再利用を投入で止める(issue #277)

- 状態: 提案(spec-writer がテストと一緒に起草。実装・critic 前)
- 日付: 2026-10-01
- レーン: データ(ADR 帯 `0100〜`)
- 関連: issue #277、issue #403(パッケージ D12)、ADR-0100(スキーマ・migration)、ADR-0101 §9(全置換の投入)、
  ADR-0104 §3(終了コード)、ADR-0110・ADR-0125(用途別ユーザーと表ごとの権限)、ADR-0127(読み取りスナップショット)

## 背景

- `importer.Apply` は全置換。`ErrKeyChanged` は「新しい出力にある種族」について、既存 DB と key↔showdown_id が
  食い違うときだけ止める。新しい出力から消えた key は検査されずに削除される。
- そのため「投入 A で key X が消える」→「投入 B で key X が別の showdown_id に付く」の2段階で、同じ key の指す種族が
  入れ替わる(投入 B の時点で既存 DB に X が無いので逆向きの検査も効かない)。key は formeOrder の添字から採番するので、
  上流のフォームの並べ替え・削除で起こりうる。team-svc・record-svc・端末に保存された key が黙って別の種族を指す。
- 技・持ち物・特性の ID も全置換で黙って消え、保存済みの構築が参照先を失う。
- pokedex は自分の DB にしか触れない(CLAUDE.md 絶対ルール4)。保存済みの構築を守るには、pokedex 側で
  key の消滅と再利用を止める。

## 決定

### 1. 種族 key の台帳 `species_key_ledger` を migration 000009 で足す

- 列: `species_key CHAR(8) ascii_bin`(主キー)、`showdown_id VARCHAR(64) ascii_bin`(一意)、`first_seen_at DATETIME`。
  型は `species` の同名の列に揃える。1つの key に showdown_id は1つ、1つの showdown_id に key は1つ。
- species への外部キーを持たない(全置換で species の行が消えても、台帳の行は残す)。
- 台帳は追記だけ。sqlc のクエリは参照と追記だけを置き、削除・更新・REPLACE を置かない(layout テストで固定)。
  importer のユーザーには ADR-0125 のとおり表ごとの DML が付く(台帳だけ INSERT に絞ることはしない。
  権限の表の一覧をコードに持たない ADR-0125 の方針を優先し、追記だけはクエリの側で守る)。
- reader は SELECT できるが使わない(公開 API・読み取りスナップショットには載せない)。

### 2. Apply の検査(1トランザクションの中、削除の前)

1. **既存の DB への移行**: 今の `species` にあって台帳に無い (key, showdown_id) を台帳に足す
   (台帳ができる前に投入した DB を、次の投入で台帳に載せる経路。migration には INSERT を書かない規則
   〈`TestMigrationsHaveNoData`〉があるので、移行は importer が行う)。
2. **key の対応**: 既存の `species` と台帳の両方に対して、新しい出力の各種族の key↔showdown_id が食い違えば
   `ErrKeyChanged`(従来の検査に台帳を加える。消えた key の別の showdown_id での再利用、消えた showdown_id の
   別の key での再登場も止める)。台帳にある組がそのまま戻る(復活)のは通す。
3. **消滅**: 既存の DB にあって新しい出力に無い、種族 key・技 ID・持ち物 ID・特性 ID を集める。
   承認されていないものがあれば `ErrKeyRemoved`(`*RemovedIDsError` に一覧を持つ)。
   承認された ID が実際には消えていなければ、打ち間違いとして `ErrInvalidInput`。
4. 検査を通れば従来どおり全置換し、最後に新しい出力の種族のうち台帳に無いものを `first_seen_at = now` で追記する。

検査の順は 1 → 2 → 3(key の食い違いは承認では通らないので、承認の検査より先に止める)。
止めたときは DB(台帳を含む)を変えない。

### 3. 消滅の承認は `-allow-removed` で人が明示する

- `import -allow-removed species:9002-002,move:teststrike`。要素は `<種類>:<ID>`(種類は species/move/item/ability)。
  種族 key と技 ID を取り違えないよう、種類は必須。形式の誤りは終了コード 2(使い方の誤り)。
- 既定は何も許さない(CronJob は消滅を自動で通さない)。承認しても再利用(§2-2)は通らない。
- 承認で消えた種族 key も台帳には残るので、後の再利用は常に止まる。

### 4. 終了コード

- `ErrKeyRemoved` は終了コード 3(人間の対応が要る。DB は変えない)。stderr に消えた ID を `<種類>:<ID>` で並べ、
  `-allow-removed` での承認の仕方を案内する(照合報告 latest.json は DB を見ずに書くので、消滅の通知は投入時の
  stderr = CronJob のログと終了コードで行う)。

### 5. 公開の API(importer)

- `ApplyWithOptions(ctx, db, out, versions, now, ApplyOptions)`。`Apply` は `ApplyOptions{}`(何も許さない)と同じ。
- `Store.Apply` と `RunStore` は `ApplyOptions` を受け取って渡す。`Run` は従来どおり(何も許さない)。
- DB を使わない判定は純粋な関数に分ける(`RemovedIDs`・`CheckRemovals`・`CheckLedger`・`ParseAllowRemoved`)。
  `make test` で確かめられるようにするため。

## 検討した代替案

- **消滅は警告だけ**: 実装は軽いが、2段階の乗っ取りを防げない(issue の既定案で不採用)。
- **migration で台帳を埋める(INSERT … SELECT)**: 移行は1回で済むが、migration にデータの INSERT を書かない規則
  (ADR-0100 §2・`TestMigrationsHaveNoData`)と衝突する。importer の投入の最初に埋める方にした。
- **台帳に removed_at を持ち、消滅を台帳と比べる**: 消滅の検出は今の DB との比較で足りる(承認した消滅は次の投入で
  もう消滅ではない)。列を増やさない。
- **台帳への importer の権限を INSERT/SELECT に絞る**: 守りは強いが、表ごとの権限の一覧をコードに持つことになり
  ADR-0125 の方針に反する。クエリで追記だけにする。
- **技・持ち物・特性にも台帳を持つ**: これらの ID は Showdown の id そのもの(ID が同じなら同じもの)なので、
  再利用で別のものを指す問題が無い。消滅の検出だけにする。

## 影響

- 既存の DB: migration 000009 の後の最初の投入で、今の species から台帳ができる。追加の手作業は無い。
- 上流でフォームが消える版を取り込むときは、人が `-allow-removed` を付けて手で投入する(CronJob は止まって終了コード 3)。
- team-svc 側の参照切れ(既に消えた ID を持つ構築)の扱いは対象外(issue #277 の範囲外)。
