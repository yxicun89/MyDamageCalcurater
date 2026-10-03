## 2026-09-22: API レーンの依頼(内部 API・性格のマスタ・showdownId)を受ける(データレーン)
Decision: P2-3 で pokedex-svc に `GET /internal/pokedex/master`(ADR-0204 の契約。クラスタ内だけ、未投入なら 503)を実装し、species に showdownId を含める。
性格は、ADR-0100 の「マスタにせず engine の固定」を改め、`natures`(id, name_ja, plus, minus)をマスタに加える(新しい migration と importer)。
Reason: calc-svc がマスタを pokedex-svc から受け取る形になり(ユーザー決定 2026-09-22、API レーン)、性格の ID → 補正と日本語名が必要になった。ADR-0013 §2 の「表・一覧はデータ」とも合う。
Impact: plan.md の P2-3 に小項目を追加。ADR-0100 に更新の注記を足す(P2-3 で)。
