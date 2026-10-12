# ADR-0811: ヘキサゴナル(ポートとアダプタ)の採用

- 状態: 採用(S0 実装済み。以後はサービスごとの移行)
- 日付: 2026-10-11
- レーン: 横断(S0 の担当はデータレーン)
- 関連: ADR-0012(サービス境界)、ADR-0105(storetest の import 検査)、docs/plan/architecture-adoption.md、docs/architecture-comparison.md

## 背景

人間があとで構造を確認しやすくするため、ソフトウェアアーキテクチャを正式に採用したい(ユーザー意向 2026-10-11)。
Go のサービスは「手順がハンドラに溜まる」「サービスごとに形が違う」状態で、依存の向きを機械で確かめる道具も無い。

## 決定

1. **Go の全サービスにヘキサゴナル(ポートとアダプタ)を採用する**。`services/<svc>/internal/` に `domain`(純粋な核)・`app`(ユースケース層)・`adapters`(httpapi・store・client・events ほか)をそろえる。
   依存は adapters → app → domain の一方向。ポートは使う側で宣言する。形と図は docs/coding-rules.md「Go のサービスの形」を正とする。
2. **新しいサービス・新しい機能は最初からこの形**で作る。
3. **機械検査は自前の `tools/archcheck`**(go/parser・標準ライブラリだけ。依存追加なし)。向き(内側が外側を import しない)だけを検査し、`make lint` から走る。
   形をそろえたサービスは検査し、未移行の一覧(`tools/archcheck/archcheck.go` の `unmigrated`)のサービスは通す。一覧に無いのに形がそろっていないサービスは失敗する。
   別サービスの `internal` を import する違反(サービス境界)は archcheck の対象外にする。サービス境界は CLAUDE.md 絶対ルール 4 と ADR-0012 の別の規則で、G1 は S1 で直し、検査を足すかは全サービスの移行の後に判断する。
4. **移行は 1 サービス = 1 PR = そのサービスのレーン**。順番は judge → calc → record → pokedex(その後 team・gateway・balance・speed)。振る舞いは変えない。
   record・team は M2 の PR が無いときに行う。pokedex の importer は対象外(S5 の後で判断)。
5. 中身の無い `domain`・`app` も、フォルダを置き `doc.go` に役割を書く。oapi-codegen の生成物(`internal/api`)の場所は今のまま(外側として扱う)。
6. Web は Feature-Sliced Design を簡略化した 4 段(ESLint で検査)、iOS は MVVM + Service プロトコルの明文化(テストで固定)。どちらも別の段階(W0〜W2・I0)で行う。iOS のターゲット分割は今はしない。
7. ADR はこの 1 本。判断が要る移行だけレーンの帯で追加し、判断の要らない移行は下の表に 1 行足す。

選ばなかった案: 新しいリポジトリへの移行(B)、go-arch-lint・golangci-lint + depguard(新しい道具の導入)。比較は docs/plan/architecture-adoption.md §3〜§4。

## 影響

- S0 以後、全レーンの PR は `make lint` の archcheck を通る。未移行のサービスは一覧で通るので、今の PR は変わらない。新しいサービスは最初から形が必要。
- 移行の PR は、着手前に `docs/ai-shared/decisions/` に 1 件足し、他レーンが同じサービスを触る PR を控えられるようにする。
- 未移行の一覧は PR ごとに 1 行ずつ減る。空になったら一覧の仕組みごと消す。

## 段階の記録

| 段階 | 内容 | 状態 |
|---|---|---|
| S0 | ADR・coding-rules・tools/archcheck・make lint(コードは動かさない) | 2026-10-11 完了(全サービスが未移行の一覧) |
| S1 | G1 の解消(RoleGrant・プロビジョニングを internal/dbmigrate へ。record・team の migrate が pokedex を参照しない) | 2026-10-12 完了(検査は足さない。CLAUDE.md の規則とレビューで守る) |
| S3a | calc の計算・一括・逆算を domain・app・adapters へ(判断は ADR-0252) | 2026-10-12 完了(calc を未移行の一覧から外した) |
| S3b | calc の調整(`adjust.go`)の手順を app へ(ADR-0252 追記 S3b) | 2026-10-12 完了(データレーンが代行) |
| S4 | record のお気に入り・計算履歴・失効ジョブを domain・app・adapters へ(判断は ADR-0254。G6 を解消) | 2026-10-12 完了(record を未移行の一覧から外した。データレーンが代行) |
| S5 | pokedex の検索・内部 API・readiness・read model を domain・app・adapters へ(判断は ADR-0180。importer は対象外) | 2026-10-12 完了(pokedex を未移行の一覧から外した) |
| S6 | team の構築の検証・手順・失効ジョブを domain・app・adapters へ(判断は ADR-0255) | 2026-10-12 完了(team を未移行の一覧から外した。データレーンが代行) |
| S7 | gateway の httpapi を adapters へ。domain・app は doc.go だけ(転送だけ) | 2026-10-12 完了(データレーンが代行。gateway を未移行の一覧から外した) |
| S2・S8・S9 | judge・balance・speed の移行 | 未着手 |
| W0〜W2・I0 | Web の import の向きの検査、iOS のテスト | 未着手 |
