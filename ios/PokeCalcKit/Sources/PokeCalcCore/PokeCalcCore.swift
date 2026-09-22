// PokeCalcCore: ドメインの型・PokeCalcService・API 実装・モック(ADR-0500 §1, §3, §4, §5)。
//
// このファイル自体はモジュールの索引だけを持つ。実装は責務ごとに分けている:
//   DomainTypes.swift          ドメインの型(openapi の enum/struct と同じ意味を持つが、生成型には依存しない)
//   PokeCalcService.swift      画面が依存する唯一のプロトコル(§3)
//   PokeCalcError.swift        唯一のエラー型
//   ClientIdentity.swift       端末ID・セッションID(§5)
//   AppConfiguration.swift     起動時の接続先設定(§5)
//   APIPokeCalcService.swift   生成クライアントでの実装。生成型 ↔ ドメインの写像はここに閉じる
//   MockPokeCalcService.swift  架空データ(Resources/*.json)での実装(§4)。ダメージは計算しない
//   MockFixtures.swift         MockPokeCalcService が読む JSON の decode 用の型
//   AttackerPreset.swift       自分側の調整プリセット(§6。P6-2a)
//   BulkRowDisplay.swift       一括計算の結果行の画面向け整形・KOTier(P6-2a)
//   CalcScreenError.swift      PokeCalcError → 計算画面が出し分けるエラーの種類(P6-2a)
//   CalcViewModel.swift        ダメージ計算画面の状態(§1「画面の状態は PokeCalcCore」。P6-2a)
