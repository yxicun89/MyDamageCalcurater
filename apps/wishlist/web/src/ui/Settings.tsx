import { useState, type CSSProperties } from "react";
import type { FetchType, Genre, GenreUpdate, Item, Site } from "../api/types";
import type { ApiClient } from "../lib/api";
import { formatAliasGroup, parseAliasGroups } from "../lib/aliases";
import { isValidSearchTemplate } from "../lib/deeplink";
import {
  COLOR_SWATCHES,
  DEFAULT_GENRE_COLOR,
  GENRE_ICONS,
  GenreGlyph,
  UiIcon,
  genreColorProps,
  type GenreIconName,
} from "./icons";
import { ErrorText, Modal, errorMessage } from "./Modal";

const FETCH_TYPES: FetchType[] = ["link_only", "api", "scrape", "headless"];

/** カンマ(, 、)か改行で区切る。前後の空白と空の項目は除く。 */
const parseList = (text: string): string[] =>
  text
    .split(/[,、\n]/)
    .map((x) => x.trim())
    .filter((x) => x !== "");
const sameList = (a: readonly string[], b: readonly string[]) => JSON.stringify(a) === JSON.stringify(b);

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
  const [color, setColor] = useState(genre?.color ?? DEFAULT_GENRE_COLOR);
  const [icon, setIcon] = useState<GenreIconName>(genre?.icon ?? "box");
  const [hosts, setHosts] = useState((genre?.match_hosts ?? []).join("\n"));
  const [keywords, setKeywords] = useState((genre?.match_keywords ?? []).join("\n"));
  const originalAliases = genre?.aliases ?? [];
  const [aliasLines, setAliasLines] = useState<string[]>(originalAliases.map(formatAliasGroup));
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
    const { groups, invalidRow } = parseAliasGroups(aliasLines);
    if (invalidRow !== null) {
      setError(`別名グループ${String(invalidRow)}は2語以上をカンマで区切って入力してください`);
      return;
    }
    setBusy(true);
    setError(null);
    try {
      if (genre) {
        const patch: GenreUpdate = {};
        if (name.trim() !== genre.name) patch.name = name.trim();
        if (template !== genre.query_template) patch.query_template = template;
        if (siteIds.join(",") !== genre.site_ids.join(",")) patch.site_ids = siteIds;
        if (JSON.stringify(groups) !== JSON.stringify(originalAliases)) patch.aliases = groups;
        if (color !== genre.color) patch.color = color;
        if (icon !== genre.icon) patch.icon = icon;
        if (!sameList(parseList(hosts), genre.match_hosts)) patch.match_hosts = parseList(hosts);
        if (!sameList(parseList(keywords), genre.match_keywords)) patch.match_keywords = parseList(keywords);
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
          aliases: groups,
          color,
          icon,
          match_hosts: parseList(hosts),
          match_keywords: parseList(keywords),
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
    <Modal label="ジャンル" className="tall" onClose={onClose}>
      <h2>ジャンル</h2>
      <div className="genre-edit-head">
        <span className="genre-icon big" {...genreColorProps(color)}>
          <GenreGlyph icon={icon} size={32} />
        </span>
        <label className="field grow">
          <span>ジャンル名</span>
          <input
            type="text"
            value={name}
            onChange={(e) => {
              setName(e.target.value);
            }}
          />
        </label>
      </div>
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
      <h3>色</h3>
      <div className="swatches" role="group" aria-label="色">
        {COLOR_SWATCHES.map((c) => (
          <button
            key={c.color}
            type="button"
            className="swatch"
            aria-label={c.label}
            aria-pressed={color === c.color}
            style={{ background: c.color, "--swatch-color": c.color } as CSSProperties}
            onClick={() => {
              setColor(c.color);
            }}
          >
            {color === c.color && <UiIcon name="check" size={18} strokeWidth={3} />}
          </button>
        ))}
      </div>
      <h3>絵柄</h3>
      <div className="icon-pick" role="group" aria-label="絵柄">
        {GENRE_ICONS.map((i) => (
          <button
            key={i.icon}
            type="button"
            className="genre-button large"
            aria-label={i.label}
            aria-pressed={icon === i.icon}
            {...genreColorProps(color)}
            onClick={() => {
              setIcon(i.icon);
            }}
          >
            <GenreGlyph icon={i.icon} size={24} />
          </button>
        ))}
      </div>
      <h3>ホームでの見え方</h3>
      <div className="genre-preview" data-testid="genre-preview" {...genreColorProps(color)}>
        <GenreGlyph icon={icon} size={28} />
      </div>
      <h3>見るサイト</h3>
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
      <h3>URL の手がかり</h3>
      <label className="field">
        <span>ホスト(カンマか改行で区切る)</span>
        <textarea
          aria-label="手がかりのホスト"
          rows={2}
          value={hosts}
          onChange={(e) => {
            setHosts(e.target.value);
          }}
        />
      </label>
      <label className="field">
        <span>語(カンマか改行で区切る)</span>
        <textarea
          aria-label="手がかりの語"
          rows={2}
          value={keywords}
          onChange={(e) => {
            setKeywords(e.target.value);
          }}
        />
      </label>
      <h3>別名グループ</h3>
      {aliasLines.map((line, i) => (
        <div key={i} className="check-row">
          <input
            type="text"
            aria-label={`別名グループ${String(i + 1)}`}
            value={line}
            onChange={(e) => {
              setAliasLines((cur) => cur.map((l, j) => (j === i ? e.target.value : l)));
            }}
          />
          <button
            type="button"
            aria-label={`別名グループ${String(i + 1)}を削除`}
            onClick={() => {
              setAliasLines((cur) => cur.filter((_, j) => j !== i));
            }}
          >
            削除
          </button>
        </div>
      ))}
      <button
        type="button"
        onClick={() => {
          setAliasLines((cur) => [...cur, ""]);
        }}
      >
        別名グループを追加
      </button>
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
  genres: Genre[];
  items: Item[];
  sites: Site[];
  client: ApiClient;
  onGenres: (g: Genre[]) => void;
  onSites: (s: Site[]) => void;
  onBack: () => void;
}

export function SettingsScreen({ genres, items, sites, client, onGenres, onSites, onBack }: Props) {
  const [genreEdit, setGenreEdit] = useState<Genre | "new" | null>(null);
  const [siteEdit, setSiteEdit] = useState<Site | "new" | null>(null);
  const sortedGenres = [...genres].sort((a, b) => a.sort_order - b.sort_order || a.id - b.id);

  return (
    <main className="settings">
      <header className="settings-head">
        <button type="button" className="square-button" aria-label="戻る" onClick={onBack}>
          <UiIcon name="back" />
        </button>
        <h1>設定</h1>
      </header>

      <section aria-label="ジャンル">
        <h2>ジャンル</h2>
        <ul className="plain">
          {sortedGenres.map((g) => (
            <li key={g.id} className="list-row genre-row">
              <span className="genre-icon" data-testid="genre-icon" {...genreColorProps(g.color)}>
                <GenreGlyph icon={g.icon} size={20} />
              </span>
              <span className="row-name">{g.name}</span>
              <span className="row-count" data-testid="genre-count">
                {String(items.filter((i) => i.genre_id === g.id).length)}
              </span>
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
