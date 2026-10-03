// issue #328(P6-18、Web 分。ADR-0314): 「このアプリについて」の文言は i18n/ja.ts の aboutText に1か所だけ持つ
// (ハードコード禁止)。非公式の注記・データの出典4件は iOS の PokeCalcCore.AboutText と一字一句同じ。
// 正は docs/ai-shared/DECISIONS.md 2026-09-26「P6-18」、docs/adr/0501-ios-screen-acceptance.md「P6-18」2・3章、
// docs/adr/0002-master-data-source.md「責務の分離」表。出典を増減するときはそれらと iOS を同時に直す。

import { describe, expect, test } from "vitest";
import { aboutText } from "./ja";

describe("aboutText(非公式の注記・データの出典)", () => {
  test("非公式の注記は DECISIONS.md の確定文言と完全一致する", () => {
    expect(aboutText.unofficialNotice).toBe(
      "このアプリは個人が私的に使うための非公式ツールです。" +
        "任天堂・クリーチャーズ・ゲームフリーク・株式会社ポケモンとは関係ありません。" +
        "ポケモン・Pokémon および関連する名称は各社の商標です。",
    );
    expect(aboutText.unofficialNotice).toContain("非公式");
  });

  test("データの出典はちょうど4件で、ADR-0002 の責務分離表と同じ順・同じ文言", () => {
    expect(aboutText.dataSources).toEqual([
      { title: "ダメージ計算の検証", detail: "@smogon/calc(MIT License)" },
      { title: "ポケモン・技・習得技の照合", detail: "Pokémon Showdown(MIT License)" },
      { title: "日本語名・図鑑番号", detail: "PokeAPI" },
      { title: "使用可能なポケモン等の基準", detail: "Pokémon HOME・Pokémon Champions の公式情報" },
    ]);
  });

  test("ADR-0002 に書かれていないライセンスは書かない(MIT License は @smogon/calc と Showdown の2件だけ)", () => {
    const withLicense = aboutText.dataSources.filter((source) => source.detail.includes("License"));
    expect(withLicense.map((source) => source.detail)).toEqual([
      "@smogon/calc(MIT License)",
      "Pokémon Showdown(MIT License)",
    ]);
    // PokeAPI・Pokémon HOME の項目は、ライセンスを断定する語を含まない。
    for (const source of aboutText.dataSources.slice(2)) {
      expect(source.title + source.detail).not.toMatch(/License|ライセンス|MIT|Apache|CC/);
    }
  });

  test("画面の見出し・リンク・戻る導線の語", () => {
    expect(aboutText.footerLinkLabel).toBe("このアプリについて");
    expect(aboutText.pageHeading).toBe("このアプリについて");
    expect(aboutText.unofficialHeading).toBe("非公式表示");
    expect(aboutText.dataSourcesHeading).toBe("データの出典");
    expect(aboutText.backLabel).toBe("計算に戻る");
  });
});
