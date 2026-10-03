import { useState } from "react";
import type { Item } from "./api/types";
import type { ApiClient } from "./lib/api";
import { loadSettings, saveSettings, type Settings } from "./lib/settings";
import { Edit } from "./ui/Edit";
import { Home } from "./ui/Home";
import { ErrorText, Modal, errorMessage } from "./ui/Modal";
import { Register } from "./ui/Register";
import { SettingsScreen } from "./ui/Settings";
import { Sheet } from "./ui/Sheet";
import { useWishlist } from "./useWishlist";
import "./styles.css";

function DeleteConfirm({
  item,
  client,
  onClose,
  onDeleted,
}: {
  item: Item;
  client: ApiClient;
  onClose: () => void;
  onDeleted: () => void;
}) {
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const remove = async () => {
    setBusy(true);
    setError(null);
    try {
      await client.deleteItem(item.id);
      onDeleted();
    } catch (e) {
      setError(errorMessage(e));
      setBusy(false);
    }
  };
  return (
    <Modal label="削除の確認" role="alertdialog" onClose={onClose}>
      <p>「{item.name}」を削除しますか?</p>
      <ErrorText message={error} />
      <div className="actions">
        <button type="button" onClick={onClose}>
          キャンセル
        </button>
        <button type="button" className="danger" disabled={busy} onClick={() => void remove()}>
          削除する
        </button>
      </div>
    </Modal>
  );
}

/** PWA のルート。画面は状態で切り替える(ルーティングのライブラリは入れない)。 */
export function App() {
  const [settings, setSettings] = useState<Settings>(loadSettings);
  const w = useWishlist(settings);
  const [view, setView] = useState<"home" | "settings">(settings.token === "" ? "settings" : "home");
  const [genreFilter, setGenreFilter] = useState<number | null>(null);
  const [sheetId, setSheetId] = useState<number | null>(null);
  const [menuId, setMenuId] = useState<number | null>(null);
  const [editId, setEditId] = useState<number | null>(null);
  const [deleteId, setDeleteId] = useState<number | null>(null);
  const [registering, setRegistering] = useState(false);

  const find = (id: number | null) => (id === null ? undefined : w.items.find((i) => i.id === id));
  const replaceItem = (next: Item) => {
    w.setItems(w.items.map((i) => (i.id === next.id ? next : i)));
  };
  const sheetItem = find(sheetId);
  const menuItem = find(menuId);
  const editItem = find(editId);
  const deleteItem = find(deleteId);

  if (view === "settings") {
    return (
      <SettingsScreen
        settings={settings}
        genres={w.genres}
        sites={w.sites}
        client={w.client}
        onSaveSettings={(s) => {
          saveSettings(s);
          setSettings(s);
        }}
        onGenres={w.setGenres}
        onSites={w.setSites}
        onBack={() => {
          setView("home");
        }}
      />
    );
  }

  return (
    <>
      <Home
        items={w.items}
        genres={w.genres}
        baseUrl={w.baseUrl}
        genreFilter={genreFilter}
        loadError={w.loadError}
        onFilter={setGenreFilter}
        onOpen={(i) => {
          setSheetId(i.id);
        }}
        onMenu={(i) => {
          setMenuId(i.id);
        }}
        onAdd={() => {
          setRegistering(true);
        }}
        onSettings={() => {
          setView("settings");
        }}
      />

      {menuItem && (
        <div
          className="overlay"
          onClick={() => {
            setMenuId(null);
          }}
        >
          <div className="panel menu" role="menu">
            <button
              type="button"
              role="menuitem"
              onClick={() => {
                setMenuId(null);
                setEditId(menuItem.id);
              }}
            >
              編集
            </button>
            <button
              type="button"
              role="menuitem"
              className="danger"
              onClick={() => {
                setMenuId(null);
                setDeleteId(menuItem.id);
              }}
            >
              削除
            </button>
          </div>
        </div>
      )}

      {sheetItem && (
        <Sheet
          key={sheetItem.id}
          item={sheetItem}
          genre={w.genres.find((g) => g.id === sheetItem.genre_id)}
          sites={w.sites}
          client={w.client}
          baseUrl={w.baseUrl}
          onClose={() => {
            setSheetId(null);
          }}
          onUpdated={replaceItem}
        />
      )}

      {registering && (
        <Register
          genres={w.genres}
          initialGenreId={genreFilter}
          client={w.client}
          onClose={() => {
            setRegistering(false);
          }}
          onCreated={(item) => {
            w.setItems([...w.items, item]);
            setRegistering(false);
          }}
        />
      )}

      {editItem && (
        <Edit
          item={editItem}
          genres={w.genres}
          client={w.client}
          onClose={() => {
            setEditId(null);
          }}
          onUpdated={replaceItem}
        />
      )}

      {deleteItem && (
        <DeleteConfirm
          item={deleteItem}
          client={w.client}
          onClose={() => {
            setDeleteId(null);
          }}
          onDeleted={() => {
            w.setItems(w.items.filter((i) => i.id !== deleteItem.id));
            setDeleteId(null);
          }}
        />
      )}
    </>
  );
}
