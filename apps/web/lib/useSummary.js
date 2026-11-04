"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { api } from "./api";

/**
 * Polls /api/summary for unread counts. `interval` is in milliseconds; pass 0
 * to disable polling (e.g. on public pages).
 */
export default function useSummary(interval = 20000) {
  const [summary, setSummary] = useState({ unread_messages: 0, unread_notifications: 0 });
  const [signedIn, setSignedIn] = useState(false);
  const timer = useRef(null);

  const refresh = useCallback(async () => {
    try {
      const data = await api("/api/summary");
      setSummary({
        unread_messages: data.unread_messages || 0,
        unread_notifications: data.unread_notifications || 0,
      });
      setSignedIn(true);
    } catch {
      setSignedIn(false);
    }
  }, []);

  useEffect(() => {
    refresh();
    if (!interval) return undefined;
    timer.current = setInterval(refresh, interval);
    return () => clearInterval(timer.current);
  }, [refresh, interval]);

  return { ...summary, signedIn, refresh };
}
