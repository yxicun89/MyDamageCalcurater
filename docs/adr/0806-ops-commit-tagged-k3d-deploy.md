# ADR-0806: k3d へのデプロイはコミット識別のタグで行う(`:local` の上書きをやめる)

- 状態: 採用(2026-10-03。issue #291)
- 関連: issue #217・PR #469(`version.Version` の `-ldflags` 埋め込みと `/healthz` の version は実装済み)、PR #403(`scripts/image-tag.sh`)、issue #295(`require-k3d-context.sh`)

## 背景
`*-k3d-deploy` は常に `pokecalc/<svc>:local` を上書きして `rollout restart` していた。Pod の image からビルド元のコミットが分からず、
ReplicaSet の履歴は同じ image 参照しか持たないので `kubectl rollout undo` しても中身は戻らなかった。

## 決定
1. 共通スクリプト `scripts/k3d-deploy-tagged.sh <caller> <overlay> <イメージ名>...` を各 `*-k3d-deploy` が呼ぶ。手順は
   context 検査(`require-k3d-context.sh`)→ `docker tag <名>:local <名>:<tag>` → `k3d image import` → overlay を `kubectl kustomize` で描画し
   image だけ `:local` から `:<tag>` へ置換して `kubectl apply -f -` → `rollout status`。
2. タグは `scripts/image-tag.sh`(HEAD の12桁。対象パスに未コミットの変更があれば `-dirty`)。対象パスは各 Makefile が `TAG_PATHS` で渡す
   (そのイメージのソースと依存する `engine` など)。
3. Git の overlay(`newTag: local`)は変えない。置換は実行時だけで、GitOps の経路・クラウド overlay に影響しない
   (issue の既定案どおり。`kustomize edit set image` は作業ツリーを書き換えるので使わない)。
4. `rollout restart` はタグが `-dirty` のときだけ行う。クリーンな同コミットの再実行は image が変わらず rollout も起きない(冪等)。
   毎回 restart すると同じ image の revision が履歴に積まれ、`rollout undo` が無意味な1つ前に戻るため。
5. gateway・calc の `/healthz` の version は `API_VERSION`(= 同じ image-tag.sh の値)で埋め込む。balance・speed・judge は独立モジュール
   (`GOWORK=off`)で `version.Version` を参照するコードが無いので ldflags は入れない(image タグで版を確認する)。
6. 戻す手順は `docs/runbooks/rollback.md`。

## 却下した案
- `:local` のまま annotation に SHA を付ける: 追跡はできるがロールバックできない。
- `kustomize edit set image`: 作業ツリーの overlay を書き換え、コミットに混入する危険がある。
- 毎回 `rollout restart`: 上記 4 のとおり履歴を汚す。

## 影響
- 古いタグのイメージが k3d ノードに溜まる(ロールバックのため意図的)。掃除はクラスタの作り直しで足りる。
- 実クラスタでの動作確認は未実施(スクリプトは偽の docker・k3d・kubectl の自動テストのみ)。
