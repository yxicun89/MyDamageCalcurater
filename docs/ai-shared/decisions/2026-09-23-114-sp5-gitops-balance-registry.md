## 2026-09-23: 素早さ SP5(GitOps)の設計。balance-registry の共有と改名の提案(タイプバランスレーンへ)
Decision: SP5(GitOps。ADR-0605)は speed 専用のクラスタ内レジストリを新設せず、balance が構築した balance-registry(services/balance/deploy/local-registry/。TB0・ADR-0018)を push 先として共有する(k3d ノードの containerd が localhost:5000 の1レジストリしか信頼しないため)。services/balance/ のファイルは変更しない(services/speed/scripts/local-registry-push.sh から kubectl -n balance-registry port-forward するだけ)。Argo CD も TB0 で導入済みの1インスタンスを共有し、speed が再インストールすることはしない。
提案(タイプバランスレーンへ。既定案: 今は何もしない): balance-registry という名前は今後クラスタ全体で共有されるとわかりにくいので、都合の良いときに pokecalc-registry へ改名することを検討してほしい。急ぎではない。
Reason: hostPort 5000 はノードにつき1つの Pod しか持てず、Argo CD も1クラスタに複数動かす理由が無いため。
Impact: ADR-0605。services/balance/ は変更しない。
