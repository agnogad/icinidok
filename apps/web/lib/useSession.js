"use client";

import { useCallback, useEffect, useState } from "react";
import { api } from "./api";

export default function useSession() {
  const [session, setSession] = useState({ loading: true, user: null });

  const refresh = useCallback(async () => {
    try {
      const data = await api("/api/auth/me");
      setSession({ loading: false, user: data.user });
    } catch {
      setSession({ loading: false, user: null });
    }
  }, []);

  useEffect(() => {
    refresh();
  }, [refresh]);

  return { ...session, refresh };
}
