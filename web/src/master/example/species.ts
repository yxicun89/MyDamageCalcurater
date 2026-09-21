// P4-2: 架空の例データ(ADR-0300 §3)。名前は「テスト」で始め、ID は「example-」で始め、
// 図鑑番号は実在と重ならない 9001 以降にする(ADR-0002)。6種族以上・5タイプ以上にまたがる。
// 画面の確認に要る組み合わせ(ADR-0300 §3): ダメージ技を2つ以上覚える種族(テストほのお)、
// 変化技を覚える種族(テストほのお・テストみず)を含める。

import type { MasterSpecies } from "../types";

export const exampleSpecies: MasterSpecies[] = [
  {
    key: "example-fire",
    dexNo: 9001,
    form: 0,
    nameJa: "テストほのお",
    types: ["fire"],
    baseStats: { hp: 80, atk: 100, def: 70, spa: 90, spd: 70, spe: 95 },
    abilities: ["example-ability-none"],
    learnset: ["example-move-tackle", "example-move-firepunch", "example-move-growl"],
  },
  {
    key: "example-water",
    dexNo: 9002,
    form: 0,
    nameJa: "テストみず",
    types: ["water"],
    baseStats: { hp: 90, atk: 75, def: 80, spa: 95, spd: 85, spe: 70 },
    abilities: ["example-ability-none"],
    learnset: ["example-move-waterblast", "example-move-growl"],
  },
  {
    key: "example-grass",
    dexNo: 9003,
    form: 0,
    nameJa: "テストくさ",
    types: ["grass"],
    baseStats: { hp: 85, atk: 90, def: 80, spa: 70, spd: 80, spe: 65 },
    abilities: ["example-ability-none"],
    learnset: ["example-move-leafcutter"],
  },
  {
    key: "example-electric",
    dexNo: 9004,
    form: 0,
    nameJa: "テストでんき",
    types: ["electric"],
    baseStats: { hp: 70, atk: 65, def: 65, spa: 105, spd: 80, spe: 110 },
    abilities: ["example-ability-adapt", "example-ability-none"],
    learnset: ["example-move-thunder"],
  },
  {
    key: "example-rocksteel",
    dexNo: 9005,
    form: 0,
    nameJa: "テストいわはがね",
    types: ["rock", "steel"],
    baseStats: { hp: 90, atk: 110, def: 130, spa: 55, spd: 80, spe: 45 },
    abilities: ["example-ability-none"],
    learnset: ["example-move-rockslide", "example-move-steelwing"],
  },
  {
    key: "example-dragonflying",
    dexNo: 9006,
    form: 0,
    nameJa: "テストりゅうひこう",
    types: ["dragon", "flying"],
    baseStats: { hp: 85, atk: 95, def: 75, spa: 95, spd: 80, spe: 95 },
    abilities: ["example-ability-none"],
    learnset: ["example-move-dragonpulse", "example-move-wingattack"],
  },
];
