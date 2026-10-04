import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { App } from "./App";
import { apiBaseUrl } from "./api/config";
import { createBrowserMasterCacheStore } from "./master/cache/browserStore";
import { createCachedMasterSources } from "./master/cache/cachedSources";
import { createOnlineMasterSource } from "./master/onlineSource";
import type { MasterSources } from "./master/types";
import "./styles/tokens.css";
import "./styles/components.css";

const container = document.getElementById("root");
if (container === null) {
  throw new Error("index.html に #root が無い");
}
createRoot(container).render(
  <StrictMode>
    <App
      // ADR-0313: オンラインは pokedex-svc の公開 API から読み、取得したマスタを IndexedDB に保存する。
      // オフラインはその保存済みのマスタだけから読む(架空の例データは使わない)。
      // App がマウント時に1回だけ呼ぶ(ids はそのとき渡る)。
      masterSources={(ids): MasterSources =>
        createCachedMasterSources({
          online: createOnlineMasterSource({
            baseUrl: apiBaseUrl(),
            fetch: globalThis.fetch.bind(globalThis),
            ids,
          }),
          store: createBrowserMasterCacheStore(),
        })
      }
    />
  </StrictMode>,
);
