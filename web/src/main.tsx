import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { App } from "./App";
import { apiBaseUrl } from "./api/config";
import { createBrowserMasterCacheStore } from "./master/cache/browserStore";
import { countSavedSpecies, createCachedMasterSources } from "./master/cache/cachedSources";
import { createOnlineMasterSource } from "./master/onlineSource";
import type { MasterSources } from "./master/types";
import "./styles/tokens.css";
import "./styles/components.css";

const container = document.getElementById("root");
if (container === null) {
  throw new Error("index.html に #root が無い");
}
// ADR-0313: 取得したマスタの保存先(IndexedDB)。オンラインの保存・オフラインの読み出し・保存済み件数の表示で共有する。
const masterCacheStore = createBrowserMasterCacheStore();
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
          store: masterCacheStore,
        })
      }
      // ADR-0358: オフライン表示の「保存済み: ○匹」。
      savedSpeciesCount={() => countSavedSpecies(masterCacheStore)}
    />
  </StrictMode>,
);
