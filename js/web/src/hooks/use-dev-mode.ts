import { useCallback, useEffect, useState } from "react";

const DEV_MODE_KEY = "streamplace:dev-mode";

export function useDevMode(): [boolean, () => void] {
  const [enabled, setEnabled] = useState(() => {
    try {
      return localStorage.getItem(DEV_MODE_KEY) === "true";
    } catch {
      return false;
    }
  });

  useEffect(() => {
    const handler = (e: StorageEvent) => {
      if (e.key === DEV_MODE_KEY) {
        setEnabled(e.newValue === "true");
      }
    };
    window.addEventListener("storage", handler);
    return () => window.removeEventListener("storage", handler);
  }, []);

  const toggle = useCallback(() => {
    setEnabled((prev) => {
      const next = !prev;
      try {
        localStorage.setItem(DEV_MODE_KEY, String(next));
      } catch {}
      return next;
    });
  }, []);

  return [enabled, toggle];
}

export function isDevMode(): boolean {
  try {
    return localStorage.getItem(DEV_MODE_KEY) === "true";
  } catch {
    return false;
  }
}
