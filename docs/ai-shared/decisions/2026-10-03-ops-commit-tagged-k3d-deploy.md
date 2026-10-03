## 2026-10-03: k3d デプロイをコミット識別のタグにした(運用 issue #291・ADR-0806。素早さレーンが空席代行)
Decision: `*-k3d-deploy`(api・web・balance・speed・judge)は `scripts/k3d-deploy-tagged.sh` 経由で、`:local` ではなく `scripts/image-tag.sh` のタグ
(コミット12桁・未コミットなら `-dirty`)で image を入れ、Pod の image から版が分かり `kubectl rollout undo` で戻せるようにした。
Reason: `:local` の上書きではビルド元が分からず、ロールバックも効かなかった。
Impact: 手順は `docs/runbooks/rollback.md`。overlay の `newTag: local` は変えていない(実行時に image だけ置換)。実クラスタでの確認は未実施。
