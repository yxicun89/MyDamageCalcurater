// 「このアプリについて」(ADR-0314)と端末データの削除(ADR-0318)の文言。ADR-0173 で i18n/ja.ts から移した
// (ja.ts が再エクスポートする)。

/**
 * 「このアプリについて」(issue 328 / P6-18、ADR-0314)の文言。非公式の注記・データの出典4件は iOS の
 * PokeCalcCore.AboutText と一字一句同じ(正は docs/ai-shared/DECISIONS.md 2026-09-26「P6-18」と ADR-0002「責務の分離」表)。
 * 出典を増減するときは ADR-0002・ADR-0501「P6-18」・iOS と同時に直す。
 */
export const aboutText = {
  footerLinkLabel: "このアプリについて",
  pageHeading: "このアプリについて",
  unofficialHeading: "非公式表示",
  unofficialNotice:
    "このアプリは個人が私的に使うための非公式ツールです。" +
    "任天堂・クリーチャーズ・ゲームフリーク・株式会社ポケモンとは関係ありません。" +
    "ポケモン・Pokémon および関連する名称は各社の商標です。",
  dataSourcesHeading: "データの出典",
  dataSources: [
    { title: "ダメージ計算の検証", detail: "@smogon/calc(MIT License)" },
    { title: "ポケモン・技・習得技の照合", detail: "Pokémon Showdown(MIT License)" },
    { title: "日本語名・図鑑番号", detail: "PokeAPI" },
    { title: "使用可能なポケモン等の基準", detail: "Pokémon HOME・Pokémon Champions の公式情報" },
  ],
  backLabel: "計算に戻る",
} as const;

/**
 * P5-5d: 「この端末のデータを削除」(ADR-0209 §8、ADR-0318 §5)。iOS(PokeCalcCore.DeviceDataText)と同じ文言(4文目だけ Web 追加)。
 * 説明の2文目の括弧だけ Web 向け。4文目は Web 側の補足(計算は削除の成否に影響されない。絶対ルール5)。
 */
export const deviceDataText = {
  sectionHeading: "データの扱い",
  explanation: [
    "アカウントはありません。履歴・お気に入り・構築は、この端末に割り当てた ID でサーバーに保存しています。",
    "ID が変わると(ブラウザのサイトデータを消したとき)、前のデータは開けなくなります。元に戻す方法はありません。",
    "開けなくなったデータは自動的に消えます。計算の履歴は記録から90日、お気に入りと構築は最後に使った日から18か月です。",
    "削除するのはサーバーに保存したデータだけです。計算・逆算は、削除の成否にかかわらず使えます。",
  ],
  deleteButton: "この端末のデータを削除",
  confirmMessage: "履歴・お気に入り・構築をサーバーから削除します。元に戻せません。",
  confirmAction: "削除する",
  cancelAction: "キャンセル",
  deleting: "削除しています…",
  partialNotice: "まだ残っています。続けて削除します。",
  failure: "サーバーに届きませんでした。通信を確認してもう一度お試しください。",
  completed: "削除しました。",
  retryButton: "もう一度削除する",
  recordLabel: "履歴・お気に入り",
  teamLabel: "構築",
  partlyDeleted: (label: string): string => `${label}は削除済みです。`,
} as const;
