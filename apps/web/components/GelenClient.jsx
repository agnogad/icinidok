"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import Link from "next/link";
import { MainColumn, PageShell } from "../components/Frame";
import {
  Ban,
  CheckCheck,
  Loader,
  Messages,
  ArrowLeft,
  Search,
  Send,
} from "../components/Icons";
import { api } from "../lib/api";
import useSession from "../lib/useSession";

const POLL_MS = 5000;

function formatStamp(value) {
  if (!value) return "";
  try {
    const date = new Date(value);
    const today = new Date();
    const sameDay = date.toDateString() === today.toDateString();
    return new Intl.DateTimeFormat("tr-TR", sameDay
      ? { hour: "2-digit", minute: "2-digit" }
      : { day: "numeric", month: "short", hour: "2-digit", minute: "2-digit" }
    ).format(date);
  } catch {
    return "";
  }
}

function roleLabel(role) {
  return role === "owner" ? "Paylaşımına yazıyor" : "Paylaşımın için yazdın";
}

export default function GelenClient() {
  const { loading, user } = useSession();
  const [conversations, setConversations] = useState([]);
  const [activeId, setActiveId] = useState("");
  const [active, setActive] = useState(null);
  const [messages, setMessages] = useState([]);
  const [filter, setFilter] = useState("all");
  const [query, setQuery] = useState("");
  const [draft, setDraft] = useState("");
  const [sending, setSending] = useState(false);
  const [error, setError] = useState("");
  const [listLoaded, setListLoaded] = useState(false);
  const threadRef = useRef(null);
  const lastIdRef = useRef("");

  const loadConversations = useCallback(async () => {
    try {
      const data = await api("/api/conversations");
      setConversations(data.conversations || []);
    } catch {
      // Keep the previous list on a failed poll.
    } finally {
      setListLoaded(true);
    }
  }, []);

  useEffect(() => {
    if (!user) return undefined;
    loadConversations();
    const timer = setInterval(loadConversations, POLL_MS);
    return () => clearInterval(timer);
  }, [user, loadConversations]);

  // Open the conversation named in ?sohbet= once the list is known.
  useEffect(() => {
    if (activeId || typeof window === "undefined") return;
    const wanted = new URLSearchParams(window.location.search).get("sohbet");
    if (wanted) setActiveId(wanted);
  }, [activeId]);

  const openConversation = useCallback(async (id, { scroll = true } = {}) => {
    try {
      const after = scroll ? "" : lastIdRef.current;
      const data = await api(`/api/conversations/${encodeURIComponent(id)}${after ? `?after=${encodeURIComponent(after)}` : ""}`);
      setActive(data.conversation);
      setMessages((current) => {
        if (scroll || !after) return data.messages || [];
        const known = new Set(current.map((item) => item.id));
        return [...current, ...(data.messages || []).filter((item) => !known.has(item.id))];
      });
      if (scroll) {
        lastIdRef.current = (data.messages || []).at(-1)?.id || "";
        setConversations((current) =>
          current.map((item) => (item.id === id ? { ...item, unread: 0 } : item))
        );
      }
      return data;
    } catch (err) {
      setError(err.message);
      return null;
    }
  }, []);

  useEffect(() => {
    if (!activeId || !user) return undefined;
    let alive = true;
    openConversation(activeId);
    const timer = setInterval(async () => {
      const data = await openConversation(activeId, { scroll: false });
      if (!alive || !data) return;
      loadConversations();
    }, POLL_MS);
    return () => {
      alive = false;
      clearInterval(timer);
    };
  }, [activeId, user, openConversation, loadConversations]);

  useEffect(() => {
    const node = threadRef.current;
    if (node) node.scrollTop = node.scrollHeight;
  }, [messages]);

  async function send(event) {
    event.preventDefault();
    const body = draft.trim();
    if (!body || !active || sending || active.status !== "open") return;
    setSending(true);
    setError("");
    try {
      const data = await api(`/api/conversations/${encodeURIComponent(active.id)}/messages`, {
        method: "POST",
        body: { body },
      });
      setMessages((current) => [...current, data.message]);
      lastIdRef.current = data.message.id;
      setDraft("");
      loadConversations();
    } catch (err) {
      setError(err.message);
    } finally {
      setSending(false);
    }
  }

  async function closeConversation() {
    if (!active) return;
    try {
      const data = await api(`/api/conversations/${encodeURIComponent(active.id)}/close`, { method: "POST" });
      setActive(data.conversation);
      loadConversations();
    } catch (err) {
      setError(err.message);
    }
  }

  const needle = query.trim().toLocaleLowerCase("tr");
  const visible = conversations.filter((item) => {
    if (filter === "unread" && !item.unread) return false;
    if (filter === "open" && item.status !== "open") return false;
    if (!needle) return true;
    return `${item.post_title} ${item.peer_name} ${item.preview}`
      .toLocaleLowerCase("tr")
      .includes(needle);
  });

  const unreadTotal = conversations.reduce((sum, item) => sum + (item.unread || 0), 0);

  const list = (
    <section className="inbox-card">
      <div className="inbox-top">
        <h2>Sohbetler</h2>
        <label className="inbox-search">
          <span className="sr-only">Konuşmalarda ara</span>
          <Search />
          <input
            type="search"
            placeholder="Sohbetlerde ara"
            value={query}
            onChange={(event) => setQuery(event.target.value)}
          />
        </label>
        <div className="inbox-filters" aria-label="Sohbet filtresi">
          <button
            type="button"
            className={`inbox-filter ${filter === "all" ? "is-active" : "is-idle"}`}
            onClick={() => setFilter("all")}
          >
            Tümü
          </button>
          <button
            type="button"
            className={`inbox-filter ${filter === "unread" ? "is-active" : "is-idle"}`}
            onClick={() => setFilter("unread")}
          >
            Okunmamış ({unreadTotal})
          </button>
          <button
            type="button"
            className={`inbox-filter ${filter === "open" ? "is-active" : "is-idle"}`}
            onClick={() => setFilter("open")}
          >
            Açık
          </button>
        </div>
      </div>
      <div className="inbox-list">
        {!listLoaded && (
          <ul aria-label="Yükleniyor">
            {[0, 1, 2].map((item) => (
              <li key={item} className="inbox-skeleton">
                <span className="skeleton-avatar" />
                <span className="skeleton-lines">
                  <span className="skeleton-line is-short" />
                  <span className="skeleton-line is-long" />
                </span>
              </li>
            ))}
          </ul>
        )}
        {listLoaded && conversations.length === 0 && (
          <p className="inbox-none">
            Henüz sohbet yok. Bir dert çekip ya da bir paylaşıma yazarak başla.
          </p>
        )}
        {listLoaded && conversations.length > 0 && visible.length === 0 && (
          <p className="inbox-none">Bu süze uygun sohbet bulunamadı.</p>
        )}
        <ul aria-label="Sohbetler">
          {visible.map((item) => (
            <li key={item.id}>
              <button
                type="button"
                className={`inbox-item${item.id === activeId ? " is-active" : ""}`}
                onClick={() => setActiveId(item.id)}
              >
                <span className="inbox-avatar" aria-hidden="true">
                  {item.peer_name.slice(0, 1).toUpperCase()}
                </span>
                <span className="inbox-item-body">
                  <span className="inbox-item-top">
                    <span className="inbox-peer">@{item.peer_name}</span>
                    <span className="inbox-time">{formatStamp(item.last_message_at)}</span>
                  </span>
                  <span className="inbox-post">{item.post_title}</span>
                  <span className="inbox-preview">{item.preview || roleLabel(item.role)}</span>
                </span>
                {item.unread > 0 && <span className="inbox-unread">{item.unread}</span>}
                {item.status === "closed" && <span className="inbox-closed">kapalı</span>}
              </button>
            </li>
          ))}
        </ul>
      </div>
    </section>
  );

  const thread = (
    <div className={`inbox-empty${activeId ? " is-open" : ""}`}>
      <div className="inbox-empty-inner">
        {!user && !loading && (
          <div className="inbox-empty-box">
            <span className="inbox-empty-icon">
              <Messages strokeWidth={1.4} />
            </span>
            <h1>Sohbetler için giriş yap</h1>
            <p>
              Paylaşımına yazan her kişi ayrı bir sohbet olarak listeye düşer.
              Sohbete devam etmek için kullanıcı adın ve 2FA koduyla giriş yap.
            </p>
            <Link className="btn btn-dark" href="/giris">
              Giriş yap
            </Link>
          </div>
        )}
        {user && !active && (
          <div className="inbox-empty-box">
            <span className="inbox-empty-icon">
              <Messages strokeWidth={1.4} />
            </span>
            <h1>Bir konuşma seç</h1>
            <p>
              Paylaşımına yazan her kişi ayrı bir konuşma olarak soldaki listeye
              düşer. İlk cevabını verdiğinde 24 saatlik sohbet başlar.
            </p>
          </div>
        )}
        {active && (
          <div className="thread">
            <header className="thread-head">
              <button
                type="button"
                className="thread-back only-mobile"
                onClick={() => {
                  setActiveId("");
                  setActive(null);
                  setMessages([]);
                  lastIdRef.current = "";
                  // Drop ?sohbet= so the open-on-load effect does not
                  // immediately reopen the conversation we just left.
                  if (typeof window !== "undefined") {
                    const url = new URL(window.location.href);
                    url.searchParams.delete("sohbet");
                    window.history.replaceState({}, "", url);
                  }
                }}
                aria-label="Sohbet listesine dön"
              >
                <ArrowLeft />
              </button>
              <div className="thread-title">
                <h1>@{active.peer_name}</h1>
                <p>
                  <span className={`thread-role role-${active.role}`}>{roleLabel(active.role)}</span>
                  <span className="thread-post">· {active.post_title}</span>
                </p>
              </div>
              <div className="thread-actions">
                {active.status === "open" ? (
                  <button type="button" className="btn btn-outline thread-close" onClick={closeConversation}>
                    <Ban />
                    Kapat
                  </button>
                ) : (
                  <span className="thread-closed">Kapalı</span>
                )}
              </div>
            </header>

            <div className="thread-body" ref={threadRef}>
              {messages.length === 0 && <p className="thread-hint">Henüz mesaj yok. İlk mesajı sen yaz.</p>}
              {messages.map((message) => {
                const mine = message.sender_id === user?.id;
                return (
                  <div key={message.id} className={`bubble${mine ? " is-mine" : ""}`}>
                    <p>{message.body}</p>
                    <time dateTime={message.created_at}>{formatStamp(message.created_at)}</time>
                  </div>
                );
              })}
            </div>

            {active.status === "open" ? (
              <form className="thread-compose" onSubmit={send}>
                {error && (
                  <p className="auth-error" role="alert">
                    {error}
                  </p>
                )}
                <div className="thread-input">
                  <textarea
                    rows={2}
                    maxLength={1000}
                    placeholder="Sakin ve saygılı yaz…"
                    value={draft}
                    onChange={(event) => setDraft(event.target.value)}
                    onKeyDown={(event) => {
                      if (event.key === "Enter" && !event.shiftKey) {
                        event.preventDefault();
                        send(event);
                      }
                    }}
                  />
                  <button type="submit" className="thread-send" aria-label="Gönder" disabled={sending || !draft.trim()}>
                    {sending ? <Loader className="auth-spin" /> : <Send />}
                  </button>
                </div>
                <p className="thread-note">
                  <CheckCheck />
                  <span>Mesajlar 24 saat boyunca kalır. Saygısız bir sohbeti kapatabilirsin.</span>
                </p>
              </form>
            ) : (
              <p className="thread-locked" role="note">
                <Ban />
                <span>Bu sohbet kapatıldı. Artık mesaj yazılamaz.</span>
              </p>
            )}
          </div>
        )}
      </div>
    </div>
  );

  return (
    <PageShell
      active="gelen"
      aside={
        <aside aria-label="Gelen kutusu" className="inbox-aside">
          {list}
        </aside>
      }
    >
      <MainColumn>
        <div className={`inbox-mobile${activeId ? " is-hidden" : ""}`}>
          <div className="inbox-mobile-pad">{list}</div>
        </div>
        {thread}
      </MainColumn>
    </PageShell>
  );
}
