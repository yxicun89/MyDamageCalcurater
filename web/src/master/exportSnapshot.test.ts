// P4-5: Web の例データを calc-svc のマスタ一式(api/openapi.yaml の MasterExport。ADR-0204)に書き出す
// (ADR-0301 §5)。calc-svc をその出力で起動すれば、Web の例データの ID がそのまま API に通る
// (オンラインの動作確認・P4-6 の E2E)。
//
// 形の規則(ADR-0204 で変わった点):
//   - 相性表は MasterExport **本体**に含める(types / typeChart)。CALC_TYPECHART_PATH は廃止され、
//     設定されていると calc-svc は起動しない(services/calc/cmd/calc/main.go)。
//   - calc-svc のローダー(services/calc/internal/master/export.go)は未知のフィールドをエラーにするので、
//     画面だけの追加フィールド(species.learnset)は書かない。種族・技・持ち物・特性・性格は
//     MasterExport の形(MasterSpecies / MasterMove / MasterItem / MasterAbility / MasterNature)にそろえる。
//   - 効果(items / abilities の effect)のキーは共通マスタ(services/internal/master/effects.go)が受け付ける
//     名前(PascalCase)。Web の DTO は camelCase なので、書き出しのときに変換する。
//
// 契約そのもの(openapi.yaml の required・PokeType・Go の ID の形式・効果のキー)との突き合わせは
// exportSnapshot.contract.test.ts(毎回ソースを読む契約テスト)。ここは形と値の対応を固定する。

import { execFileSync } from "node:child_process";
import { mkdtempSync, readFileSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { beforeAll, describe, expect, test } from "vitest";
import type { Ability } from "../engine/types";
import { typeNameJa, isTypeId } from "../i18n/ja";
import { localPath } from "../test/localPath";
import { exampleMasterSource } from "./exampleSource";
import { toCalcSnapshot } from "./exportSnapshot";
import type { MasterData } from "./types";

let master: MasterData;

beforeAll(async () => {
  master = await exampleMasterSource.load();
});

function sortedKeys(value: object): string[] {
  return Object.keys(value).sort();
}

/** Web の DTO(camelCase)の効果を、共通マスタが読む形(PascalCase)にした期待値。 */
function pascalCaseKeys(effect: object | null): Record<string, unknown> | null {
  if (effect === null) {
    return null;
  }
  return Object.fromEntries(
    Object.entries(effect).map(([key, value]) => [`${key.charAt(0).toUpperCase()}${key.slice(1)}`, value]),
  );
}

describe("toCalcSnapshot(api/openapi.yaml の MasterExport。ADR-0204)", () => {
  test("最上位は MasterExport の必須フィールドちょうど(dataVersion・types・typeChart を含む)", () => {
    const snapshot = toCalcSnapshot(master);
    expect(snapshot.schemaVersion).toBe(1);
    expect(sortedKeys(snapshot)).toEqual(
      [
        "schemaVersion",
        "dataVersion",
        "types",
        "typeChart",
        "species",
        "moves",
        "items",
        "abilities",
        "natures",
      ].sort(),
    );
  });

  test("dataVersion は空でない識別子で、例データ由来だと分かる(ADR-0002: 架空のデータだと分かる名前)", () => {
    const snapshot = toCalcSnapshot(master);
    expect(typeof snapshot.dataVersion).toBe("string");
    expect(snapshot.dataVersion.length).toBeGreaterThan(0);
    expect(snapshot.dataVersion).toContain("example");
    // 同じ入力なら毎回同じ(書き出しの結果が呼ぶたびに変わらない)。
    expect(toCalcSnapshot(master).dataVersion).toBe(snapshot.dataVersion);
  });

  describe("types(相性表のタイプ。MasterType)", () => {
    test("相性表の18タイプを過不足なく、id・sortOrder・nameJa だけで持つ", () => {
      const snapshot = toCalcSnapshot(master);
      expect(master.typeChart.types).toHaveLength(18);
      expect(snapshot.types.map((type) => type.id).sort()).toEqual([...master.typeChart.types].sort());
      for (const type of snapshot.types) {
        expect(sortedKeys(type)).toEqual(["id", "nameJa", "sortOrder"].sort());
      }
    });

    test("sortOrder は整数で重複しない(services/internal/master の TypeChartData が重複を拒む)", () => {
      const snapshot = toCalcSnapshot(master);
      const orders = snapshot.types.map((type) => type.sortOrder);
      for (const order of orders) {
        expect(Number.isInteger(order)).toBe(true);
      }
      expect(new Set(orders).size).toBe(orders.length);
    });

    test("nameJa は空でなく、i18n/ja.ts の typeNameJa と一致する", () => {
      const snapshot = toCalcSnapshot(master);
      for (const type of snapshot.types) {
        expect(type.nameJa).not.toBe("");
        expect(isTypeId(type.id)).toBe(true);
        if (isTypeId(type.id)) {
          expect(type.nameJa).toBe(typeNameJa[type.id]);
        }
      }
    });
  });

  describe("typeChart(相性表の行。MasterTypeChartEntry)", () => {
    test("18×18=324件を、attackType・defenseType・code だけで持つ(組は重複しない)", () => {
      const snapshot = toCalcSnapshot(master);
      expect(snapshot.typeChart).toHaveLength(324);
      const pairs = new Set<string>();
      for (const entry of snapshot.typeChart) {
        expect(sortedKeys(entry)).toEqual(["attackType", "code", "defenseType"].sort());
        pairs.add(`${entry.attackType}>${entry.defenseType}`);
      }
      expect(pairs.size).toBe(snapshot.typeChart.length);
    });

    test("code は 0/1/2/4 のいずれかで、master.typeChart.effectiveness と一致する(等倍の既定は 2)", () => {
      const snapshot = toCalcSnapshot(master);
      for (const entry of snapshot.typeChart) {
        expect([0, 1, 2, 4]).toContain(entry.code);
        const expected = master.typeChart.effectiveness[entry.attackType]?.[entry.defenseType] ?? 2;
        expect(entry.code, `${entry.attackType}→${entry.defenseType}`).toBe(expected);
      }
    });

    // critic指摘: 例データ(testdata/golden/typechart.json)は324組すべてを明示しているため、
    // 「組が欠けていたら等倍(2)を既定にする」という分岐が例データだけでは検証できない。
    // 一部の組をわざと欠かせた入力で、その分岐を直接確かめる。
    test("effectiveness に無い組は等倍(2)を既定にする", () => {
      const [attackType, defenseType] = master.typeChart.types;
      if (attackType === undefined || defenseType === undefined) {
        throw new Error("相性表にタイプが無い(例データが壊れている)");
      }
      const sourceRow = master.typeChart.effectiveness[attackType];
      if (sourceRow === undefined) {
        throw new Error(`${attackType} の行が無い`);
      }
      const row = Object.fromEntries(
        Object.entries(sourceRow).filter(([defendType]) => defendType !== defenseType),
      );
      const effectiveness = { ...master.typeChart.effectiveness, [attackType]: row };
      const missingPair: MasterData = {
        ...master,
        typeChart: { types: master.typeChart.types, effectiveness },
      };

      const snapshot = toCalcSnapshot(missingPair);
      const entry = snapshot.typeChart.find(
        (candidate) => candidate.attackType === attackType && candidate.defenseType === defenseType,
      );
      expect(entry, `${attackType}→${defenseType} の行が書き出されていない`).toBeDefined();
      expect(entry?.code).toBe(2);
    });

    test("attackType・defenseType は types に載っているタイプだけ", () => {
      const snapshot = toCalcSnapshot(master);
      const known = new Set(snapshot.types.map((type) => type.id));
      for (const entry of snapshot.typeChart) {
        expect(known).toContain(entry.attackType);
        expect(known).toContain(entry.defenseType);
      }
    });
  });

  describe("species(MasterSpecies)", () => {
    test("MasterSpecies の必須フィールドちょうど(learnset も types も書かない)", () => {
      const snapshot = toCalcSnapshot(master);
      expect(snapshot.species).toHaveLength(master.species.length);
      for (const species of snapshot.species) {
        expect(sortedKeys(species)).toEqual(
          [
            "key",
            "dexNo",
            "form",
            "showdownId",
            "nameJa",
            "type1",
            "type2",
            "baseStats",
            "isMega",
            "baseSpeciesKey",
            "requiredItemId",
            "abilities",
          ].sort(),
        );
      }
    });

    test("key・dexNo・form・nameJa・baseStats は例データのまま、タイプは type1 / type2(単タイプは type2 が null)", () => {
      const snapshot = toCalcSnapshot(master);
      snapshot.species.forEach((species, index) => {
        const source = master.species[index];
        expect(species.key).toBe(source?.key);
        expect(species.dexNo).toBe(source?.dexNo);
        expect(species.form).toBe(source?.form);
        expect(species.nameJa).toBe(source?.nameJa);
        expect(species.baseStats).toEqual(source?.baseStats);
        expect(species.type1).toBe(source?.types[0]);
        expect(species.type2).toBe(source?.types[1] ?? null);
      });
    });

    test("例データにメガシンカは無いので isMega は false、baseSpeciesKey・requiredItemId は null", () => {
      const snapshot = toCalcSnapshot(master);
      for (const species of snapshot.species) {
        expect(species.isMega).toBe(false);
        expect(species.baseSpeciesKey).toBeNull();
        expect(species.requiredItemId).toBeNull();
      }
    });

    test("abilities は slot(1 から連番)と abilityId の行で、例データの順を保つ", () => {
      const snapshot = toCalcSnapshot(master);
      snapshot.species.forEach((species, index) => {
        const source = master.species[index];
        expect(species.abilities).toEqual(
          (source?.abilities ?? []).map((abilityId, slotIndex) => ({ slot: slotIndex + 1, abilityId })),
        );
      });
    });
  });

  test("技は MasterMove の必須フィールドちょうど(例データに追加効果・機構は無いので effect は null・mechanisms は空配列)", () => {
    const snapshot = toCalcSnapshot(master);
    expect(snapshot.moves).toHaveLength(master.moves.length);
    snapshot.moves.forEach((move, index) => {
      const source = master.moves[index];
      expect(sortedKeys(move)).toEqual(
        ["category", "effect", "id", "mechanisms", "nameJa", "power", "priority", "type"].sort(),
      );
      expect(move.effect).toBeNull();
      expect(move.mechanisms).toEqual([]);
      expect(move.id).toBe(source?.id);
      expect(move.nameJa).toBe(source?.nameJa);
      expect(move.type).toBe(source?.type);
      expect(move.category).toBe(source?.category);
      expect(move.power).toBe(source?.power);
      expect(move.priority).toBe(source?.priority);
    });
  });

  test.each(["items", "abilities"] as const)(
    "%s は id・nameJa・effect で、効果のキーは共通マスタが読む PascalCase(補正なしは null)",
    (name) => {
      const snapshot = toCalcSnapshot(master);
      expect(snapshot[name]).toHaveLength(master[name].length);
      snapshot[name].forEach((entry, index) => {
        const source = master[name][index];
        expect(sortedKeys(entry)).toEqual(["effect", "id", "nameJa"].sort());
        expect(entry.id).toBe(source?.id);
        expect(entry.nameJa).toBe(source?.nameJa);
        // 例データの効果は入れ子を持たない(statMods のキーは engine と同じ小文字のまま)。
        expect(entry.effect).toEqual(pascalCaseKeys(source?.effect ?? null));
      });
    },
  );

  // critic指摘: 例データの特性はdefAbsorbTypesを持たないため、入れ子の効果を変換する分岐が
  // 例データだけでは検証できない。共有の例データ(web/src/master/example/)は変えず、
  // このテストだけの使い捨ての特性を1件差し込んで確かめる。
  test("特性の defAbsorbTypes(入れ子の効果)も、外側のタイプIDは変換せず中身だけ PascalCase にする", () => {
    const absorbingAbility: Ability = {
      id: "exampleabilityabsorbfortest",
      nameJa: "テスト吸収(テスト専用)",
      effect: { defAbsorbTypes: { water: { healNumerator: 1, healDenominator: 4 } } },
    };
    const withAbsorb: MasterData = {
      ...master,
      abilities: [...master.abilities, absorbingAbility],
    };

    const snapshot = toCalcSnapshot(withAbsorb);
    const entry = snapshot.abilities.find((candidate) => candidate.id === absorbingAbility.id);
    expect(entry, "差し込んだ特性が書き出されていない").toBeDefined();
    expect(entry?.effect).toEqual({
      // 外側のキー(water。タイプID)は変換しない。値(AbsorbEffect)の中だけ PascalCase にする。
      DefAbsorbTypes: { water: { HealNumerator: 1, HealDenominator: 4 } },
    });
  });

  test("性格は id・nameJa・plus・minus(無補正は plus・minus とも null)", () => {
    const snapshot = toCalcSnapshot(master);
    expect(snapshot.natures).toEqual(master.natures);
    for (const nature of snapshot.natures) {
      expect(sortedKeys(nature)).toEqual(["id", "minus", "nameJa", "plus"].sort());
    }
  });

  test("例データのすべての ID が同じ順で書き出され、JSON にしても変わらない", () => {
    const snapshot = toCalcSnapshot(master);
    expect(snapshot.species.map((entry) => entry.key)).toEqual(master.species.map((entry) => entry.key));
    expect(snapshot.moves.map((entry) => entry.id)).toEqual(master.moves.map((entry) => entry.id));
    expect(snapshot.items.map((entry) => entry.id)).toEqual(master.items.map((entry) => entry.id));
    expect(snapshot.abilities.map((entry) => entry.id)).toEqual(master.abilities.map((entry) => entry.id));
    expect(snapshot.natures.map((entry) => entry.id)).toEqual(master.natures.map((entry) => entry.id));
    expect(JSON.parse(JSON.stringify(snapshot))).toEqual(snapshot);
  });

  test("入力のマスタを書き換えない", () => {
    const before = JSON.stringify(master);
    toCalcSnapshot(master);
    expect(JSON.stringify(master)).toBe(before);
  });
});

describe("web/scripts/export-example-master.mjs", () => {
  test("引数のパスに toCalcSnapshot(例データ) の JSON を書く", () => {
    const webRoot = localPath("../../", import.meta.url);
    const outDir = mkdtempSync(join(tmpdir(), "pokecalc-export-"));
    const outPath = join(outDir, "master.json");
    try {
      execFileSync(process.execPath, [join(webRoot, "scripts", "export-example-master.mjs"), outPath], {
        cwd: webRoot,
        stdio: "pipe",
        timeout: 60_000,
      });
      const written = JSON.parse(readFileSync(outPath, "utf8")) as unknown;
      expect(written).toEqual(toCalcSnapshot(master));
    } finally {
      rmSync(outDir, { recursive: true, force: true });
    }
  }, 90_000);
});
