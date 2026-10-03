## 2026-10-03: API 契約・SQL の生成物を Git に置かない(ユーザー決定。ADR-0806)
Decision: Go・TypeScript・iOS の生成物(`scripts/ensure-gen.sh list`)を追跡から外し、使う側の前段で自動に生成する(`make gen` は test・lint・build・Docker・CI の前、Web は npm の pre フック、iOS は `make ios-*` の前で `ios-gen`)。Xcode で直接開く前・`go build` を直接使う前は、それぞれ `make ios-gen`・`make gen` を1回実行する。iOS だけ追跡を残して merge ドライバで衝突を解く案は却下した(新しい生成対象に追従できない・生成器の版を取り違える・clone ごとの設定に依存・rebase や GitHub 上のマージで効かない)。
Reason: API を変える PR どうしが生成物で衝突し、再生成し忘れも起きた(ios-gen-check が赤)。
Impact: 全レーン。未マージのブランチは、main を取り込むときに COORDINATION.md「生成物を追跡から外したあとの取り込み」の手順(`git rm -r --cached` → `make gen GEN_FORCE=1`)で解く。
