// P8-1c(ADR-0325): 画像があれば <img>、無ければ(未取得・404・キー無し・読み込み失敗)既存のタイプ色エンブレム。
// 画像は装飾(alt="")。名前は隣のテキストで読める。常時動くアニメーションは無い。

import { StrictMode } from "react";
import { act, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { describe, expect, test, vi } from "vitest";
import { PokemonImage } from "./PokemonImage";
import { PokemonImagesProvider } from "./PokemonImagesContext";
import { parsePokemonImageManifest } from "./pokemonImages";

const MANIFEST = parsePokemonImageManifest({
  version: 1,
  images: {
    "0445-000": { thumb: "thumb/0445-000.ab12cd34.webp", detail: "detail/0445-000.ef567890.webp" },
  },
});

function emblem(): React.ReactElement {
  return <span data-testid="type-emblem" />;
}

function renderImage(ui: React.ReactElement, manifest = MANIFEST) {
  return render(<PokemonImagesProvider manifest={manifest}>{ui}</PokemonImagesProvider>);
}

describe("PokemonImage(manifest を直接渡す)", () => {
  test("キーが manifest にあれば <img>(thumb の URL・loading=lazy・alt 空・装飾)を出し、エンブレムは出さない", () => {
    const { container } = renderImage(
      <PokemonImage speciesKey="0445-000" size="thumb" fallback={emblem()} />,
    );
    const img = container.querySelector("img");
    expect(img).not.toBeNull();
    expect(img?.getAttribute("src")).toBe("/images/thumb/0445-000.ab12cd34.webp");
    expect(img?.getAttribute("loading")).toBe("lazy");
    expect(img?.getAttribute("alt")).toBe("");
    // 装飾画像: スクリーンリーダーの画像一覧に出ない(alt 空 = role presentation)。
    expect(screen.queryByRole("img")).not.toBeInTheDocument();
    expect(screen.queryByTestId("type-emblem")).not.toBeInTheDocument();
  });

  test("size=detail は detail の URL", () => {
    const { container } = renderImage(
      <PokemonImage speciesKey="0445-000" size="detail" fallback={emblem()} />,
    );
    expect(container.querySelector("img")?.getAttribute("src")).toBe("/images/detail/0445-000.ef567890.webp");
  });

  test("CLS を避けるため幅・高さの属性(または固定寸法のクラス)を持つ: 寸法の指定が1つも無い <img> にしない", () => {
    const { container } = renderImage(
      <PokemonImage speciesKey="0445-000" size="thumb" fallback={emblem()} />,
    );
    const img = container.querySelector("img");
    const hasSize =
      (img?.hasAttribute("width") === true && img.hasAttribute("height")) || (img?.className ?? "") !== "";
    expect(hasSize).toBe(true);
  });

  test("manifest に無いキーはエンブレム(<img> は出さない)", () => {
    const { container } = renderImage(
      <PokemonImage speciesKey="9999-000" size="thumb" fallback={emblem()} />,
    );
    expect(container.querySelector("img")).toBeNull();
    expect(screen.getByTestId("type-emblem")).toBeInTheDocument();
  });

  test("manifest が null(未取得・失敗)ならエンブレム", () => {
    const { container } = renderImage(
      <PokemonImage speciesKey="0445-000" size="thumb" fallback={emblem()} />,
      null,
    );
    expect(container.querySelector("img")).toBeNull();
    expect(screen.getByTestId("type-emblem")).toBeInTheDocument();
  });

  test("Provider が無くてもエンブレム(既存画面・既存テストはそのまま動く)", () => {
    const { container } = render(<PokemonImage speciesKey="0445-000" size="thumb" fallback={emblem()} />);
    expect(container.querySelector("img")).toBeNull();
    expect(screen.getByTestId("type-emblem")).toBeInTheDocument();
  });

  test("画像の読み込みに失敗(onError)したらエンブレムに戻り、<img> は消える", () => {
    const { container } = renderImage(
      <PokemonImage speciesKey="0445-000" size="thumb" fallback={emblem()} />,
    );
    const img = container.querySelector("img");
    expect(img).not.toBeNull();
    fireEvent.error(img as HTMLImageElement);
    expect(container.querySelector("img")).toBeNull();
    expect(screen.getByTestId("type-emblem")).toBeInTheDocument();
  });

  test("別のキーに変わったら、前のキーの読み込み失敗を引きずらない", () => {
    const manifest = parsePokemonImageManifest({
      version: 1,
      images: {
        "0445-000": { thumb: "thumb/0445-000.ab12cd34.webp", detail: "detail/0445-000.ef567890.webp" },
        "0006-000": { thumb: "thumb/0006-000.11111111.webp", detail: "detail/0006-000.22222222.webp" },
      },
    });
    const { container, rerender } = render(
      <PokemonImagesProvider manifest={manifest}>
        <PokemonImage speciesKey="0445-000" size="thumb" fallback={emblem()} />
      </PokemonImagesProvider>,
    );
    fireEvent.error(container.querySelector("img") as HTMLImageElement);
    expect(container.querySelector("img")).toBeNull();
    rerender(
      <PokemonImagesProvider manifest={manifest}>
        <PokemonImage speciesKey="0006-000" size="thumb" fallback={emblem()} />
      </PokemonImagesProvider>,
    );
    expect(container.querySelector("img")?.getAttribute("src")).toBe("/images/thumb/0006-000.11111111.webp");
  });
});

describe("PokemonImagesProvider(manifest を取りに行く)", () => {
  function fetchReturning(status: number, body: unknown): ReturnType<typeof vi.fn<typeof fetch>> {
    return vi.fn<typeof fetch>(() =>
      Promise.resolve(new Response(typeof body === "string" ? body : JSON.stringify(body), { status })),
    );
  }

  const MANIFEST_JSON = {
    version: 1,
    images: {
      "0445-000": { thumb: "thumb/0445-000.ab12cd34.webp", detail: "detail/0445-000.ef567890.webp" },
    },
  };

  test("取得中はエンブレム、取得できたら <img> に切り替わる", async () => {
    let resolve: (value: Response) => void = () => undefined;
    const fetchMock = vi.fn<typeof fetch>(
      () =>
        new Promise<Response>((r) => {
          resolve = r;
        }),
    );
    const { container } = render(
      <PokemonImagesProvider fetch={fetchMock}>
        <PokemonImage speciesKey="0445-000" size="thumb" fallback={emblem()} />
      </PokemonImagesProvider>,
    );
    expect(screen.getByTestId("type-emblem")).toBeInTheDocument();
    expect(container.querySelector("img")).toBeNull();
    await act(async () => {
      resolve(new Response(JSON.stringify(MANIFEST_JSON), { status: 200 }));
      await Promise.resolve();
    });
    await waitFor(() => {
      expect(container.querySelector("img")?.getAttribute("src")).toBe(
        "/images/thumb/0445-000.ab12cd34.webp",
      );
    });
    expect(screen.queryByTestId("type-emblem")).not.toBeInTheDocument();
  });

  test("manifest の取得は1回だけ(画像を使う部品が複数あっても・StrictMode でも)", async () => {
    const fetchMock = fetchReturning(200, MANIFEST_JSON);
    const { container } = render(
      <StrictMode>
        <PokemonImagesProvider fetch={fetchMock}>
          <PokemonImage speciesKey="0445-000" size="thumb" fallback={emblem()} />
          <PokemonImage speciesKey="0445-000" size="detail" fallback={emblem()} />
        </PokemonImagesProvider>
      </StrictMode>,
    );
    await waitFor(() => {
      expect(container.querySelectorAll("img")).toHaveLength(2);
    });
    expect(fetchMock).toHaveBeenCalledTimes(1);
  });

  test.each<[string, number, unknown]>([
    ["404", 404, '{"error":"not_found"}'],
    ["不正な JSON", 200, "{not json"],
    ["version 違い", 200, { version: 2, images: {} }],
  ])("%s でもエンブレムのまま。alert も例外も出さない", async (_name, status, body) => {
    const fetchMock = fetchReturning(status, body);
    const errorSpy = vi.spyOn(console, "error").mockImplementation(() => undefined);
    const { container } = render(
      <PokemonImagesProvider fetch={fetchMock}>
        <PokemonImage speciesKey="0445-000" size="thumb" fallback={emblem()} />
      </PokemonImagesProvider>,
    );
    await waitFor(() => {
      expect(fetchMock).toHaveBeenCalledTimes(1);
    });
    await act(async () => {
      await Promise.resolve();
    });
    expect(container.querySelector("img")).toBeNull();
    expect(screen.getByTestId("type-emblem")).toBeInTheDocument();
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
    expect(errorSpy).not.toHaveBeenCalled();
    errorSpy.mockRestore();
  });

  test("fetch が例外(ネットワーク失敗)でもエンブレムのまま", async () => {
    const fetchMock = vi.fn<typeof fetch>(() => Promise.reject(new TypeError("Failed to fetch")));
    render(
      <PokemonImagesProvider fetch={fetchMock}>
        <PokemonImage speciesKey="0445-000" size="thumb" fallback={emblem()} />
      </PokemonImagesProvider>,
    );
    await waitFor(() => {
      expect(fetchMock).toHaveBeenCalledTimes(1);
    });
    expect(screen.getByTestId("type-emblem")).toBeInTheDocument();
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });

  test("アンマウント後に取得が終わっても例外・警告を出さない", async () => {
    let resolve: (value: Response) => void = () => undefined;
    const fetchMock = vi.fn<typeof fetch>(
      () =>
        new Promise<Response>((r) => {
          resolve = r;
        }),
    );
    const errorSpy = vi.spyOn(console, "error").mockImplementation(() => undefined);
    const { unmount } = render(
      <PokemonImagesProvider fetch={fetchMock}>
        <PokemonImage speciesKey="0445-000" size="thumb" fallback={emblem()} />
      </PokemonImagesProvider>,
    );
    unmount();
    await act(async () => {
      resolve(new Response(JSON.stringify(MANIFEST_JSON), { status: 200 }));
      await Promise.resolve();
    });
    expect(errorSpy).not.toHaveBeenCalled();
    errorSpy.mockRestore();
  });
});
