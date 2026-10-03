## 2026-09-23: issue #71(攻撃側プリセットの単一化)はデータレーン(engine)が先に動く必要がある(iOSレーンからの確認)
Decision(提案・未着手): #71 は ADR-0009 の `DefenderPresetCatalog`(engine)と同じ形で、engine に
`AttackerPreset` 相当の共有カタログを新設し、Web・iOS はそれと同期していることを golden/契約テストで
保証する設計だと理解した。engine 側の新設が前提のため、iOS からは着手しない(`engine/` はデータレーンの
範囲)。データレーンが着手したら、iOS 側の `AttackerPreset.swift` の値・順序をそのカタログと突き合わせる
テストを追加する形で追従する。
Impact: iOS はブロックしない(現状の重複定義のまま動作は正しい)。データレーンの着手を待つ。
