// P5-5d(ADR-0318): 「この端末のデータを削除」の文言は i18n/ja.ts の deviceDataText に1か所だけ持つ(ハードコード禁止)。
// 正は docs/adr/0209-record-team-data-retention.md §8(完全一致)。iOS(PokeCalcCore.DeviceDataText)とは
// 説明2文目の括弧だけ違う(Web は「ブラウザのサイトデータを消したとき」)。4文目(計算は影響されない)は ADR-0318 §5 の Web 側の補足。
// ここの文言はあえてリテラル(取り違えを検出するため)。

import { describe, expect, test } from "vitest";
import { deviceDataText, recordClientText } from "./ja";

describe("deviceDataText(ADR-0209 §8)", () => {
  test("§8 の文言と完全一致する", () => {
    expect(deviceDataText.sectionHeading).toBe("データの扱い");
    expect(deviceDataText.explanation).toEqual([
      "アカウントはありません。履歴・お気に入り・構築は、この端末に割り当てた ID でサーバーに保存しています。",
      "ID が変わると(ブラウザのサイトデータを消したとき)、前のデータは開けなくなります。元に戻す方法はありません。",
      "開けなくなったデータは自動的に消えます。計算の履歴は記録から90日、お気に入りと構築は最後に使った日から18か月です。",
      "削除するのはサーバーに保存したデータだけです。計算・逆算は、削除の成否にかかわらず使えます。",
    ]);
    expect(deviceDataText.deleteButton).toBe("この端末のデータを削除");
    expect(deviceDataText.confirmMessage).toBe(
      "履歴・お気に入り・構築をサーバーから削除します。元に戻せません。",
    );
    expect(deviceDataText.deleting).toBe("削除しています…");
    expect(deviceDataText.partialNotice).toBe("まだ残っています。続けて削除します。");
    expect(deviceDataText.failure).toBe("サーバーに届きませんでした。通信を確認してもう一度お試しください。");
    expect(deviceDataText.completed).toBe("削除しました。");
  });

  test("ダイアログ・再試行・対象名の文言(iOS と同じ)", () => {
    expect(deviceDataText.confirmAction).toBe("削除する");
    expect(deviceDataText.cancelAction).toBe("キャンセル");
    expect(deviceDataText.retryButton).toBe("もう一度削除する");
    expect(deviceDataText.recordLabel).toBe("履歴・お気に入り");
    expect(deviceDataText.teamLabel).toBe("構築");
    expect(deviceDataText.partlyDeleted(deviceDataText.teamLabel)).toBe("構築は削除済みです。");
    expect(deviceDataText.partlyDeleted(deviceDataText.recordLabel)).toBe("履歴・お気に入りは削除済みです。");
  });

  test("Web 固有の語を使い、iOS 固有の語(アプリを削除)を含まない", () => {
    expect(deviceDataText.explanation.join("")).toContain("ブラウザ");
    expect(deviceDataText.explanation.join("")).not.toContain("アプリを削除");
  });

  test("recordClientText.unavailable は空でない(record_unavailable の message)", () => {
    expect(recordClientText.unavailable).not.toBe("");
  });
});
