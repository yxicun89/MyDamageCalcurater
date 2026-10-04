// ADR-0323: 画面の登録ファイル(`*.screen.tsx`)の整合テスト。各レーンが共有ファイルを触らずに画面を足せる代わりに、
// 重複・欠落・形の誤りはここ(と起動時の buildScreenRegistry)で検出する。

import { describe, expect, test } from "vitest";
import { defineScreen, noClient, type RegisteredScreen } from "./screenDefinition";
import {
  RESERVED_SEGMENTS,
  SCREENS,
  buildScreenRegistry,
  instantiateScreens,
  visibleScreens,
} from "./screens";
import { DEFAULT_SCREEN } from "./routes";
import { SCREEN_ROUTES } from "./routes";
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
  test("既存の8画面が order の順に登録されている(id・segment・usesMaster)", () => {
    expect(SCREENS.map((screen) => [screen.id, screen.segment, screen.usesMaster])).toEqual([
      ["calc", "calc", true],
      ["reverse", "reverse", true],
      ["balance", "balance", true],
      ["speed", "speed", false],
      ["judge", "judge", true],
      ["team", "team", true],
      ["favorites", "favorites", false],
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

  test("instantiateScreens は hidden でない全画面の描画の口を同じ順で返し(ADR-0330)、生成時に通信しない", () => {
    const calls: string[] = [];
    const fetchSpy: typeof fetch = (input) => {
      calls.push(input instanceof Request ? input.url : input.toString());
      return Promise.reject(new Error("unexpected fetch"));
    };
    const instances = instantiateScreens(SCREENS, { baseUrl: "", fetch: fetchSpy, ids });
    expect(instances.map((instance) => [instance.id, instance.usesMaster])).toEqual(
      visibleScreens(SCREENS).map((screen) => [screen.id, screen.usesMaster]),
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

  test("defineScreen を使わずコンポーネントを既定エクスポートにした登録ファイルは、ファイル名入りの例外", () => {
    function FooScreen() {
      return null;
    }
    expect(() => buildScreenRegistry({ "../foo/foo.screen.tsx": FooScreen })).toThrow(
      /foo\.screen\.tsx.*defineScreen/,
    );
  });

  test.each([
    ["id が文字列でない", { ...fakeScreen(), id: 1 }],
    ["segment が文字列でない", { ...fakeScreen(), segment: null }],
    ["label が文字列でない", { ...fakeScreen(), label: undefined }],
    ["order が数値でない", { ...fakeScreen(), order: "100" }],
    ["instantiate が関数でない", { ...fakeScreen(), instantiate: "x" }],
  ])("%s 登録ファイルは、ファイル名入りの例外", (_name, value) => {
    expect(() => buildScreenRegistry({ "./bad.screen.tsx": value })).toThrow(/bad\.screen\.tsx/);
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

// ADR-0330(F-07): defineScreen の省略可の hidden(既定 false)。hidden の画面は登録・検証の対象だが、
// タブ・URL・マウント・クライアント生成には出ない。hidden を外すだけで元に戻る。
describe("hidden(非表示の画面。ADR-0330)", () => {
  function dummy(hidden: boolean | undefined, createClient: () => undefined = noClient): RegisteredScreen {
    return defineScreen({
      id: "dummy",
      segment: "dummy",
      label: "ダミー",
      order: 1,
      usesMaster: true,
      ...(hidden === undefined ? {} : { hidden }),
      createClient,
      render: () => null,
    });
  }

  test("hidden の既定は false", () => {
    expect(dummy(undefined).hidden).toBe(false);
    expect(dummy(false).hidden).toBe(false);
    expect(dummy(true).hidden).toBe(true);
  });

  test("実際の登録では、判定だけが hidden で、タブの表(SCREEN_ROUTES)に判定が無い", () => {
    expect(SCREENS.filter((screen) => screen.hidden).map((screen) => screen.id)).toEqual(["judge"]);
    expect(SCREEN_ROUTES.map((route) => route.id)).not.toContain("judge");
    expect(SCREEN_ROUTES.map((route) => route.id)).toEqual([
      "calc",
      "reverse",
      "balance",
      "speed",
      "team",
      "favorites",
      "adjust",
    ]);
  });

  test("visibleScreens は hidden を除き、order の並びを保つ", () => {
    const registry = buildScreenRegistry({
      "./a.screen.tsx": fakeScreen({ id: "a", segment: "a", order: 100 }),
      "./h.screen.tsx": defineScreen({
        id: "h",
        segment: "h",
        label: "隠し",
        order: 150,
        usesMaster: true,
        hidden: true,
        createClient: noClient,
        render: () => null,
      }),
      "./b.screen.tsx": fakeScreen({ id: "b", segment: "b", order: 200 }),
    });
    expect(registry.map((screen) => screen.id)).toEqual(["a", "h", "b"]);
    expect(visibleScreens(registry).map((screen) => screen.id)).toEqual(["a", "b"]);
  });

  test("hidden の画面でも、id・segment・order の重複は検証で弾く", () => {
    const hiddenOf = (id: string, segment: string, order: number): RegisteredScreen =>
      defineScreen({
        id,
        segment,
        label: "隠し",
        order,
        usesMaster: true,
        hidden: true,
        createClient: noClient,
        render: () => null,
      });
    expect(() =>
      buildScreenRegistry({
        "./a.screen.tsx": fakeScreen({ id: "dup", segment: "a", order: 1 }),
        "./b.screen.tsx": hiddenOf("dup", "b", 2),
      }),
    ).toThrow(/id "dup"/);
    expect(() =>
      buildScreenRegistry({
        "./a.screen.tsx": fakeScreen({ id: "a", segment: "dup", order: 1 }),
        "./b.screen.tsx": hiddenOf("b", "dup", 2),
      }),
    ).toThrow(/segment "dup"/);
    expect(() =>
      buildScreenRegistry({
        "./a.screen.tsx": fakeScreen({ id: "a", segment: "a", order: 1 }),
        "./b.screen.tsx": hiddenOf("b", "b", 1),
      }),
    ).toThrow(/order 1/);
  });

  test("instantiateScreens は hidden の画面のクライアントを作らず(マウントもされない)、hidden を外すと作る", () => {
    let created = 0;
    const counting = (): undefined => {
      created += 1;
      return undefined;
    };
    const deps = { baseUrl: "", fetch: () => Promise.reject(new Error("unexpected fetch")), ids };

    expect(instantiateScreens([dummy(true, counting)], deps)).toEqual([]);
    expect(created).toBe(0);

    // 再表示は hidden を外すだけ。
    expect(instantiateScreens([dummy(false, counting)], deps).map((instance) => instance.id)).toEqual([
      "dummy",
    ]);
    expect(created).toBe(1);
  });
});
