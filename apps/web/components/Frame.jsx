"use client";

import Link from "next/link";
import {
  ArrowUpRight,
  Bell,
  Help,
  House,
  LogoMark,
  Messages,
  Pen,
  UserRound,
} from "./Icons";
import ThemeToggle from "./ThemeToggle";
import useSummary from "../lib/useSummary";
import useSession from "../lib/useSession";

const navItems = [
  { key: "home", href: "/", label: "Duvar", icon: House },
  { key: "paylas", href: "/paylas", label: "Paylaş", icon: Pen },
  { key: "gelen", href: "/gelen", label: "Gelen kutusu", icon: Messages },
  { key: "bildirimler", href: "/bildirimler", label: "Bildirimler", icon: Bell },
];

function badgeFor(key, summary) {
  if (key === "gelen") return summary.unread_messages;
  if (key === "bildirimler") return summary.unread_notifications;
  return 0;
}

export function SideNav({ active }) {
  const summary = useSummary();
  return (
    <nav aria-label="Ana gezinme" className="side-nav">
      <Link className="side-logo" href="/" aria-label="içini dök. — duvara dön">
        <LogoMark className="logo-40" />
        <span className="nav-tooltip">içini dök.</span>
      </Link>
      <a
        href="https://beyler.studio/"
        target="_blank"
        rel="noopener noreferrer"
        aria-label="Beyler Studio — yeni sekmede açılır"
        className="studio-pill"
      >
        <span>Beyler</span>
        <span>
          Studio <ArrowUpRight className="icon-12" />
        </span>
      </a>

      <ul className="nav-list">
        {navItems.map(({ key, href, label, icon: Icon }) => {
          const badge = badgeFor(key, summary);
          return (
            <li key={key}>
              <Link
                href={href}
                aria-label={badge ? `${label} (${badge} okunmamış)` : label}
                aria-current={active === key ? "page" : undefined}
                className={`icon-52${active === key ? " is-active" : ""}`}
              >
                <Icon />
                {badge > 0 && <span className="nav-badge">{badge > 99 ? "99+" : badge}</span>}
                <span className="nav-tooltip">{label}</span>
              </Link>
            </li>
          );
        })}
      </ul>

      <div className="side-bottom">
        <ThemeToggle />
        <Link
          href="/hakkinda"
          aria-label="Yardım ve güvenlik"
          aria-current={active === "hakkinda" ? "page" : undefined}
          className={`icon-52${active === "hakkinda" ? " is-active" : ""}`}
        >
          <Help />
          <span className="nav-tooltip">Yardım ve güvenlik</span>
        </Link>
        <Link
          href="/ben"
          aria-label="Profilim"
          aria-current={active === "ben" ? "page" : undefined}
          className="profile-link"
        >
          <span className="profile-dot" aria-hidden="true" />
          <span>Profilim</span>
          <span className="nav-tooltip">Profilim</span>
        </Link>
      </div>
    </nav>
  );
}

export function MobileHeader() {
  const summary = useSummary();
  return (
    <header className="mobile-header">
      <div className="mobile-header-inner">
        <Link className="mobile-logo" href="/" aria-label="içini dök. — duvar">
          <LogoMark className="logo-32" />
          <span className="wordmark">
            içini dök<span className="wordmark-dot">.</span>
          </span>
        </Link>
        <a
          href="https://beyler.studio/"
          target="_blank"
          rel="noopener noreferrer"
          aria-label="Beyler Studio — yeni sekmede açılır"
          className="mobile-studio"
        >
          <span>Beyler Studio</span>
          <ArrowUpRight className="icon-14" />
        </a>
        <div className="mobile-head-actions">
          {summary.signedIn && (
            <Link className="mobile-help" href="/bildirimler" aria-label="Bildirimler">
              <Bell />
              {summary.unread_notifications > 0 && (
                <span className="nav-badge">{summary.unread_notifications > 99 ? "99+" : summary.unread_notifications}</span>
              )}
            </Link>
          )}
          <Link className="mobile-help" href="/hakkinda" aria-label="Yardım ve güvenlik">
            <Help />
          </Link>
        </div>
      </div>
    </header>
  );
}

export function Banner() {
  const { loading, user } = useSession();
  // Hidden until the session is known, and while signed in: the banner is a
  // call to sign in, so it must not appear for someone already signed in.
  if (loading || user) return null;
  return (
    <div className="banner-wrap">
      <div className="banner">
        <span>
          Dertleri okuyabilirsin. Yazmak ve destek olmak için kullanıcı adınla
          giriş yap.
        </span>
        <Link href="/giris">Giriş yap</Link>
      </div>
    </div>
  );
}

const bottomItems = [
  { key: "home", href: "/", label: "Duvar", icon: House },
  { key: "paylas", href: "/paylas", label: "Paylaş", icon: Pen },
  { key: "gelen", href: "/gelen", label: "Gelen", icon: Messages },
  { key: "bildirimler", href: "/bildirimler", label: "Bildirimler", icon: Bell },
  { key: "ben", href: "/ben", label: "Profilim", icon: UserRound },
];

export function BottomNav({ active }) {
  const summary = useSummary();
  return (
    <nav aria-label="Ana gezinme" className="bottom-nav">
      <ul className="bottom-list">
        {bottomItems.map(({ key, href, label, icon: Icon }) => {
          const badge = badgeFor(key, summary);
          return (
            <li key={key}>
              <Link
                href={href}
                aria-current={active === key ? "page" : undefined}
                className={`bottom-link${active === key ? " is-active" : ""}`}
              >
                <span className="bottom-icon">
                  <Icon />
                  {badge > 0 && <span className="nav-badge">{badge > 99 ? "99+" : badge}</span>}
                </span>
                {label}
              </Link>
            </li>
          );
        })}
      </ul>
    </nav>
  );
}

export function MainColumn({ children }) {
  return (
    <main id="icerik" className="main">
      <MobileHeader />
      <Banner />
      {children}
    </main>
  );
}

export function PageShell({ active, aside, children }) {
  return (
    <>
      <a href="#icerik" className="skip-link">
        İçeriğe geç
      </a>
      <div className="shell">
        <SideNav active={active} />
        {aside}
        {children}
        <BottomNav active={active} />
      </div>
    </>
  );
}
