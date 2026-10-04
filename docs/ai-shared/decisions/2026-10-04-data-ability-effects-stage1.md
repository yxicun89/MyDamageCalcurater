## 2026-10-04: 未対応の特性の段階1を効果データと engine で計算に入れる(データ・engine。ADR-0176)
Decision: AbilityEffect に段階1の項目(TypeConvert・PowerMods・AuraType/AuraMod・StatMods・SeparateStatMods・CritDamageMod・PreventsCritical・IgnoresOpponentRanks・IgnoresDefenderAbility・Breakable)を足し、連鎖の順と丸めを oracle(@smogon/calc 0.12.0 の Champions 世代)に合わせる。威力は フィールド → PowerMods → オーラ → タイプ変換 → 持ち物、実数値は 攻撃側の特性 → 防御側の特性 → 持ち物(はりきりはランクの直後に単独で丸める)、最終補正は 壁 → 急所の補正 → 抜群の軽減 → 持ち物 → きのみ。タイプ変換・かたやぶり・急所無効は計算の最初に1回だけ決める。未対応の印の定義には段階1では Breakable を付けない。
Reason: ユーザーの実使用で「フェアリースキンが未対応」と報告された(F-04)。技のフラグや HP に依存しない特性は、効果データと engine だけで oracle と全件一致させられる。
Impact:
- API・Web・iOS: 契約(MasterEffect は自由形式)は不変。結果の未対応の印が減り、数値が oracle どおりに変わる。WASM の境界は新しいキーを camelCase(入れ子は PascalCase も)で受ける。
- データ・デプロイ順: アプリ(calc-svc・Web の WASM)→ master-release で再取り込み(古いアプリは新しいキーを未知のフィールドとして拒否するため)。
- 後続: 段階2(技のフラグ。パンクロック等)・段階3(HP 条件)。
