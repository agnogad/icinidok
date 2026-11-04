"use client";

import { useEffect, useMemo, useState } from "react";
import { useRouter } from "next/navigation";
import {
  ArrowRight,
  Eye,
  Leaf,
  MessageCircle,
  ShieldCheck,
} from "./Icons";
import { api } from "../lib/api";
import useSession from "../lib/useSession";

const fallbackCategories = ["Genel"];

export default function PaylasForm() {
  const router = useRouter();
  const { user } = useSession();
  const [categories, setCategories] = useState(fallbackCategories);
  const [category, setCategory] = useState("Genel");
  const [title, setTitle] = useState("");
  const [body, setBody] = useState("");
  const [age, setAge] = useState(false);
  const [status, setStatus] = useState("");
  const [busy, setBusy] = useState(false);

  // Categories are the same server-owned list the wall uses.
  useEffect(() => {
    let alive = true;
    api("/api/meta")
      .then((data) => {
        if (alive && Array.isArray(data?.categories) && data.categories.length > 0) {
          setCategories(data.categories);
          setCategory((current) =>
            data.categories.includes(current) ? current : data.categories[0]
          );
        }
      })
      .catch(() => {});
    return () => {
      alive = false;
    };
  }, []);

  const titleCount = title.length;
  const bodyCount = body.length;
  // Rules mirror handleCreatePost exactly so the button and the server agree:
  // title 10-120 chars and >=2 words, body 80-4000 chars and >=12 words.
  const titleWords = title.trim() ? title.trim().split(/\s+/).length : 0;
  const bodyWords = body.trim() ? body.trim().split(/\s+/).length : 0;
  const canSubmit = useMemo(
    () =>
      title.trim().length >= 10 &&
      titleWords >= 2 &&
      body.trim().length >= 80 &&
      bodyWords >= 12 &&
      age,
    [title, titleWords, body, bodyWords, age]
  );

  // Tell the reader which rule is still unmet instead of leaving a dead button.
  const firstMissing = !age
    ? "Devam etmek için 18 yaş onayını işaretle."
    : title.trim().length < 10
      ? "Başlık en az 10 karakter olmalı."
      : titleWords < 2
        ? "Başlık en az iki kelime olmalı."
        : body.trim().length < 80
          ? "Detay en az 80 karakter olmalı."
          : bodyWords < 12
            ? `Detay en az 12 kelime olmalı (şu an ${bodyWords}).`
            : "";

  const previewTitle = title.trim()
    ? title
    : "Paylaşımının başlığı burada görünecek";
  const previewBody = body.trim()
    ? body
    : "Anlatmak istediklerini yazmaya başladığında kısa önizlemesi burada belirecek.";

  async function submit(event) {
    event.preventDefault();
    if (!canSubmit || busy) return;
    if (!user) {
      router.push("/giris");
      return;
    }
    setBusy(true);
    setStatus("");
    try {
      const data = await api("/api/posts", {
        method: "POST",
        body: { category, title: title.trim(), body: body.trim() },
      });
      window.location.href = `/dert/${data.post.id}`;
    } catch (error) {
      if (error.status === 401) {
        router.push("/giris");
        return;
      }
      setStatus(error.message);
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="paylas-container">
      <div className="paylas-head">
        <h1>
          <span className="only-mobile">İçini dök</span>
          <span className="only-desktop">Bir şey paylaş</span>
        </h1>
        <p>
          <span className="only-mobile">
            Takma adınla paylaşırsın; kullanıcı adın dışında kimlik bilgin
            duvarda görünmez.
          </span>
          <span className="only-desktop">
            İçini dök, bir şey anlat veya aradığını paylaş.
          </span>
        </p>
      </div>

      <div className="paylas-layout">
        <div className="paylas-editor">
          <div className="paylas-info-mobile">
            <p>
              <Eye strokeWidth={1.6} />
              <span>
                <strong>Başlık ve detayın kısa önizlemesi duvarda</strong>,{" "}
                <strong>detayın tamamı paylaşım sayfasında</strong> herkese açık
                görünür.
              </span>
            </p>
            <p>
              <ShieldCheck strokeWidth={1.6} />
              <span>
                İsim, telefon, adres, okul ya da iş yeri gibi seni veya başkasını
                tanıtabilecek bilgileri yazmamanı öneririz.
              </span>
            </p>
          </div>

          <div className="paylas-info-desktop">
            <Eye />
            <p>
              <strong>Paylaşımın herkese açık.</strong> Kullanıcı adın dışında
              kimlik bilgin gizli.
            </p>
          </div>

          <form className="paylas-form" onSubmit={submit}>
            <fieldset className="f-categories">
              <legend className="field-legend">Kategori</legend>
              <div className="paylas-cats">
                {categories.map((item) => (
                  <label
                    key={item}
                    className={category === item ? "is-checked" : ""}
                  >
                    <input
                      type="radio"
                      name="category"
                      value={item}
                      checked={category === item}
                      onChange={() => setCategory(item)}
                    />
                    {item}
                  </label>
                ))}
              </div>
            </fieldset>

            <label className="f-title">
              <span className="field-label">
                <span>
                  Başlık{" "}
                  <span className="only-mobile field-label-note">
                    · duvarda görünür
                  </span>
                </span>
                <span className="field-count">{titleCount}/120</span>
              </span>
              <input
                className="paylas-title-input"
                name="title"
                maxLength={120}
                placeholder="Örn. Kimseye anlatamadığım bir şey var"
                value={title}
                onChange={(event) => setTitle(event.target.value)}
              />
              <span className="field-hint">
                <span className="only-mobile">
                  10–120 karakter, en az iki farklı kelime. Selamlaşmak yerine
                  konuyu anlat: Örn. İş arıyorum veya Kendimi yalnız hissediyorum.
                </span>
                <span className="only-desktop">Konunu birkaç kelimeyle özetle.</span>
              </span>
            </label>

            <label className="f-body">
              <span className="field-label">
                <span>
                  <span className="only-mobile">Detay </span>
                  <span className="only-desktop">Anlatmak istediğin</span>
                  <span className="only-mobile field-label-note">
                    · paylaşım sayfasında görünür
                  </span>
                </span>
                <span className="field-count">{bodyCount}/4000</span>
              </span>
              <textarea
                className="paylas-body-input"
                name="body"
                rows={8}
                maxLength={4000}
                placeholder="Ne yaşadın, seni nasıl etkiledi? Birkaç cümleyle anlat…"
                value={body}
                onChange={(event) => setBody(event.target.value)}
              />
              <span className="field-hint">
                En az 80 karakter ve 12 kelimeyle anlat. Aynı kelimeleri tekrar
                etmek yeterli değil.
              </span>
            </label>

            <p className="paylas-privacy f-privacy">
              <ShieldCheck />
              Seni veya başkasını tanıtabilecek kişisel bilgileri paylaşma.
            </p>

            <label className="paylas-age f-age">
              <input
                type="checkbox"
                checked={age}
                onChange={(event) => setAge(event.target.checked)}
              />
              <span>
                18 yaşından büyüğüm ve buranın profesyonel destek hizmeti
                olmadığını biliyorum.
              </span>
            </label>

            <div className="paylas-captcha" aria-hidden="true" />

            <p className="paylas-spam f-spam">
              Spam&apos;e karşı: iki paylaşım arasında en az 10 dakika olmalı ve
              günde en fazla 4 paylaşım yapılabilir.
            </p>

            <div className="f-submit">
              <button
                type="submit"
                className="paylas-submit"
                disabled={!canSubmit || busy}
                aria-describedby="submit-hint"
              >
                {busy ? "Paylaşılıyor…" : "Paylaş"}
                <ArrowRight />
              </button>
              {!canSubmit && !busy && firstMissing && (
                <p id="submit-hint" className="field-hint f-submit-hint">
                  {firstMissing}
                </p>
              )}
            </div>
          </form>

          {status && (
            <p className="paylas-status" role="status" style={{ marginTop: 16 }}>
              {status}
            </p>
          )}
        </div>

        <aside className="paylas-preview" aria-label="Paylaşım önizlemesi">
          <h2>
            <Eye />
            Duvarda böyle görünecek
          </h2>
          <article className="preview-card">
            <div className="preview-head">
              <svg
                width="40"
                height="40"
                viewBox="0 0 44 44"
                aria-hidden="true"
                className="preview-avatar"
              >
                <circle cx="22" cy="22" r="22" fill="#5c7a6b" />
                <circle cx="22" cy="22" r="16.5" fill="#f6f7ef" opacity=".08" />
                <g transform="translate(10 10)">
                  <Leaf size={24} stroke="#f6f7ef" strokeWidth={1.6} />
                </g>
              </svg>
              <div className="preview-who">
                <p className="preview-name">Takma adın</p>
                <p className="preview-time">Şimdi</p>
              </div>
              <span className="preview-cat">{category}</span>
            </div>
            <h3 className="preview-title">{previewTitle}</h3>
            <p className="preview-excerpt">{previewBody}</p>
            <div className="preview-foot">
              <MessageCircle />
              Sohbet başlat
            </div>
          </article>
          <p className="preview-note">
            Yazdıkça önizleme güncellenir. Detayın tamamı paylaşım sayfasında
            herkese açık görünür.
          </p>
        </aside>
      </div>
    </div>
  );
}
