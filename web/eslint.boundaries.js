// import の向きの検査(ADR-0811・docs/plan/architecture-adoption.md §5.2)。
//
// 段: app → screens → features → shared。下の段は上の段を import しない。
//   - features(機能)は shared だけを import する。機能 → 機能と、機能 → screens は禁止。
//   - shared は shared の中だけ。app・screens・features を import しない。
//   - domain/ は通信の生成型(api/*.gen)を import しない。
// 既知の違反は下の KNOWN_VIOLATIONS に凍結する(ファイル単位の eslint-disable は使わない)。今は空。
// 例外に無い新しい違反は `npm run lint` で失敗する。

const FEATURES = ["adjust", "speed", "team", "favorites", "record", "deviceData"];
const SHARED = ["domain", "master", "engine", "api", "ui", "i18n", "images", "styles"];

// ディレクトリ名 d への親方向の相対 import(../d・../../d/x など)に当たる指定子の型。
// 同じ階層の ./d(例: i18n/ja.ts の ./speed は i18n 自身のファイル)には当てない。
const dir = (name) =>
  [1, 2, 3].flatMap((n) => {
    const up = "../".repeat(n);
    return [`${up}${name}`, `${up}${name}/**`];
  });

const group = (name, dirs, message) => ({ name, group: dirs.flatMap(dir), message });

/** 層ごとの禁止グループ。name は例外一覧の allow で指す。 */
function featureGroups(self) {
  return [
    group(
      "features",
      FEATURES.filter((f) => f !== self),
      "機能 → 機能は禁止(§5.2)。shared に出すか、app / screens から束ねる。",
    ),
    group("screens", ["screens"], "機能 → screens は禁止。共有部品は ui/ に置く(W1)。"),
    group("app", ["app"], "機能 → app は禁止(下 → 上)。"),
  ];
}

function sharedGroups() {
  return [
    group("features", FEATURES, "shared → 機能は禁止(下 → 上)。"),
    group("screens", ["screens"], "shared → screens は禁止(下 → 上)。"),
    group("app", ["app"], "shared → app は禁止(下 → 上)。"),
  ];
}

const domainGenGroup = {
  name: "gen",
  group: ["../api/*.gen", "../../api/*.gen", "../api/*.gen.ts", "../../api/*.gen.ts"],
  message: "domain/ は通信の生成型(api/*.gen)を import しない。domain 側に型を持つ(W5)。",
};

/**
 * 凍結した既知の違反。file は web/ からの相対パス、allow は許す禁止グループ名(例: features・screens・app・gen)。
 * W0 で凍結した 11 件(W1〜W5)は W1・W2 の段で解消して空になった(ADR-0811)。新しい違反は足さず、置き場所で直す。
 * 形の例: { wave: "W1", file: "src/adjust/AdjustScreen.tsx", allow: ["screens"] }
 */
export const KNOWN_VIOLATIONS = [];

// 検査しない: テスト(結合テストは複数の段をまたいで組み立てる)と、画面の登録 *.screen.tsx
// (screens の段なので app・shared を import してよい)。
const NOT_CHECKED = ["**/*.test.{ts,tsx}", "**/*.screen.tsx"];

const toRule = (groups) => ({
  "no-restricted-imports": [
    "error",
    { patterns: groups.map(({ group: g, message }) => ({ group: g, message })) },
  ],
});

/** 層の規則を、例外のファイルだけ allow を抜いた形で出す。 */
function layerConfigs(layerFiles, groups, knownViolations) {
  const exceptions = knownViolations.filter((e) => layerFiles.some((f) => matches(f, e.file)));
  const byFile = new Map();
  for (const e of exceptions) {
    byFile.set(e.file, [...(byFile.get(e.file) ?? []), ...e.allow]);
  }
  const configs = [
    {
      files: layerFiles,
      ignores: [...byFile.keys(), ...NOT_CHECKED],
      rules: toRule(groups),
    },
  ];
  for (const [file, allowed] of byFile) {
    configs.push({
      files: [file],
      rules: toRule(groups.filter((g) => !allowed.includes(g.name))),
    });
  }
  return configs;
}

function matches(layerGlob, file) {
  const prefix = layerGlob.replace(/\/\*\*.*$/, "/");
  return file.startsWith(prefix);
}

/** 例外一覧から規則を組み立てる(例外の仕組みはテストで架空の一覧を渡して確かめる)。 */
export function buildImportBoundaryConfigs(knownViolations) {
  return [
    ...FEATURES.flatMap((f) => layerConfigs([`src/${f}/**/*.{ts,tsx}`], featureGroups(f), knownViolations)),
    ...SHARED.flatMap((d) => {
      const groups = d === "domain" ? [...sharedGroups(), domainGenGroup] : sharedGroups();
      return layerConfigs([`src/${d}/**/*.{ts,tsx}`], groups, knownViolations);
    }),
  ];
}

export const importBoundaryConfigs = buildImportBoundaryConfigs(KNOWN_VIOLATIONS);
