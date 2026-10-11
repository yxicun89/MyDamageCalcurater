## 2026-10-11: iOS の配色を「K. 空色と白」に差し替えた(G-05 / I-ios-15 / ADR-0530。Web レーンへ)
Decision: ユーザー決定(ec レーン経由・2026-10-11)の配色「K. 空色と白」を、`PokeCalcDesign` のトークンの**値だけ**差し替えて反映した(キー名・API・画面は不変)。
背景はグラデーションをやめたのではなく、`bgGradientStart`/`bgGradientEnd` を ほぼ同じ淡い空色の 2 色にした(差はわずか。トークン名は維持)。
主色は指定の #2F80ED 系のうち、白文字・淡い空色の上の文字で WCAG 4.5:1 を満たす #1D66D0 にした(#2F80ED は白に 3.87:1 で届かない)。
タイプ 18 色は HSL の彩度を 0.95 倍にし(normal・dark は無彩色で不変)、タイプ色の上の黒/白の文字は 18 色すべて 4.5:1 以上(最小は かくとう 4.71:1・どく 4.74:1)。
ダークは青みの紺(#0B1624 系)。値の表(Web はこの表に合わせると iOS と同じ色になる。Web の tokens.css と design.md は未反映のまま。iOS の PopPaletteTests・DesignTokenTests は新値で固定):

| トークン | 旧ライト | 旧ダーク | 新ライト | 新ダーク |
|---|---|---|---|---|
| bgBase | F4F5F7 | 0E1015 | EEF6FF | 0B1624 |
| bgGlass(ダーク) | - | 1A1D24 60% | - | 16253B 60% |
| textSecondary(ライト) | 5C6270 | - | 4F5B70 | - |
| brandPrimary | 1F5FD6 | 7FA8FF | 1D66D0 | 7DB4FF |
| onPrimary | FFFFFF | 0E1015 | FFFFFF | 0B1624 |
| surfaceCard | FFFFFF | 1A1D24 | FFFFFF | 16253B |
| bgGradientStart | FFF6E0 | 14131C | F4F9FF | 10203A |
| bgGradientEnd | E8F1FF | 0E1622 | E6F1FF | 0B1624 |
| tableHeader | DCE7FB | 1C2638 | D9E9FC | 1B2E4A |
| tableZebra | EEF2F8 | 151922 | E9F3FE | 122035 |
| tableHover | E3ECFB | 1D2535 | DDEBFD | 1B2C45 |
| focusRing | 1F5FD6 | 7FA8FF | 1D66D0 | 7DB4FF |
| shadowColor | 黒10% | 黒40% | #143C78 12% | 黒45% |
| matchupResist(新規・耐性の文字/枠/アイコン。水色系。ADR-0422 §3 の案のまま) | - | - | 0A5E8E | 6CCBF5 |
| matchupResistSoft(新規・耐性の背景) | - | - | D7F0FB | 0E2F40 |
| matchupImmune(新規・無効の文字/枠/アイコン。灰) | - | - | 4A5568 | AEB8C6 |
| matchupImmuneSoft(新規・無効の背景) | - | - | E6E9EE | 222B38 |

| タイプ | 旧 | 新(ライト・ダーク共通) |
|---|---|---|
| fire | #E8622B | #E36430 |
| water | #3B8FE0 | #3F8FDC |
| electric | #F2C21B | #EDBF20 |
| grass | #4CAF50 | #4EAD52 |
| ice | #5CC8D8 | #5FC6D5 |
| fighting | #D0453A | #CF4A40(彩度×0.95 だと白文字に反転するため明度を少し上げた) |
| poison | #9B51C6 | #9A54C3 |
| ground | #B8863B | #B5853E |
| flying | #7FA7E8 | #82A8E5 |
| psychic | #E8517F | #E45580 |
| bug | #93B21E | #91AE22 |
| rock | #A99A62 | #A79964 |
| ghost | #6B55A8 | #6C57A6 |
| dragon | #5160D8 | #5463D5 |
| steel | #6E93A8 | #6F93A7 |
| fairy | #E68AD6 | #E48CD4 |
| normal・dark | #9FA19F・#54474A | 変更なし(無彩色のため) |

Reason: 配色の方針転換(ユーザー決定)。コントラストの検査(`PopPaletteTests` の文字 4.5:1・枠/輪 3:1、タイプ文字 4.5:1)は弱めず値の調整で通した。
Impact: Web レーン(同じ値へ追う場合は上の表。主色は #1D66D0・ダーク #7DB4FF、気に入らなければ両者で再調整)。iOS の画面・UITest は変更なし(色は直接書かない規則)。
