"use client";

import { useEffect, useState } from "react";
import { Moon, Sun } from "./Icons";

export default function ThemeToggle() {
  const [dark, setDark] = useState(false);

  useEffect(() => {
    setDark(document.documentElement.dataset.theme === "dark");
  }, []);

  function toggle() {
    const next = !dark;
    setDark(next);
    if (next) {
      document.documentElement.dataset.theme = "dark";
    } else {
      delete document.documentElement.dataset.theme;
    }
    try {
      localStorage.setItem("icini-theme", next ? "dark" : "light");
    } catch {}
  }

  return (
    <button
      type="button"
      className="theme-btn"
      aria-label={dark ? "Açık temaya geç" : "Koyu temaya geç"}
      title={dark ? "Açık temaya geç" : "Koyu temaya geç"}
      aria-pressed={dark}
      onClick={toggle}
    >
      {dark ? <Sun className="icon-20" /> : <Moon className="icon-20" />}
      <span className="nav-tooltip">{dark ? "Açık tema" : "Koyu tema"}</span>
    </button>
  );
}
