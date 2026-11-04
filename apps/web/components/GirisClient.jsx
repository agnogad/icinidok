"use client";

import { useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { MobileHeader, PageShell } from "./Frame";
import {
  ArrowRight,
  CheckCheck,
  Loader,
  LogoMark,
  ShieldCheck,
} from "./Icons";
import { api } from "../lib/api";

export default function GirisClient() {
  const router = useRouter();
  const [mode, setMode] = useState("login");

  const [loginStep, setLoginStep] = useState("username");
  const [loginName, setLoginName] = useState("");
  const [loginCode, setLoginCode] = useState("");
  const [loginError, setLoginError] = useState("");
  const [loginBusy, setLoginBusy] = useState(false);

  const [signupStep, setSignupStep] = useState("username");
  const [signupName, setSignupName] = useState("");
  const [signup, setSignup] = useState(null);
  const [signupCode, setSignupCode] = useState("");
  const [signupError, setSignupError] = useState("");
  const [signupBusy, setSignupBusy] = useState(false);
  const [recoveryCodes, setRecoveryCodes] = useState([]);
  const [copied, setCopied] = useState(false);

  function switchMode(next) {
    setMode(next);
    setLoginError("");
    setSignupError("");
  }

  function resetLogin() {
    setLoginStep("username");
    setLoginCode("");
    setLoginError("");
  }

  function resetSignup() {
    setSignupStep("username");
    setSignup(null);
    setSignupCode("");
    setSignupError("");
    setRecoveryCodes([]);
    setCopied(false);
  }

  function submitUsername(event) {
    event.preventDefault();
    if (!loginName.trim()) {
      setLoginError("Kullanıcı adını yaz.");
      return;
    }
    setLoginError("");
    setLoginStep("code");
  }

  async function submitLogin(event) {
    event.preventDefault();
    setLoginBusy(true);
    setLoginError("");
    try {
      await api("/api/auth/login", {
        method: "POST",
        body: { username: loginName.trim(), code: loginCode.trim() },
      });
      router.push("/");
      router.refresh();
    } catch (error) {
      setLoginError(error.message);
    } finally {
      setLoginBusy(false);
    }
  }

  async function startSignup(event) {
    event.preventDefault();
    setSignupBusy(true);
    setSignupError("");
    try {
      const data = await api("/api/auth/signup/start", {
        method: "POST",
        body: { username: signupName.trim() },
      });
      setSignup(data);
      setSignupStep("scan");
    } catch (error) {
      setSignupError(error.message);
    } finally {
      setSignupBusy(false);
    }
  }

  async function confirmSignup(event) {
    event.preventDefault();
    setSignupBusy(true);
    setSignupError("");
    try {
      const data = await api("/api/auth/signup/confirm", {
        method: "POST",
        body: { signup_id: signup.signup_id, code: signupCode.trim() },
      });
      setRecoveryCodes(data.recovery_codes || []);
      setSignupStep("done");
    } catch (error) {
      setSignupError(error.message);
    } finally {
      setSignupBusy(false);
    }
  }

  async function copyRecovery() {
    try {
      await navigator.clipboard.writeText(recoveryCodes.join("\n"));
      setCopied(true);
      setTimeout(() => setCopied(false), 2500);
    } catch {
      setCopied(false);
    }
  }

  return (
    <PageShell active="home">
      <main id="icerik" className="main">
        <MobileHeader />
        <div className="giris-wrap">
          <div className="giris-card auth-card">
            <LogoMark className="giris-logo" />

            <div className="auth-tabs" role="tablist" aria-label="Hesap işlemi">
              <button
                type="button"
                role="tab"
                aria-selected={mode === "login"}
                className={`auth-tab${mode === "login" ? " is-active" : ""}`}
                onClick={() => switchMode("login")}
              >
                Giriş
              </button>
              <button
                type="button"
                role="tab"
                aria-selected={mode === "signup"}
                className={`auth-tab${mode === "signup" ? " is-active" : ""}`}
                onClick={() => switchMode("signup")}
              >
                Kayıt ol
              </button>
            </div>

            {mode === "login" && loginStep === "username" && (
              <form className="auth-form" onSubmit={submitUsername}>
                <h1>Kullanıcı adınla giriş yap</h1>
                <p>
                  Şifre yok. Kullanıcı adını yaz, telefonundaki authenticator
                  uygulamasındaki 6 haneli kodu gir.
                </p>
                <label className="auth-field">
                  <span>Kullanıcı adı</span>
                  <input
                    name="username"
                    autoComplete="username"
                    autoCapitalize="none"
                    spellCheck={false}
                    maxLength={24}
                    placeholder="örn. gece_kirpi"
                    value={loginName}
                    onChange={(event) => setLoginName(event.target.value)}
                    autoFocus
                  />
                </label>
                {loginError && (
                  <p className="auth-error" role="alert">
                    {loginError}
                  </p>
                )}
                <button
                  type="submit"
                  className="btn btn-dark auth-submit"
                  disabled={loginBusy}
                >
                  Devam et
                  <ArrowRight />
                </button>
              </form>
            )}

            {mode === "login" && loginStep === "code" && (
              <form className="auth-form" onSubmit={submitLogin}>
                <h1>Doğrulama kodu</h1>
                <p>
                  <span className="auth-identity">@{loginName.trim()}</span>{" "}
                  hesabı için authenticator uygulamasındaki güncel kodu ya da
                  daha önce kaydettiğin bir kurtarma kodunu gir.
                </p>
                <label className="auth-field">
                  <span>6 haneli kod veya kurtarma kodu</span>
                  <input
                    name="code"
                    autoComplete="one-time-code"
                    inputMode="text"
                    maxLength={20}
                    placeholder="123456"
                    value={loginCode}
                    onChange={(event) => setLoginCode(event.target.value)}
                    autoFocus
                  />
                </label>
                {loginError && (
                  <p className="auth-error" role="alert">
                    {loginError}
                  </p>
                )}
                <button
                  type="submit"
                  className="btn btn-dark auth-submit"
                  disabled={loginBusy || !loginCode.trim()}
                >
                  {loginBusy ? <Loader className="auth-spin" /> : null}
                  {loginBusy ? "Kontrol ediliyor…" : "Giriş yap"}
                </button>
                <button type="button" className="auth-link" onClick={resetLogin}>
                  Kullanıcı adını değiştir
                </button>
              </form>
            )}

            {mode === "signup" && signupStep === "username" && (
              <form className="auth-form" onSubmit={startSignup}>
                <h1>Anonim hesap aç</h1>
                <p>
                  E-posta, telefon ve şifre istemiyoruz. Bir kullanıcı adı seç;
                  giriş yalnızca authenticator uygulamasındaki kodla yapılır.
                </p>
                <label className="auth-field">
                  <span>Kullanıcı adı</span>
                  <input
                    name="username"
                    autoComplete="username"
                    autoCapitalize="none"
                    spellCheck={false}
                    maxLength={24}
                    placeholder="örn. gece_kirpi"
                    value={signupName}
                    onChange={(event) => setSignupName(event.target.value)}
                    autoFocus
                  />
                </label>
                <p className="auth-hint">
                  3–24 karakter. Küçük harf, rakam ve alt çizgi. Bu ad duvarda
                  takma adın olarak görünür.
                </p>
                {signupError && (
                  <p className="auth-error" role="alert">
                    {signupError}
                  </p>
                )}
                <button
                  type="submit"
                  className="btn btn-dark auth-submit"
                  disabled={signupBusy || !signupName.trim()}
                >
                  {signupBusy ? <Loader className="auth-spin" /> : null}
                  {signupBusy ? "Hazırlanıyor…" : "QR kodu oluştur"}
                </button>
              </form>
            )}

            {mode === "signup" && signupStep === "scan" && signup && (
              <form className="auth-form" onSubmit={confirmSignup}>
                <h1>Authenticator&apos;a ekle</h1>
                <p>
                  Telefonundaki authenticator uygulamasıyla QR kodu tara.
                  Uygulama yoksa kodu elle de girebilirsin.
                </p>
                <img
                  className="auth-qr"
                  src={signup.qr}
                  width="220"
                  height="220"
                  alt="İki adımlı doğrulama için QR kodu"
                />
                <p className="auth-secret">
                  Elle eklemek için: <code>{signup.secret}</code>
                </p>
                <label className="auth-field">
                  <span>Uygulamadaki 6 haneli kod</span>
                  <input
                    name="code"
                    autoComplete="one-time-code"
                    inputMode="numeric"
                    maxLength={6}
                    placeholder="123456"
                    value={signupCode}
                    onChange={(event) => setSignupCode(event.target.value)}
                    autoFocus
                  />
                </label>
                <p className="auth-hint">
                  QR kodu 15 dakika geçerli. Kod doğrulanınca hesap açılır.
                </p>
                {signupError && (
                  <p className="auth-error" role="alert">
                    {signupError}
                  </p>
                )}
                <button
                  type="submit"
                  className="btn btn-dark auth-submit"
                  disabled={signupBusy || signupCode.trim().length !== 6}
                >
                  {signupBusy ? <Loader className="auth-spin" /> : null}
                  {signupBusy ? "Doğrulanıyor…" : "Kodu doğrula ve hesabı aç"}
                </button>
                <button type="button" className="auth-link" onClick={resetSignup}>
                  İptal et
                </button>
              </form>
            )}

            {mode === "signup" && signupStep === "done" && (
              <div className="auth-form">
                <h1>Hesabın hazır</h1>
                <p>
                  <span className="auth-identity">@{signup?.username}</span>{" "}
                  artık senin. Aşağıdaki kurtarma kodlarını güvenli bir yere
                  kaydet; authenticator cihazını kaybedersen giriş için tek
                  çaren bunlar.
                </p>
                <div className="auth-recovery">
                  {recoveryCodes.map((code) => (
                    <code key={code}>{code}</code>
                  ))}
                </div>
                <button
                  type="button"
                  className="btn btn-outline auth-submit"
                  onClick={copyRecovery}
                >
                  <CheckCheck />
                  {copied ? "Kopyalandı" : "Kurtarma kodlarını kopyala"}
                </button>
                <p className="auth-warning">
                  Bu kodlar bir kez gösterilir. Her kod yalnızca bir kez
                  kullanılabilir. E-posta ile sıfırlama yok.
                </p>
                <button
                  type="button"
                  className="btn btn-dark auth-submit"
                  onClick={() => router.push("/")}
                >
                  Duvara git
                  <ArrowRight />
                </button>
              </div>
            )}

            <p className="giris-note auth-note">
              <ShieldCheck />
              <span>
                E-posta ve şifre yok. Oturum çerezi yalnızca giriş için
                kullanılır; kullanıcı adın dışında kimlik bilgisi saklanmaz.
              </span>
            </p>
            <Link className="simple-back" href="/">
              Duvara dön
            </Link>
          </div>
        </div>
      </main>
    </PageShell>
  );
}
