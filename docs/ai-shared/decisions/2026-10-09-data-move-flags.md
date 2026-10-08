## 2026-10-09: 技のフラグを取り込み、フラグに依存する特性 14 件を計算に入れる(F-04 段階2。ADR-0178)
Decision:
- 技のフラグ 9 種(bite・bullet・contact・pulse・punch・recoil・secondary・slicing・sound。値の正は engine の AllMoveFlags)を
  Showdown の flags・recoil/hasCrashDamage・secondary/secondaries から導き、`move_flags(move_id, flag)`(migration 000013)に入れる。
  calc から同じ規則で導いた値と照合し、攻撃技の食い違いは Blocker、変化技は警告。
- 取得物(Showdown・calc)の `flags` は必須。古い取得物は拒否するので `make import-fetch` で取り直す。
- 内部 API `MasterMove.flags` は省略可(表が空 = 取り込み前は全技でキーを省く = 不明)。公開 API の Move に `flags`・`mechanisms` を足す。
  read model(balance・speed)は変えない。
- engine はフラグが不明(FlagsKnown 偽)のとき、フラグに依存する効果を掛けずに計算し、その特性に unsupported_effect の印を付ける。
Reason: パンクロック等(F-04)を計算に入れるには技の性質が要る。取得元のデータから導き(ハードコードしない)、oracle と照合する。
Impact: デプロイの順序(この順で行う):
1. migrate(000013。move_flags を作る)
2. アプリの入れ替え(pokedex-svc・calc-svc・Web の WASM)。この時点では move_flags が空なので flags は省かれ、
   フラグ依存の特性は今までどおり印付きで計算される
3. 取り直し(`make import-fetch`)と再取り込み(importer → master-release)。effects.json の新しい定義と move_flags が同時に入る
Web・iOS: 契約は省略可のキーの追加だけ。iOS は生成型に追従するだけ。Web のオフラインは技の対象・機構・フラグを WASM に渡すようになった
(IndexedDB の古いキャッシュはこれらを持たないので、オフラインでは不明として印付きで計算する)。
