## 2026-10-11: 配色を「空色と白」に(ADR-0355。iOS・ec レーンへ)
Decision: ユーザー決定の K 案に合わせ、Web のトークンの**値だけ**を差し替えた(名前・用途・部品は不変)。iOS も docs/design.md の同名トークンの新しい値に合わせる。
- ライト: bg.base `#EEF6FF`、surface.card 白、brand.primary `#1F68D8`(`#2F80ED` 系は AA 不足のため明度を下げた)、text.primary `#14243A`、text.secondary `#4A5A70`、表の 3 色・背景の 2 色(ほぼ単色)も更新。
- ダーク: 紺寄り(bg.base `#0B1320`、surface.card `#16202F`、text.primary `#EAF1FB`、focus/brand `#7FB0FF`)。
- タイプ色は彩度 0.95 倍。タイプバッジの文字色はかくとうだけ白に変わった(`typeInk`)。他の文字色は不変。
Reason: 現行配色が不評。
Impact: iOS の ColorToken・typeInk の値。API・engine なし。
