# ADR-0306: ルートの `make e2e` を実 E2E に接続し、k3d 依存のスモークは現在のコンテキストで分岐する

- 状態: 採用(2026-09-25。issue #72 への対応。Web レーンが着手。API レーンと共同担当)
- 日付: 2026-09-25
- 関連: ADR-0300(Web アーキテクチャ §9 Web のテストを make test / lint / build に組み込む)、ADR-0203(gateway の k3d デプロイとスモーク)、
  ADR-0302(Web コンテナ)、docs/test-strategy.md「L5 E2E」、issue #72

## 背景
ルートの `make e2e` は help に「k3d 上のスモーク + Playwright」と書かれているが、実体の `scripts/e2e.sh` は全4行で、
`echo "e2e: (P4-6 で スモーク + Playwright を実装)"` を出して `exit 0` する未実装スタブのままだった。
一方で P4-6 は `docs/plan.md` で完了済み(`[x]`)になっており、実装済みの E2E は `web/Makefile` に揃っている
(`web-e2e` / `web-e2e-online` / `web-e2e-balance` / `web-e2e-container` / `web-k3d-smoke` / `web-k3d-e2e`)。
API レーンにも `services/gateway/Makefile` の `api-smoke` がある。

結果として、ルートのコマンド契約だけが中身と食い違い、`make e2e` が「テスト0件で成功」を返す。
これは AGENTS.md の「未実装ターゲットの正常終了を成功と数えない」に反し、「E2E を実施した」という誤認を生む。

`scripts/e2e.sh` はレーン横断の共有ファイルで、呼び先は Web レーンと API レーンの両方にまたがるため、
接続方針を ADR に残す。ADR 番号は COORDINATION.md のレーンごとの帯(Web は `0300〜`)に従い、着手したレーンの帯から取る。

## 決定

### 1. 常に実行する3件(k3d クラスタが要らない)
`make e2e` は、まず以下を **必ずこの順で** 実行する。いずれも既存クラスタを要さず、`make dev` と同じ「k8s を使わない開発ループ」で動く。

| 順 | ターゲット | 中身 | 追加の前提 |
|---|---|---|---|
| 1 | `web-e2e` | オフライン(WASM)の Playwright | なし(`wasm` / `web-deps` は依存で自動) |
| 2 | `web-e2e-online` | オンライン(API)の Playwright。例データで calc-svc を自前起動 | Go(`doctor.sh` の必須ツール) |
| 3 | `web-e2e-balance` | タイプバランスの Playwright。例データで balance-svc を自前起動 | Go(同上) |

呼び出しは **必ず `make <ターゲット>` 経由**にし、`npm run` / `npx playwright` を `scripts/e2e.sh` へ写経しない。
起動方法の正は `web/Makefile` と `web/package.json` の1か所に保つ(二重管理しない)。

`web-e2e-container` はここに入れない。Docker イメージのビルドを毎回伴って遅く、確かめている対象(SPA fallback・MIME・キャッシュ・gzip の
配信設定)はクラスタがあるときの `web-k3d-smoke` と重なるため。必要なときに `make web-e2e-container` を単体で叩く。

### 2. k3d 依存の3件は「現在のコンテキストが k3d-$(CLUSTER) のときだけ」実行する
既存クラスタ(`make up` 済み)を要する以下は、条件付きで追加実行する。

| 順 | ターゲット | 中身 |
|---|---|---|
| 4 | `api-smoke` | gateway 経由の API スモーク(curl) |
| 5 | `web-k3d-smoke` | k3d 上の Web の配信スモーク(curl) |
| 6 | `web-k3d-e2e` | gateway 経由の入口を実ブラウザで操作する E2E |

判定は `web-k3d-deploy`・ルート Makefile の `import-k8s` と同じ既存パターンを使う:
`kubectl config current-context` が `k3d-$(CLUSTER)`(既定 `pokecalc`)と一致するか。

- 一致しない / `kubectl` が無い / コンテキスト未設定 → **飛ばすが、黙っては飛ばさない**。
  何を飛ばしたか(3つのターゲット名)と、どうすれば実行できるか(`make up` の後に `make e2e`)を出力する。
- 一致する → 3件を追加で実行する。

「無ければスキップし、明示する」形にする理由:

1. クラスタが無い環境で `make e2e` が即座に落ちると、通常の開発ループ(`make dev` の思想)で E2E をまったく回せなくなる。
   issue #72 が嫌っている「0件で成功」の逆の失敗(常に赤で、誰も見なくなる)を招く。
2. 前例がある。`scripts/codex-review.sh` は codex が無ければスキップし、`scripts/check-publishable.sh` は
   前提が無い検査を `note`(スキップの明示)にして続行する。本 ADR はその形を、判定条件だけ既存の
   `kubectl config current-context` チェックに置き換えて使う。
3. 他レーンの作業を壊さない。`make e2e` はレーン横断の共有ターゲットで、k3d クラスタは他レーンと共有の資源である。
   クラスタの有無に関係なく同じコマンドが使える方が、レーンをまたぐ利用者の手が止まらない。

**スキップを成功と数えない**ための担保は次の2つに置く:

- 常時実行の3件は、どんな環境変数でも飛ばせない(スキップの逃げ道を作らない)。したがって `make e2e` が
  「1件も実行せずに成功」で終わることはない。
- リリース前など「k3d 分まで含めて確かめた」と言いたいときのために `E2E_REQUIRE_K3D=1` を用意する。
  このとき、コンテキストが `k3d-$(CLUSTER)` でなければスキップせず非0で終わる。

### 3. 失敗の扱いとクラスタへの影響
- `scripts/e2e.sh` は `set -euo pipefail` と `cd "$(git rev-parse --show-toplevel)"` から始め、
  **1件でも失敗したらそこで非0で終わる**(以降のターゲットを流さない)。どのターゲットで落ちたかを出力に出す。
- `make e2e` はクラスタを作らない・消さない・変えない。`make up` / `make down` / `deploy-latest` / `*-k3d-deploy` を呼ばず、
  `kubectl` も読み取り(`config current-context`)だけに使う。クラスタ・DB の削除は人間の確認が要る操作
  (CLAUDE.md「人間の確認が必要なこと」)。
- ルート Makefile の `e2e` は `CLUSTER=$(CLUSTER) ./scripts/e2e.sh` の形で `CLUSTER` を渡す
  (`CLUSTER ?= pokecalc` の既定値は環境へ自動で出ないため。`make e2e CLUSTER=別名` を効かせる)。

### 4. テスト
`scripts/e2e_test.sh` を追加し、`make test-scripts`(= `make test` に含まれる)から流す。
既存の `scripts/argocd-bootstrap_test.sh` と同じ方式(手書きの ok/ng ヘルパー・bats を足さない)で、
`make` と `kubectl` を PATH 先頭の偽物へ差し替え、**実ブラウザ・実クラスタ・実ネットワークには触らない**。
使い捨ての git リポジトリへコピーして流し、`MAKE` 環境変数も偽物へ向ける。

固定する振る舞い: 常時3件の実行と順序 / コンテキスト一致時の6件と順序 / 不一致・kubectl 無し・コンテキスト未設定・
`CLUSTER` 名違いでのスキップと明示メッセージ / 各ターゲットの失敗が非0で伝播し以降を流さないこと /
どんな環境変数でも0件成功にできないこと / `E2E_REQUIRE_K3D=1` の挙動 / ルート Makefile の配線。

## 影響
- Web レーン: `scripts/e2e.sh` の実装、ルート Makefile の `e2e`・`test-scripts` の1行ずつ。
- API レーン: `api-smoke` を `make e2e` から呼ばれる側になる。`services/gateway/Makefile` と
  `services/gateway/scripts/smoke.sh` は変更しない(呼び方だけが増える)。
- 他レーン: `make test` の所要時間は増えない(`e2e_test.sh` は偽物の下で走る単体テスト)。
  `make e2e` の所要時間は増えるが、これまでが0件だったので実質的な回帰はない。

## 代案(採らなかったもの)
- **k3d 依存の3件も常に実行する**: クラスタが無い環境で `make e2e` が必ず落ちる。開発ループで E2E を回せなくなる。
- **k3d 依存の3件を `make e2e` から外し、別ターゲット(`make e2e-k3d`)にする**: help の「k3d 上のスモーク + Playwright」と
  食い違ったままになり、issue #72 の「contract と中身の食い違い」を別の形で残す。ただし §2 のスキップ表示と
  `E2E_REQUIRE_K3D=1` があれば実質的に同じ使い分けができる。
- **`kubectl get nodes` まで叩いて疎通も見る**: コンテキストが残ったままクラスタが消えている場合に親切だが、
  判定が重くなり、既存の `web-k3d-deploy` / `import-k8s` のチェックと形が揃わなくなる。まずはコンテキスト一致だけにし、
  クラスタが消えていた場合は `api-smoke` が明示的に失敗する(スキップではなく失敗として出る)ことに任せる。
