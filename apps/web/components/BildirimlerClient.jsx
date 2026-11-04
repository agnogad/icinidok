"use client";

import { useCallback, useEffect, useState } from "react";
import Link from "next/link";
import { Bell, Loader, Refresh } from "../components/Icons";
import { MainColumn, PageShell } from "../components/Frame";
import { api } from "../lib/api";
import useSession from "../lib/useSession";

function formatStamp(value) {
  if (!value) return "";
  try {
    return new Intl.DateTimeFormat("tr-TR", {
      dateStyle: "medium",
      timeStyle: "short",
    }).format(new Date(value));
  } catch {
    return "";
  }
}

function linkFor(item) {
  if (item.conversation_id) return `/gelen?sohbet=${item.conversation_id}`;
  if (item.post_id) return `/dert?id=${item.post_id}`;
  return "";
}

export default function BildirimlerClient() {
  const { loading, user } = useSession();
  const [items, setItems] = useState([]);
  const [unread, setUnread] = useState(0);
  const [busy, setBusy] = useState(false);
  const [filters, setFilters] = useState("all");

  const load = useCallback(async () => {
    setBusy(true);
    try {
      const data = await api("/api/notifications");
      setItems(data.notifications || []);
      setUnread(data.unread || 0);
    } catch {
      setItems([]);
    } finally {
      setBusy(false);
    }
  }, []);

  useEffect(() => {
    if (user) load();
  }, [user, load]);

  async function markAllRead() {
    try {
      await api("/api/notifications/read", { method: "POST" });
      setItems((current) => current.map((item) => ({ ...item, read: true })));
      setUnread(0);
    } catch {
      // ignore
    }
  }

  const visible = filters === "all" ? items : items.filter((item) => !item.read);

  return (
    <PageShell active="bildirimler">
      <MainColumn>
        <div className="scroll-area">
          <div className="notif-container">
            <header className="notif-head">
              <h1>
                <Bell />
                Bildirimler
              </h1>
              <div className="notif-actions">
                <div className="notif-filters" role="group" aria-label="Bildirim filtresi">
                  <button
                    type="button"
                    className={`inbox-filter ${filters === "all" ? "is-active" : "is-idle"}`}
                    onClick={() => setFilters("all")}
                  >
                    Tümü
                  </button>
                  <button
                    type="button"
                    className={`inbox-filter ${filters === "unread" ? "is-active" : "is-idle"}`}
                    onClick={() => setFilters("unread")}
                  >
                    Okunmamış ({unread})
                  </button>
                </div>
                <button type="button" className="btn btn-outline notif-mark" onClick={markAllRead} disabled={unread === 0}>
                  Tümünü okundu işaretle
                </button>
                <button type="button" className="btn btn-outline notif-refresh" aria-label="Bildirimleri yenile" onClick={load} disabled={busy}>
                  {busy ? <Loader className="auth-spin" /> : <Refresh />}
                </button>
              </div>
            </header>
            <p className="notif-sub">
              Paylaşımlarına gelen destekler, sohbet mesajları ve güvenlik
              uyarıları.
            </p>

            {!user && !loading && (
              <p className="notif-state">
                Bildirimleri görmek için <Link href="/giris">giriş yap</Link>.
              </p>
            )}
            {user && !busy && visible.length === 0 && (
              <p className="notif-state">
                {items.length === 0 ? "Henüz bildirim yok." : "Okunmamış bildirim yok."}
              </p>
            )}

            <ul className="notif-list">
              {visible.map((item) => {
                const href = linkFor(item);
                const content = (
                  <>
                    <span className={`notif-dot kind-${item.kind}`} aria-hidden="true" />
                    <span className="notif-text">{item.text}</span>
                    <time dateTime={item.created_at}>{formatStamp(item.created_at)}</time>
                  </>
                );
                return (
                  <li key={item.id} className={`notif-item${item.read ? "" : " is-unread"}`}>
                    {href ? (
                      <Link className="notif-link" href={href}>
                        {content}
                      </Link>
                    ) : (
                      <span className="notif-link">{content}</span>
                    )}
                  </li>
                );
              })}
            </ul>
          </div>
        </div>
      </MainColumn>
    </PageShell>
  );
}
