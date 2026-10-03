import { describe, expect, it, vi } from "vitest";
import { registerServiceWorker } from "./registerSw";

// AC-PWA-05
describe("registerServiceWorker", () => {
  it("baseUrl 配下の sw.js を scope 付きで登録する", async () => {
    const register = vi.fn().mockResolvedValue({});
    await registerServiceWorker({
      baseUrl: "/wishlist/",
      enabled: true,
      nav: { serviceWorker: { register } as never },
    });
    expect(register).toHaveBeenCalledWith("/wishlist/sw.js", { scope: "/wishlist/" });
  });
  it("enabled=false(開発サーバー)では登録しない", async () => {
    const register = vi.fn();
    await registerServiceWorker({
      baseUrl: "/wishlist/",
      enabled: false,
      nav: { serviceWorker: { register } as never },
    });
    expect(register).not.toHaveBeenCalled();
  });
  it("serviceWorker 未対応でも例外を投げない", async () => {
    await expect(registerServiceWorker({ enabled: true, nav: {} as never })).resolves.toBeUndefined();
  });
  it("登録に失敗しても例外を投げない", async () => {
    const register = vi.fn().mockRejectedValue(new Error("boom"));
    await expect(
      registerServiceWorker({
        baseUrl: "/wishlist/",
        enabled: true,
        nav: { serviceWorker: { register } as never },
      }),
    ).resolves.toBeUndefined();
  });
});
