"use client";

import { useCallback, useEffect, useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { api } from "../lib/api";
import useSession from "../lib/useSession";
import { CheckCheck, Loader, ShieldCheck, Users } from "./Icons";

function formatDate(value) {
  if (!value) return "";
  try {
    return new Intl.DateTimeFormat("tr-TR", {
      day: "numeric",
      month: "long",
      year: "numeric",
    }).format(new Date(value));
  } catch {
    return "";
  }
}

export default function ProfileCard() {
  const router = useRouter();
  const { loading, user } = useSession();
  const [tab, setTab] = useState("posts");
  const [stats, setStats] = useState(null);
  const [codesLeft, setCodesLeft] = useState(null);
  const [myPosts, setMyPosts] = useState([]);
  const [savedPosts, setSavedPosts] = useState([]);
  const [dataLoading, setDataLoading] = useState(false);
  const [busy, setBusy] = useState(false);

  const [rotateOpen, setRotateOpen] = useState(false);
  const [rotateCode, setRotateCode] = useState("");
  const [rotateError, setRotateError] = useState("");
  const [rotating, setRotating] = useState(false);
  const [newCodes, setNewCodes] = useState([]);
  const [copied, setCopied] = useState(false);

  const loadData = useCallback(async () => {
    setDataLoading(true);
    try {
      const [statsData, postsData, savedData] = await Promise.all([
        api("/api/me/stats"),
        api("/api/me/posts"),
        api("/api/me/saved"),
      ]);
      setStats(statsData.stats);
      setCodesLeft(statsData.recovery_codes_left);
      setMyPosts(postsData.posts || []);
      setSavedPosts(savedData.posts || []);
    } catch {
      // Leave the last known values in place.
    } finally {
      setDataLoading(false);
    }
  }, []);

  useEffect(() => {
    if (user) loadData();
  }, [user, loadData]);

  async function logout() {
    setBusy(true);
    try {
      await api("/api/auth/logout", { method: "POST" });
    } catch {
      // Cookie is cleared server-side regardless.
    }
    router.push("/");
    router.refresh();
  }

  async function rotate(event) {
    event.preventDefault();
    setRotating(true);
    setRotateError("");
    try {
      const data = await api("/api/me/recovery-codes", {
        method: "POST",
        body: { code: rotateCode.trim() },
      });
      setNewCodes(data.recovery_codes || []);
      setRotateCode("");
    } catch (error) {
      setRotateError(error.message);
    } finally {
      setRotating(false);
    }
  }

  async function copyCodes() {
    try {
      await navigator.clipboard.writeText(newCodes.join("\n"));
      setCopied(true);
      setTimeout(() => setCopied(false), 2500);
    } catch {
      setCopied(false);
    }
  }

  function closeRotate() {
    setRotateOpen(false);
    setNewCodes([]);
    setRotateError("");
    setRotateCode("");
    loadData();
  }

  if (loading) {
    return (
      <div className="simple-card profile-card">
        <p className="profile-loading">
          <Loader className="auth-spin" /> Profil yükleniyor…
        </p>
      </div>
    );
  }

  if (!user) {
    return (
      <div className="simple-card profile-card">
        <h1>Profilim</h1>
        <p>
          Henüz giriş yapmadın. Kullanıcı adın ve authenticator kodunla giriş
          yaptığında paylaşımların, sohbetlerin ve kaydettiklerin burada
          görünür.
        </p>
        <Link className="btn btn-dark" href="/giris">
          Kullanıcı adıyla giriş yap
        </Link>
        <Link className="simple-back" href="/">
          Duvara dön
        </Link>
      </div>
    );
  }

  const numbers = stats || { posts: 0, supports_on_posts: 0, given_support: 0, chats: 0, unread_messages: 0, saved: 0 };
  const shown = tab === "posts" ? myPosts : savedPosts;

  return (
    <div className="simple-card profile-card">
      <span className="profile-avatar" aria-hidden="true">
        {user.username.slice(0, 1).toUpperCase()}
      </span>
      <h1>@{user.username}</h1>
      <p className="profile-meta">
        Anonim hesap · {formatDate(user.created_at)} tarihinde açıldı
      </p>

      <dl className="profile-stats">
        <div>
          <dt>Paylaşım</dt>
          <dd>{numbers.posts}</dd>
        </div>
        <div>
          <dt>Aldığı destek</dt>
          <dd>{numbers.supports_on_posts}</dd>
        </div>
        <div>
          <dt>Verilen destek</dt>
          <dd>{numbers.given_support}</dd>
        </div>
        <div>
          <dt>Açık sohbet</dt>
          <dd>
            {numbers.chats}
            {numbers.unread_messages > 0 && <span className="profile-pill">{numbers.unread_messages} yeni</span>}
          </dd>
        </div>
        <div>
          <dt>Kaydedilen</dt>
          <dd>{numbers.saved}</dd>
        </div>
        <div>
          <dt>Kurtarma kodu</dt>
          <dd>{codesLeft ?? "—"}</dd>
        </div>
      </dl>

      <div className="profile-actions">
        <Link className="btn btn-dark" href="/paylas">
          Bir şey paylaş
        </Link>
        <Link className="btn btn-outline" href="/gelen">
          Gelen kutusu
          {numbers.unread_messages > 0 && <span className="profile-pill">{numbers.unread_messages}</span>}
        </Link>
        <button type="button" className="btn btn-outline" onClick={() => setRotateOpen(true)}>
          Kurtarma kodlarını yenile
        </button>
        <button type="button" className="btn btn-outline" onClick={logout} disabled={busy}>
          {busy ? "Çıkılıyor…" : "Çıkış yap"}
        </button>
      </div>

      <p className="profile-note">
        <ShieldCheck />
        <span>
          E-posta, telefon ve şifre saklanmaz. Hesabına yalnızca authenticator
          kodu veya kurtarma kodlarıyla girilir.
        </span>
      </p>

      {rotateOpen && (
        <div className="profile-rotate">
          <h2>Kurtarma kodlarını yenile</h2>
          {newCodes.length === 0 ? (
            <form className="auth-form" onSubmit={rotate}>
              <p>
                Onaylamak için authenticator uygulamasındaki güncel kodu ya da
                elindeki bir kurtarma kodunu gir. Mevcut kodların tümü geçersiz
                olur.
              </p>
              <label className="auth-field">
                <span>6 haneli kod veya kurtarma kodu</span>
                <input
                  maxLength={20}
                  placeholder="123456"
                  value={rotateCode}
                  onChange={(event) => setRotateCode(event.target.value)}
                  autoFocus
                />
              </label>
              {rotateError && (
                <p className="auth-error" role="alert">
                  {rotateError}
                </p>
              )}
              <div className="profile-rotate-actions">
                <button type="submit" className="btn btn-dark" disabled={rotating || !rotateCode.trim()}>
                  {rotating ? "Doğrulanıyor…" : "Yeni kodlar üret"}
                </button>
                <button type="button" className="btn btn-outline" onClick={closeRotate}>
                  Vazgeç
                </button>
              </div>
            </form>
          ) : (
            <div className="auth-form">
              <p>Yeni kodların aşağıda. Eski kodlar artık geçersiz.</p>
              <div className="auth-recovery">
                {newCodes.map((code) => (
                  <code key={code}>{code}</code>
                ))}
              </div>
              <div className="profile-rotate-actions">
                <button type="button" className="btn btn-outline" onClick={copyCodes}>
                  <CheckCheck />
                  {copied ? "Kopyalandı" : "Kopyala"}
                </button>
                <button type="button" className="btn btn-dark" onClick={closeRotate}>
                  Kaydettim
                </button>
              </div>
            </div>
          )}
        </div>
      )}

      <div className="profile-tabs" role="tablist" aria-label="Profil listeleri">
        <button
          type="button"
          role="tab"
          aria-selected={tab === "posts"}
          className={`profile-tab${tab === "posts" ? " is-active" : ""}`}
          onClick={() => setTab("posts")}
        >
          Paylaşımlarım ({myPosts.length})
        </button>
        <button
          type="button"
          role="tab"
          aria-selected={tab === "saved"}
          className={`profile-tab${tab === "saved" ? " is-active" : ""}`}
          onClick={() => setTab("saved")}
        >
          Kaydettiklerim ({savedPosts.length})
        </button>
      </div>

      {dataLoading && shown.length === 0 ? (
        <p className="profile-empty">Yükleniyor…</p>
      ) : shown.length === 0 ? (
        <p className="profile-empty">
          {tab === "posts"
            ? "Henüz paylaşımın yok."
            : "Henüz bir şey kaydetmedin. Duvarda yer imine dokun."}
        </p>
      ) : (
        <ul className="profile-list">
          {shown.map((post) => (
            <li key={post.id}>
              <Link href={`/dert?id=${post.id}`} className="profile-post">
                <span className="profile-post-cat">{post.category}</span>
                <span className="profile-post-title">{post.title}</span>
                <span className="profile-post-meta">
                  <Users />
                  {post.supports}
                  <span aria-hidden="true">·</span>
                  {formatDate(post.created_at)}
                </span>
              </Link>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
