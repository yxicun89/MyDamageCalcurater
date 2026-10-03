// ADR-0173: 画面の登録ファイル(`*.screen.tsx`)の整合テスト。各レーンが共有ファイルを触らずに画面を足せる代わりに、
// 重複・欠落・形の誤りはここ(と起動時の buildScreenRegistry)で検出する。

import { describe, expect, test } from "vitest";
import { defineScreen, noClient, type RegisteredScreen } from "./screenDefinition";
import { RESERVED_SEGMENTS, SCREENS, buildScreenRegistry, instantiateScreens } from "./screens";
import { DEFAULT_SCREEN } from "./routes";
import type { ClientIds } from "../api/clientIds";

function fakeScreen(
  overrides: Partial<Pick<RegisteredScreen, "id" | "segment" | "label" | "order">> = {},
): RegisteredScreen {
  return defineScreen({
    id: overrides.id ?? "fake",
    segment: overrides.segment ?? "fake",
    label: overrides.label ?? "偽の画面",
    order: overrides.order ?? 1,
    usesMaster: true,
    createClient: noClient,
    render: () => null,
  });
}

const ids: ClientIds = { deviceId: "device", sessionId: "session" };

describe("実際の登録", () => {
  test("既存の7画面が order の順に登録されている(id・segment・usesMaster)", () => {
    expect(SCREENS.map((screen) => [screen.id, screen.segment, screen.usesMaster])).toEqual([
      ["calc", "calc", true],
      ["reverse", "reverse", true],
      ["balance", "balance", true],
      ["speed", "speed", false],
      ["judge", "judge", true],
      ["team", "team", true],
      ["adjust", "adjust", true],
    ]);
  });

  test("id・segment・order はどれも重複せず、segment は予約済みでない", () => {
    const unique = (values: readonly (string | number)[]): boolean => new Set(values).size === values.length;
    expect(unique(SCREENS.map((screen) => screen.id))).toBe(true);
    expect(unique(SCREENS.map((screen) => screen.segment))).toBe(true);
    expect(unique(SCREENS.map((screen) => screen.order))).toBe(true);
    for (const screen of SCREENS) {
      expect(RESERVED_SEGMENTS).not.toContain(screen.segment);
      expect(screen.label).not.toBe("");
    }
  });

  test("既定の画面が登録されている", () => {
    expect(SCREENS.map((screen) => screen.id)).toContain(DEFAULT_SCREEN);
  });

  test("instantiateScreens は全画面の描画の口を同じ順で返し、生成時に通信しない", () => {
    const calls: string[] = [];
    const fetchSpy: typeof fetch = (input) => {
      calls.push(input instanceof Request ? input.url : input.toString());
      return Promise.reject(new Error("unexpected fetch"));
    };
    const instances = instantiateScreens(SCREENS, { baseUrl: "", fetch: fetchSpy, ids });
    expect(instances.map((instance) => [instance.id, instance.usesMaster])).toEqual(
      SCREENS.map((screen) => [screen.id, screen.usesMaster]),
    );
    expect(calls).toEqual([]);
  });
});

describe("buildScreenRegistry の検証", () => {
  test("order の昇順に並べる(登録ファイルのパスの順ではない)", () => {
    const registry = buildScreenRegistry({
      "./b.screen.tsx": fakeScreen({ id: "b", segment: "b", order: 200 }),
      "./a.screen.tsx": fakeScreen({ id: "a", segment: "a", order: 150 }),
      "./c.screen.tsx": fakeScreen({ id: "c", segment: "c", order: 100 }),
    });
    expect(registry.map((screen) => screen.id)).toEqual(["c", "a", "b"]);
  });

  test("既定エクスポートが無い登録ファイルは例外", () => {
    expect(() => buildScreenRegistry({ "./x.screen.tsx": undefined })).toThrow(/x\.screen\.tsx/);
  });

  test("id の重複は例外", () => {
    expect(() =>
      buildScreenRegistry({
        "./a.screen.tsx": fakeScreen({ id: "same", segment: "a", order: 1 }),
        "./b.screen.tsx": fakeScreen({ id: "same", segment: "b", order: 2 }),
      }),
    ).toThrow(/id "same"/);
  });

  test("segment の重複は例外", () => {
    expect(() =>
      buildScreenRegistry({
        "./a.screen.tsx": fakeScreen({ id: "a", segment: "same", order: 1 }),
        "./b.screen.tsx": fakeScreen({ id: "b", segment: "same", order: 2 }),
      }),
    ).toThrow(/segment "same"/);
  });

  test("order の重複は例外(並びが登録ファイルの読み込み順に依存しないように)", () => {
    expect(() =>
      buildScreenRegistry({
        "./a.screen.tsx": fakeScreen({ id: "a", segment: "a", order: 5 }),
        "./b.screen.tsx": fakeScreen({ id: "b", segment: "b", order: 5 }),
      }),
    ).toThrow(/order 5/);
  });

  test.each([["about"], ["Calc"], ["a/b"], [""], ["-a"]])("segment %j は例外", (segment) => {
    expect(() => buildScreenRegistry({ "./a.screen.tsx": fakeScreen({ segment }) })).toThrow();
  });

  test.each([[Number.NaN], [Number.POSITIVE_INFINITY]])("order %d は例外", (order) => {
    expect(() => buildScreenRegistry({ "./a.screen.tsx": fakeScreen({ order }) })).toThrow();
  });

  test("空の表示名は例外", () => {
    expect(() => buildScreenRegistry({ "./a.screen.tsx": fakeScreen({ label: "" }) })).toThrow();
  });
});

describe("defineScreen", () => {
  test("createClient は instantiate のときに1回だけ呼ばれ、render にそのクライアントが渡る", () => {
    let created = 0;
    const seen: number[] = [];
    const screen = defineScreen({
      id: "x",
      segment: "x",
      label: "x",
      order: 1,
      usesMaster: false,
      createClient: () => {
        created += 1;
        return created;
      },
      render: (_env, client) => {
        seen.push(client);
        return null;
      },
    });
    expect(created).toBe(0);
    const instance = screen.instantiate({ baseUrl: "", fetch: globalThis.fetch, ids });
    expect(created).toBe(1);
    if (instance.usesMaster) {
      throw new Error("usesMaster: false の定義から作った口のはず");
    }
    const env = { onlineMasterSource: { load: () => Promise.reject(new Error("unused")) } };
    expect(instance.render(env)).toBeNull();
    expect(instance.render(env)).toBeNull();
    expect(created).toBe(1);
    expect(seen).toEqual([1, 1]);
  });
});
