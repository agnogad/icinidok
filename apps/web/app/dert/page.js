"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { MainColumn, PageShell } from "../../components/Frame";
import { Loader, MessageCircle, Send, Users } from "../../components/Icons";
import { api } from "../../lib/api";
import useSession from "../../lib/useSession";

function formatDate(value) {
  if (!value) return "";
  try {
    return new Intl.DateTimeFormat("tr-TR", {
      day: "numeric",
      month: "long",
      hour: "2-digit",
      minute: "2-digit",
    }).format(new Date(value));
  } catch {
    return "";
  }
}

export default function DertPage() {
  const { user } = useSession();
  const [id, setId] = useState("");
  const [post, setPost] = useState(null);
  const [state, setState] = useState("loading");
  const [supported, setSupported] = useState(false);
  const [draft, setDraft] = useState("");
  const [sending, setSending] = useState(false);
  const [error, setError] = useState("");

  useEffect(() => {
    const params = new URLSearchParams(window.location.search);
    const fromQuery = params.get("id");
    if (fromQuery) {
      setId(fromQuery);
      return;
    }
    const parts = window.location.pathname.split("/").filter(Boolean);
    setId(decodeURIComponent(parts[parts.length - 1] || ""));
  }, []);

  useEffect(() => {
    if (!id) return undefined;
    let alive = true;
    setState("loading");
    api(`/api/posts/${encodeURIComponent(id)}`)
      .then((data) => {
        if (!alive) return;
        setPost({ ...data.post, mine: data.mine, conversationId: data.conversation_id });
        setSupported(Boolean(data.supported));
        setState("ready");
      })
      .catch((err) => {
        if (alive) setState(err.status === 404 ? "missing" : "error");
      });
    return () => {
      alive = false;
    };
  }, [id]);

  async function toggleSupport() {
    if (!post || !user) return;
    const wasSupported = supported;
    setSupported(!wasSupported);
    setPost((current) => ({
      ...current,
      supports: Math.max(0, current.supports + (wasSupported ? -1 : 1)),
    }));
    try {
      const data = await api(`/api/posts/${encodeURIComponent(post.id)}/support`, { method: "POST" });
      setSupported(data.supported);
      setPost((current) => ({ ...current, supports: data.supports }));
    } catch {
      setSupported(wasSupported);
      setPost((current) => ({
        ...current,
        supports: Math.max(0, current.supports + (wasSupported ? 1 : -1)),
      }));
    }
  }

  async function startChat(event) {
    event.preventDefault();
    if (!post || sending) return;
    if (!user) {
      window.location.href = "/giris";
      return;
    }
    setSending(true);
    setError("");
    try {
      const data = await api("/api/conversations", {
        method: "POST",
        body: { post_id: post.id, message: draft.trim() },
      });
      window.location.href = `/gelen?sohbet=${data.conversation.id}`;
    } catch (err) {
      setError(err.message);
    } finally {
      setSending(false);
    }
  }

  return (
    <PageShell active="home">
      <MainColumn>
        <div className="scroll-area">
          <div className="simple-container">
            <Link className="simple-back" href="/">
              ← Duvara dön
            </Link>
            <div className="simple-card" style={{ marginTop: 12 }}>
              {state === "loading" && <p>Paylaşım yükleniyor…</p>}
              {state === "missing" && (
                <p>Bu paylaşım bulunamadı veya kaldırılmış olabilir.</p>
              )}
              {state === "error" && (
                <p>Paylaşım yüklenemedi. Bağlantını kontrol edip tekrar dene.</p>
              )}
              {state === "ready" && post && (
                <>
                  <span className="detail-cat">{post.category}</span>
                  <h1 style={{ marginTop: 16 }}>{post.title}</h1>
                  <div className="detail-meta">
                    <span>{post.author}</span>
                    <time dateTime={post.created_at}>{formatDate(post.created_at)}</time>
                  </div>
                  <p className="detail-body">{post.body}</p>
                  <div className="detail-stats">
                    <button
                      type="button"
                      className={`support-btn${supported ? " is-on" : ""}`}
                      aria-pressed={supported}
                      onClick={toggleSupport}
                    >
                      <Users />
                      {post.supports} destek
                    </button>
                    <span>
                      <MessageCircle />
                      {post.chats} sohbet
                    </span>
                  </div>

                  {post.mine ? (
                    <p className="chat-self">
                      Bu senin paylaşımın. Sana yazan kişiler{" "}
                      <Link href="/gelen">gelen kutuna</Link> düşer.
                    </p>
                  ) : post.conversationId ? (
                    <Link className="btn btn-dark chat-open" href={`/gelen?sohbet=${post.conversationId}`}>
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
                      {error && (
                        <p className="auth-error" role="alert">
                          {error}
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
                </>
              )}
            </div>
          </div>
        </div>
      </MainColumn>
    </PageShell>
  );
}
