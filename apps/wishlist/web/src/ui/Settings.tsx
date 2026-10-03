import { useState } from "react";
import type { FetchType, Genre, Site } from "../api/types";
import type { ApiClient } from "../lib/api";
import { isHttpUrl, isValidSearchTemplate } from "../lib/deeplink";
import type { Settings as SettingsValue } from "../lib/settings";
import { ErrorText, Modal, errorMessage } from "./Modal";

const FETCH_TYPES: FetchType[] = ["link_only", "api", "scrape", "headless"];

interface GenreDialogProps {
  genre: Genre | null;
  genres: Genre[];
  sites: Site[];
  client: ApiClient;
  onClose: () => void;
  onSaved: (genres: Genre[]) => void;
}

function GenreDialog({ genre, genres, sites, client, onClose, onSaved }: GenreDialogProps) {
  const [name, setName] = useState(genre?.name ?? "");
  const [template, setTemplate] = useState(genre?.query_template ?? "{name} {option}");
  const [siteIds, setSiteIds] = useState<number[]>(genre?.site_ids ?? []);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  // 選択済み(表示順)を先に、未選択をあとに並べる。
  const ordered = [
    ...siteIds.flatMap((id) => sites.filter((s) => s.id === id)),
    ...sites.filter((s) => !siteIds.includes(s.id)),
  ];

  const toggle = (id: number) => {
    setSiteIds((cur) => (cur.includes(id) ? cur.filter((x) => x !== id) : [...cur, id]));
  };
  const moveUp = (id: number) => {
    setSiteIds((cur) => {
      const i = cur.indexOf(id);
      if (i <= 0) return cur;
      const next = [...cur];
      [next[i - 1], next[i]] = [cur[i] as number, cur[i - 1] as number];
      return next;
    });
  };

  const save = async () => {
    setBusy(true);
    setError(null);
    try {
      if (genre) {
        const patch: { name?: string; query_template?: string; site_ids?: number[] } = {};
        if (name.trim() !== genre.name) patch.name = name.trim();
        if (template !== genre.query_template) patch.query_template = template;
        if (siteIds.join(",") !== genre.site_ids.join(",")) patch.site_ids = siteIds;
        const updated = Object.keys(patch).length === 0 ? genre : await client.updateGenre(genre.id, patch);
        onSaved(genres.map((g) => (g.id === genre.id ? updated : g)));
      } else {
        const sortOrder = genres.reduce((m, g) => Math.max(m, g.sort_order), 0) + 1;
        // default を持つ項目(query_template・sort_order)は省略せず送る。
        const created = await client.createGenre({
          name: name.trim(),
          query_template: template,
          sort_order: sortOrder,
          site_ids: siteIds,
        });
        onSaved([...genres, created]);
      }
      onClose();
    } catch (e) {
      setError(errorMessage(e));
    } finally {
      setBusy(false);
    }
  };

  return (
    <Modal label="ジャンル" onClose={onClose}>
      <h2>ジャンル</h2>
      <label className="field">
        <span>ジャンル名</span>
        <input
          type="text"
          value={name}
          onChange={(e) => {
            setName(e.target.value);
          }}
        />
      </label>
      <label className="field">
        <span>検索ワードのテンプレート</span>
        <input
          type="text"
          value={template}
          onChange={(e) => {
            setTemplate(e.target.value);
          }}
        />
      </label>
      <ul className="plain">
        {ordered.map((s) => {
          const checked = siteIds.includes(s.id);
          return (
            <li key={s.id} className="check-row">
              <label>
                <input
                  type="checkbox"
                  checked={checked}
                  onChange={() => {
                    toggle(s.id);
                  }}
                />
                {s.name}
              </label>
              {checked && (
                <button
                  type="button"
                  aria-label={`${s.name}を上へ`}
                  disabled={siteIds[0] === s.id}
                  onClick={() => {
                    moveUp(s.id);
                  }}
                >
                  ↑
                </button>
              )}
            </li>
          );
        })}
      </ul>
      <ErrorText message={error} />
      <div className="actions">
        <button type="button" onClick={onClose}>
          キャンセル
        </button>
        <button
          type="button"
          className="primary"
          disabled={busy || name.trim() === ""}
          onClick={() => void save()}
        >
          保存
        </button>
      </div>
    </Modal>
  );
}

interface SiteDialogProps {
  site: Site | null;
  sites: Site[];
  client: ApiClient;
  onClose: () => void;
  onSaved: (sites: Site[]) => void;
}

function SiteDialog({ site, sites, client, onClose, onSaved }: SiteDialogProps) {
  const [name, setName] = useState(site?.name ?? "");
  const [template, setTemplate] = useState(site?.search_url_template ?? "");
  const [fetchType, setFetchType] = useState<FetchType>(site?.fetch_type ?? "link_only");
  const [isReference, setIsReference] = useState(site?.is_reference ?? false);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  const save = async () => {
    if (!isValidSearchTemplate(template)) {
      setError("検索URLのテンプレートは http(s) の URL で、{q} を含めてください");
      return;
    }
    setBusy(true);
    setError(null);
    try {
      if (site) {
        const patch: {
          name?: string;
          search_url_template?: string;
          fetch_type?: FetchType;
          is_reference?: boolean;
        } = {};
        if (name.trim() !== site.name) patch.name = name.trim();
        if (template !== site.search_url_template) patch.search_url_template = template;
        if (fetchType !== site.fetch_type) patch.fetch_type = fetchType;
        if (isReference !== site.is_reference) patch.is_reference = isReference;
        const updated = Object.keys(patch).length === 0 ? site : await client.updateSite(site.id, patch);
        onSaved(sites.map((s) => (s.id === site.id ? updated : s)));
      } else {
        const created = await client.createSite({
          name: name.trim(),
          search_url_template: template,
          fetch_type: fetchType,
          is_reference: isReference,
        });
        onSaved([...sites, created]);
      }
      onClose();
    } catch (e) {
      setError(errorMessage(e));
    } finally {
      setBusy(false);
    }
  };

  return (
    <Modal label="サイト" onClose={onClose}>
      <h2>サイト</h2>
      <label className="field">
        <span>サイト名</span>
        <input
          type="text"
          value={name}
          onChange={(e) => {
            setName(e.target.value);
          }}
        />
      </label>
      <label className="field">
        <span>検索URLのテンプレート</span>
        <input
          type="text"
          inputMode="url"
          value={template}
          onChange={(e) => {
            setTemplate(e.target.value);
          }}
        />
      </label>
      <label className="field">
        <span>取得方式</span>
        <select
          value={fetchType}
          onChange={(e) => {
            setFetchType(e.target.value as FetchType);
          }}
        >
          {FETCH_TYPES.map((t) => (
            <option key={t} value={t}>
              {t}
            </option>
          ))}
        </select>
      </label>
      <label className="check-row">
        <input
          type="checkbox"
          checked={isReference}
          onChange={(e) => {
            setIsReference(e.target.checked);
          }}
        />
        基準価格に使う
      </label>
      <ErrorText message={error} />
      <div className="actions">
        <button type="button" onClick={onClose}>
          キャンセル
        </button>
        <button
          type="button"
          className="primary"
          disabled={busy || name.trim() === ""}
          onClick={() => void save()}
        >
          保存
        </button>
      </div>
    </Modal>
  );
}

interface Props {
  settings: SettingsValue;
  genres: Genre[];
  sites: Site[];
  client: ApiClient;
  onSaveSettings: (s: SettingsValue) => void;
  onGenres: (g: Genre[]) => void;
  onSites: (s: Site[]) => void;
  onBack: () => void;
}

export function SettingsScreen({
  settings,
  genres,
  sites,
  client,
  onSaveSettings,
  onGenres,
  onSites,
  onBack,
}: Props) {
  const [apiUrl, setApiUrl] = useState(settings.apiBaseUrl ?? "");
  const [token, setToken] = useState(settings.token);
  const [connError, setConnError] = useState<string | null>(null);
  const [genreEdit, setGenreEdit] = useState<Genre | "new" | null>(null);
  const [siteEdit, setSiteEdit] = useState<Site | "new" | null>(null);
  const sortedGenres = [...genres].sort((a, b) => a.sort_order - b.sort_order || a.id - b.id);

  return (
    <main className="settings">
      <header className="topbar">
        <button type="button" className="text-button" onClick={onBack}>
          戻る
        </button>
        <h1>設定</h1>
      </header>

      <section aria-label="接続">
        <label className="field">
          <span>APIのURL</span>
          <input
            type="text"
            inputMode="url"
            placeholder="空なら、このページと同じ場所"
            value={apiUrl}
            onChange={(e) => {
              setApiUrl(e.target.value);
            }}
          />
        </label>
        <label className="field">
          <span>トークン</span>
          <input
            type="password"
            autoComplete="off"
            value={token}
            onChange={(e) => {
              setToken(e.target.value);
            }}
          />
        </label>
        <button
          type="button"
          className="primary"
          onClick={() => {
            const url = apiUrl.trim();
            if (url !== "" && !isHttpUrl(url)) {
              setConnError("APIのURLは http(s) で始まる URL にしてください");
              return;
            }
            setConnError(null);
            onSaveSettings({ apiBaseUrl: url === "" ? null : url, token: token.trim() });
          }}
        >
          保存
        </button>
        {connError !== null && <p role="alert">{connError}</p>}
      </section>

      <section aria-label="ジャンル">
        <h2>ジャンル</h2>
        <ul className="plain">
          {sortedGenres.map((g) => (
            <li key={g.id} className="list-row">
              <span>{g.name}</span>
              <button
                type="button"
                aria-label={`${g.name}を編集`}
                onClick={() => {
                  setGenreEdit(g);
                }}
              >
                編集
              </button>
            </li>
          ))}
        </ul>
        <button
          type="button"
          onClick={() => {
            setGenreEdit("new");
          }}
        >
          ジャンルを追加
        </button>
      </section>

      <section aria-label="サイト">
        <h2>サイト</h2>
        <ul className="plain">
          {sites.map((s) => (
            <li key={s.id} className="list-row">
              <span>{s.name}</span>
              <button
                type="button"
                aria-label={`${s.name}を編集`}
                onClick={() => {
                  setSiteEdit(s);
                }}
              >
                編集
              </button>
            </li>
          ))}
        </ul>
        <button
          type="button"
          onClick={() => {
            setSiteEdit("new");
          }}
        >
          サイトを追加
        </button>
      </section>

      {genreEdit !== null && (
        <GenreDialog
          genre={genreEdit === "new" ? null : genreEdit}
          genres={genres}
          sites={sites}
          client={client}
          onClose={() => {
            setGenreEdit(null);
          }}
          onSaved={onGenres}
        />
      )}
      {siteEdit !== null && (
        <SiteDialog
          site={siteEdit === "new" ? null : siteEdit}
          sites={sites}
          client={client}
          onClose={() => {
            setSiteEdit(null);
          }}
          onSaved={onSites}
        />
      )}
    </main>
  );
}
