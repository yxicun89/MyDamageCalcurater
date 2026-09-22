import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { App } from "./App";
import { apiBaseUrl } from "./api/config";
import { exampleMasterSource } from "./master/exampleSource";
import { createOnlineMasterSource } from "./master/onlineSource";
import type { MasterSources } from "./master/types";
import "./styles/tokens.css";

const container = document.getElementById("root");
if (container === null) {
  throw new Error("index.html に #root が無い");
}
createRoot(container).render(
  <StrictMode>
    <App
      // P4-16(ADR-0304 §追記 A-6・A-8): オフラインは架空の例データ、オンラインは pokedex-svc の
      // 公開 API から読む(createOnlineMasterSource)。App がマウント時に1回だけ呼ぶ(ids はそのとき渡る)。
      masterSources={(ids): MasterSources => ({
        offline: exampleMasterSource,
        online: createOnlineMasterSource({
          baseUrl: apiBaseUrl(),
          fetch: globalThis.fetch.bind(globalThis),
          ids,
        }),
      })}
    />
  </StrictMode>,
);
