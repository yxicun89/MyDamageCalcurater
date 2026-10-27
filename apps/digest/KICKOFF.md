# digest(技術トレンド・ダイジェスト)起動指示

自分専用の技術トレンド・ダイジェストアプリ。この節の指示は、Claude Code(ローカル・クラウドどちらも)で
**「apps/digest/KICKOFF.md に従って進めて」** と言われたときに上から順に実行する。

## 0. 最初に読むもの

1. `apps/digest/CLAUDE.md` と `apps/digest/SPEC.md` を**全部**読む。**SPEC.md を正**として実装する
   - どちらかが無ければ実装に入らない。`docs/plan/blockers/digest.md` に「`apps/digest/SPEC.md`(と CLAUDE.md)を置いてください」と書き、
     `docs/ai-shared/state/digest.md` の Next を同じ内容にしてコミット・push して止まる(本当に先に進めない唯一のケース)
2. ルートの `CLAUDE.md` / `AGENTS.md` / `docs/ai-shared/COORDINATION.md` は**共通の運用**(ブランチ・PR・レーン)だけ従う。
   ダメ計固有の仕様・用語・技術規約(engine・golden・openapi のレーン所有など)は digest には適用しない。矛盾したら digest の仕様は `apps/digest/SPEC.md` を優先
3. `docs/ai-shared/state/digest.md` の Status / Next を見て、途中から再開する(完了済みの M は飛ばす)

## 1. 範囲

- SPEC.md「9. マイルストーン」の **M0〜M6 を順に実装**する
- **M7 は k8s マニフェストの作成まで**(適用はしない)。検証は `kustomize build` と、入れられれば `kubeconform` まで
- **M8(Flutter)と M9 は対象外**
- 変更してよいのは `apps/digest/` の中と、次のレーンのファイルだけ:
  ルート `Makefile` の `include apps/digest/Makefile` の 1 行、`docs/ai-shared/state/digest.md`、`docs/ai-shared/decisions/`、`docs/plan/blockers/digest.md`
- Go モジュールはルートの `go.work` に入れず `GOWORK=off` で扱う(wishlist と同じ)

## 2. 決定済み事項(SPEC.md「10. 未決事項」への回答。ユーザー決定 2026-10-27)

- **D1**: API キーはまだ無い。LLM は `llm.Fake` で実装・テストし、実 API を叩く確認は `-tags=live` のテストとして書くだけにする(実行しない)
- **D2**: iPhone からは Tailscale で接続(README に手順を書く)
- **D3**: Mac のスリープ中に更新が止まるのは許容する
- **D4**: クライアントは Flutter(今回は対象外)

## 3. 進め方

- ブランチは `lane/digest`(初回は `git push -u origin lane/digest`)。ローカルの作業ディレクトリは `~/MyDamageCalcurater-digest`
- **ブロッカーが出るまで止まらずに進める**。途中で確認を求めない(日中・深夜を問わない)
- 各 M は共通ワークフローで進める: quick-scanner → spec-writer(受け入れ条件と失敗するテスト)→ implementer → critic(FAIL なら implementer に戻す。最大 3 回)。
  サブエージェントは文書・k8s・スクリプトでは `model: "sonnet"`、DB・API 契約・中核ロジックでは既定(Opus)
- 各 M の完了条件を満たしたら **`make check` を通して 1 M = 1 コミット**(例: `M2: collector と URL 正規化`)
  - SPEC の `make check` は `apps/digest/Makefile` に定義する。ルートから `make digest-check` でも呼べるように、
    wishlist と同じくルート `Makefile` 末尾に `include apps/digest/Makefile` を足し、ターゲット名は `digest-` 接頭辞にする
  - 同じ失敗で 3 回ループしたら `docs/plan/blockers/digest.md` に既定案付きで書き、依存しない次の M・作業へ進む
- 各 M の後に `docs/ai-shared/state/digest.md` の Status / Next を更新する(セッションが切れても次が続きから再開できるように)
- テストを消したり弱めたりして通さない

## 4. クラウド環境での注意

- **Docker が使えない場合**、testcontainers の結合テストは build tag `integration` で分離し、ユニットテストだけで `make check` が通るようにする。
  MySQL を apt で入れられるなら、それで結合テストも回す
- **外部フィードに届かない場合**、M1 の sourcecheck は「プロキシ拒否」と「ソース側の不具合」を区別して記録する。
  拒否されたソースは `enabled` のまま残し、`verify: true` に「cloud で未検証」とコメントを付ける。**無効化はしない**
- M7 は `kustomize build` と `kubeconform`(入れられれば)での検証までにする。クラスタへの適用・課金の発生する操作はしない

## 5. 判断に迷ったとき

- SPEC に書いていない仕様は足さない。どうしても必要なら最小限で実装し、PR 説明の「判断したこと」に列挙する
  (残す価値のある判断は `docs/ai-shared/decisions/` にも 1 件 1 ファイルで書く)
- 本当に先に進めないときだけ止まり、何が必要かを具体的に `docs/plan/blockers/digest.md` に書く

## 6. 最後に

作業ブランチ `lane/digest` を push し、main 向けの PR を作る(マージはしない)。PR 説明に以下を書く:

1. 完了した M と、それぞれの完了条件の結果
2. sourcecheck の結果表(OK / 修正済み / cloud で未検証 / 無効化と理由)
3. 判断したことの一覧
4. Mac に戻ってやること(M7 の適用、`tailscale serve`、API キー設定、M8)
