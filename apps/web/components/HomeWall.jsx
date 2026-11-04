"use client";

import { useEffect, useRef, useState } from "react";
import Link from "next/link";
import {
  Bookmark,
  Loader,
  MessageCircle,
  Pen,
  Refresh,
  Search,
  Send,
  Shuffle,
  Users,
  X,
} from "./Icons";
import { api } from "../lib/api";
import useSession from "../lib/useSession";

function displayDate(post) {
  const value = post.created_at || post.datetime;
  if (value) {
    return new Intl.DateTimeFormat("tr-TR", {
      day: "numeric",
      month: "short",
      hour: "2-digit",
      minute: "2-digit",
      timeZone: "UTC",
    }).format(new Date(value));
  }
  return post.date || "";
}

function displayTitle(post) {
  const value = post.created_at || post.datetime;
  if (value) {
    return new Intl.DateTimeFormat("tr-TR", {
      dateStyle: "long",
      timeStyle: "short",
      timeZone: "UTC",
    }).format(new Date(value));
  }
  return post.title_attr || displayDate(post);
}

export default function HomeWall() {
  const { user } = useSession();
  const [wallPosts, setWallPosts] = useState([]);
  const [postsLoaded, setPostsLoaded] = useState(false);
  const [categories, setCategories] = useState(["Tümü"]);
  const [sorts, setSorts] = useState([
    { key: "gundem", label: "Gündem" },
    { key: "yeni", label: "Yeni" },
  ]);
  const [selected, setSelected] = useState(null);
  const [query, setQuery] = useState("");
  const [sort, setSort] = useState("gundem");
  const [category, setCategory] = useState("Tümü");
  const [saved, setSaved] = useState([]);
  const [supported, setSupported] = useState([]);
  const [spinning, setSpinning] = useState(false);
  const [note, setNote] = useState("");
  const [drawOpen, setDrawOpen] = useState(false);
  const [draft, setDraft] = useState("");
  const [sending, setSending] = useState(false);
  const [chatError, setChatError] = useState("");
  const dialogRef = useRef(null);
  const detailRef = useRef(null);
  const spinTimer = useRef(null);

  useEffect(() => {
    const dialog = detailRef.current;
    if (!dialog) return;
    if (selected && !dialog.open) dialog.showModal();
    if (!selected && dialog.open) dialog.close();
  }, [selected]);

  useEffect(() => {
    try {
      const stored = JSON.parse(localStorage.getItem("icini-saved") || "[]");
      if (Array.isArray(stored)) setSaved(stored);
    } catch {}
  }, []);

  // Signed-in readers get their server-side saved/supported state.
  useEffect(() => {
    if (!user) return undefined;
    let alive = true;
    api("/api/me/saved")
      .then((data) => {
        if (alive && Array.isArray(data?.posts)) {
          setSaved(data.posts.map((post) => post.id));
        }
      })
      .catch(() => {});
    return () => {
      alive = false;
    };
  }, [user]);

  // Categories come from the server so the UI and the API never drift apart.
  useEffect(() => {
    let alive = true;
    api("/api/meta")
      .then((data) => {
        if (alive && Array.isArray(data?.categories)) {
          setCategories(["Tümü", ...data.categories]);
        }
        if (alive && Array.isArray(data?.sorts) && data.sorts.length > 0) {
          setSorts(data.sorts);
        }
      })
      .catch(() => {});
    return () => {
      alive = false;
    };
  }, []);

  useEffect(() => {
    let alive = true;
    api("/api/posts")
      .then((data) => {
        if (alive && Array.isArray(data?.posts)) {
          setWallPosts(data.posts);
        }
      })
      .catch(() => {})
      .finally(() => {
        if (alive) setPostsLoaded(true);
      });
    return () => {
      alive = false;
    };
  }, []);

  useEffect(() => {
    const dialog = dialogRef.current;
    if (!dialog) return;
    if (drawOpen && !dialog.open) dialog.showModal();
    if (!drawOpen && dialog.open) dialog.close();
  }, [drawOpen]);

  useEffect(() => () => clearTimeout(spinTimer.current), []);

  const needle = query.trim().toLocaleLowerCase("tr");
  let visible = wallPosts.filter((post) => {
    if (category !== "Tümü" && post.category !== category) return false;
    if (!needle) return true;
    return `${post.title} ${post.excerpt}`
      .toLocaleLowerCase("tr")
      .includes(needle);
  });
  if (sort === "yeni") {
    visible = [...visible].sort(
      (a, b) => new Date(b.datetime).getTime() - new Date(a.datetime).getTime()
    );
  }

  async function openPost(id) {
    setChatError("");
    setDraft("");
    setSelected({ loading: true, id });
    try {
      const data = await api(`/api/posts/${encodeURIComponent(id)}`);
      setSelected({ loading: false, ...data.post, mine: data.mine, supported: data.supported, saved: data.saved, conversationId: data.conversation_id });
      setSupported((current) =>
        data.supported
          ? current.includes(id) ? current : [...current, id]
          : current.filter((item) => item !== id)
      );
      setSaved((current) =>
        data.saved
          ? current.includes(id) ? current : [...current, id]
          : current.filter((item) => item !== id)
      );
    } catch (error) {
      setSelected({
        loading: false,
        error: error.message || "Paylaşım yüklenemedi.",
      });
    }
  }

  async function toggleSaved(id) {
    const wasSaved = saved.includes(id);
    // Optimistic flip, reverted if the request fails.
    setSaved((current) =>
      wasSaved ? current.filter((item) => item !== id) : [...current, id]
    );
    if (!user) {
      const next = wasSaved
        ? saved.filter((item) => item !== id)
        : [...saved, id];
      try {
        localStorage.setItem("icini-saved", JSON.stringify(next));
      } catch {}
      return;
    }
    try {
      await api(`/api/posts/${encodeURIComponent(id)}/save`, { method: "POST" });
    } catch {
      setSaved((current) =>
        wasSaved ? [...current, id] : current.filter((item) => item !== id)
      );
    }
  }

  async function toggleSupport(id) {
    if (!user) {
      setNote("Destek vermek için giriş yap.");
      return;
    }
    const wasSupported = supported.includes(id);
    const delta = wasSupported ? -1 : 1;
    setSupported((current) =>
      wasSupported ? current.filter((item) => item !== id) : [...current, id]
    );
    setWallPosts((current) =>
      current.map((post) =>
        post.id === id ? { ...post, supports: Math.max(0, post.supports + delta) } : post
      )
    );
    setSelected((current) =>
      current && current.id === id && !current.loading
        ? { ...current, supports: Math.max(0, (current.supports || 0) + delta), supported: !wasSupported }
        : current
    );
    try {
      const data = await api(`/api/posts/${encodeURIComponent(id)}/support`, { method: "POST" });
      setWallPosts((current) =>
        current.map((post) => (post.id === id ? { ...post, supports: data.supports } : post))
      );
      setSelected((current) =>
        current && current.id === id && !current.loading
          ? { ...current, supports: data.supports, supported: data.supported }
          : current
      );
    } catch (error) {
      // Roll back on failure.
      setSupported((current) =>
        wasSupported ? [...current, id] : current.filter((item) => item !== id)
      );
      setWallPosts((current) =>
        current.map((post) =>
          post.id === id ? { ...post, supports: Math.max(0, post.supports - delta) } : post
        )
      );
      setNote(error.message || "Destek kaydedilemedi.");
    }
  }

  async function startChat(event) {
    event.preventDefault();
    if (!selected || selected.loading || sending) return;
    const body = draft.trim();
    if (body.length < 1) {
      setChatError("Kısa da olsa bir şey yaz.");
      return;
    }
    if (!user) {
      window.location.href = "/giris";
      return;
    }
    setSending(true);
    setChatError("");
    try {
      const data = await api("/api/conversations", {
        method: "POST",
        body: { post_id: selected.id, message: body },
      });
      window.location.href = `/gelen?sohbet=${data.conversation.id}`;
    } catch (error) {
      setChatError(error.message);
    } finally {
      setSending(false);
    }
  }

  function refresh() {
    clearTimeout(spinTimer.current);
    setSpinning(true);
    setNote("");
    api("/api/posts")
      .then((data) => {
        if (Array.isArray(data?.posts) && data.posts.length > 0) {
          setWallPosts(data.posts);
        }
      })
      .catch(() => {})
      .finally(() => {
        spinTimer.current = setTimeout(() => setSpinning(false), 500);
      });
  }

  return (
    <>
      <section className="wall-hero">
        <div className="hero-head">
          <div className="hero-copy">
            <h1 className="hero-title">İçinde kalmasın.</h1>
            <p className="hero-sub">İçini dök. Seni dinleyen biri var.</p>
          </div>
          <div className="hero-actions">
            <button
              type="button"
              className="btn btn-outline draw-btn"
              aria-haspopup="dialog"
              onClick={() => setDrawOpen(true)}
            >
              <Shuffle className="icon-18" strokeWidth={1.7} />
              <span>Dert çek</span>
              <span className="draw-badge">3 hak</span>
            </button>
            <Link className="btn btn-dark share-btn" href="/paylas">
              <Pen className="icon-18" strokeWidth={1.7} />
              <span>Paylaş</span>
            </Link>
          </div>
        </div>

        <form
          role="search"
          aria-label="Dert ara"
          className="wall-search"
          onSubmit={(event) => event.preventDefault()}
        >
          <div className="search-field">
            <Search aria-hidden="true" />
            <input
              aria-label="Dertlerde ara"
              placeholder="Başlık veya derdin içinde ara…"
              maxLength={80}
              value={query}
              onChange={(event) => setQuery(event.target.value)}
            />
          </div>
          <button type="submit" className="search-submit">
            Ara
          </button>
        </form>

        <div className="sort-row" role="group" aria-label="Dert sıralaması">
          {sorts.map((option) => (
            <button
              key={option.key}
              type="button"
              aria-pressed={sort === option.key}
              className={`chip ${sort === option.key ? "chip-active" : "chip-idle"}`}
              onClick={() => setSort(option.key)}
            >
              {option.label}
            </button>
          ))}
          <button
            type="button"
            aria-label="Dertleri yenile"
            className={`btn btn-outline refresh-btn${spinning ? " is-spinning" : ""}`}
            onClick={refresh}
          >
            <Refresh />
            <span className="refresh-label">Yenile</span>
          </button>
        </div>

        <div
          role="group"
          aria-label="Kategoriye göre süz"
          className="cat-row"
        >
          {categories.map((item) => (
            <button
              key={item}
              type="button"
              aria-pressed={category === item}
              className={`chip cat-chip ${
                category === item ? "chip-active" : "chip-idle"
              }`}
              onClick={() => setCategory(item)}
            >
              {item}
            </button>
          ))}
        </div>
      </section>

      <div className="wall-list">
        <section aria-label="Dertler">
          {!postsLoaded ? (
            <ul className="wall-grid" aria-label="Dertler yükleniyor">
              {[0, 1, 2, 3, 4, 5].map((item) => (
                <li key={item}>
                  <div className="post-card post-skeleton" aria-hidden="true">
                    <span className="skeleton-line is-short" />
                    <span className="skeleton-line is-long" />
                    <span className="skeleton-line is-long" />
                  </div>
                </li>
              ))}
            </ul>
          ) : visible.length > 0 ? (
            <ul className="wall-grid">
              {visible.map((post) => (
                <li key={post.id}>
                  <article
                    className={`post-card${post.folder === "mutlu" ? " is-mutlu" : ""}`}
                  >
                    <span className="post-back" aria-hidden="true" />
                    <span className="post-paper" aria-hidden="true" />
                    <div className="post-body">
                      <div className="post-top">
                        <span className="post-cat">{post.category}</span>
                        <button
                          type="button"
                          className="bookmark-btn"
                          aria-pressed={saved.includes(post.id)}
                          aria-label="Kaydet (yalnızca sen görürsün)"
                          title="Kaydet"
                          onClick={() => toggleSaved(post.id)}
                        >
                          <Bookmark
                            fill={saved.includes(post.id) ? "currentColor" : "none"}
                          />
                        </button>
                      </div>
                      <h2 className="post-title">
                        <Link
                          href={`/dert?id=${post.id}`}
                          onClick={(event) => {
                            event.preventDefault();
                            openPost(post.id);
                          }}
                        >
                          {post.title}
                        </Link>
                      </h2>
                      <p className="post-excerpt">{post.excerpt}</p>
                      <div className="post-meta">
                        <span className="post-author" title={post.author}>
                          {post.author}
                        </span>
                        <time
                          className="post-time"
                          dateTime={post.created_at || post.datetime}
                          title={displayTitle(post)}
                        >
                          {displayDate(post)}
                        </time>
                      </div>
                      <div className="post-stats" aria-label="Paylaşım istatistikleri">
                        <button
                          type="button"
                          className={`support-btn${supported.includes(post.id) ? " is-on" : ""}`}
                          aria-pressed={supported.includes(post.id)}
                          aria-label={`Destek ver (${post.supports} destek)`}
                          onClick={() => toggleSupport(post.id)}
                        >
                          <Users />
                          {post.supports} destek
                        </button>
                        <span className="post-stat-divider" aria-hidden="true" />
                        <span title="Bu paylaşım için yazan farklı kişi sayısı">
                          <MessageCircle />
                          {post.chats} sohbet
                        </span>
                      </div>
                    </div>
                  </article>
                </li>
              ))}
            </ul>
          ) : (
            <p className="wall-empty">
              Bu aramaya uygun bir dert bulunamadı. Aramayı ya da kategoriyi
              değiştirmeyi dene.
            </p>
          )}
        </section>

        <div className="wall-more">
          <button
            type="button"
            className="btn btn-outline"
            onClick={() => setNote("Şu an başka dert yok. Daha sonra tekrar bak.")}
          >
            Daha fazla göster
          </button>
          {note && <p className="wall-note">{note}</p>}
        </div>
      </div>

      <dialog
        ref={detailRef}
        aria-labelledby="detail-heading"
        className="detail-dialog"
        onClose={() => setSelected(null)}
        onClick={(event) => {
          if (event.target === detailRef.current) setSelected(null);
        }}
      >
        {selected && (
          <div className="detail-dialog-inner">
            <div className="draw-head">
              <h2 id="detail-heading" className="draw-title">
                {selected.loading ? "Paylaşım yükleniyor…" : selected.title}
              </h2>
              <button
                type="button"
                className="draw-close"
                aria-label="Kapat"
                onClick={() => setSelected(null)}
              >
                <X />
              </button>
            </div>
            {selected.loading ? (
              <p className="detail-loading">Getiriliyor…</p>
            ) : selected.error ? (
              <p className="detail-loading">{selected.error}</p>
            ) : (
              <div className="detail-dialog-body">
                <div className="detail-meta">
                  <span className="detail-cat">{selected.category}</span>
                  <span>{selected.author}</span>
                  <time dateTime={selected.created_at}>
                    {displayDate(selected)}
                  </time>
                </div>
                <p className="detail-body">{selected.body}</p>
                <div className="detail-stats">
                  <button
                    type="button"
                    className={`support-btn${supported.includes(selected.id) ? " is-on" : ""}`}
                    aria-pressed={supported.includes(selected.id)}
                    onClick={() => toggleSupport(selected.id)}
                  >
                    <Users />
                    {selected.supports} destek
                  </button>
                  <span>
                    <MessageCircle />
                    {selected.chats} sohbet
                  </span>
                </div>

                {selected.mine ? (
                  <p className="chat-self">
                    Bu senin paylaşımın. Sana yazan kişiler{" "}
                    <Link href="/gelen">gelen kutuna</Link> düşer.
                  </p>
                ) : selected.conversationId ? (
                  <Link className="btn btn-dark chat-open" href={`/gelen?sohbet=${selected.conversationId}`}>
                    <MessageCircle />
                    Sohbete devam et
                  </Link>
                ) : (
                  <form className="chat-start" onSubmit={startChat}>
                    <label className="chat-label" htmlFor="chat-draft">
                      İlk mesajını yaz; 24 saatlik sakin sohbet burada başlar.
                    </label>
                    <textarea
                      id="chat-draft"
                      className="chat-draft"
                      rows={3}
                      maxLength={1000}
                      placeholder="Seni dinliyorum, anlatmak ister misin?"
                      value={draft}
                      onChange={(event) => setDraft(event.target.value)}
                    />
                    {chatError && (
                      <p className="auth-error" role="alert">
                        {chatError}
                      </p>
                    )}
                    <button
                      type="submit"
                      className="btn btn-dark chat-send"
                      disabled={sending || !draft.trim()}
                    >
                      {sending ? <Loader className="auth-spin" /> : <Send />}
                      {sending ? "Gönderiliyor…" : user ? "Sohbeti başlat" : "Giriş yap ve yaz"}
                    </button>
                  </form>
                )}
              </div>
            )}
          </div>
        )}
      </dialog>

      <dialog
        ref={dialogRef}
        aria-labelledby="draw-heading"
        className="draw-dialog"
        onClose={() => setDrawOpen(false)}
        onClick={(event) => {
          if (event.target === dialogRef.current) setDrawOpen(false);
        }}
      >
        {drawOpen && (
          <div className="draw-inner">
            <div className="draw-head">
              <div>
                <h2 id="draw-heading" className="draw-title">
                  Bir dert sana denk gelsin.
                </h2>
                <p className="draw-sub">Bazen birini dinlemekle başlar.</p>
              </div>
              <button
                type="button"
                className="draw-close"
                aria-label="Kapat"
                onClick={() => setDrawOpen(false)}
              >
                <X />
              </button>
            </div>
            <div className="draw-card">
              <p>Günde 3 farklı dert çekmek için hesabınla giriş yap.</p>
              <Link
                className="btn btn-outline draw-login"
                href="/giris"
                onClick={() => setDrawOpen(false)}
              >
                Kullanıcı adıyla giriş yap
              </Link>
            </div>
          </div>
        )}
      </dialog>
    </>
  );
}
