const AUTH_RETURN_KEY = "streamplace:auth-return-path";

type AuthReturnStorage = Pick<Storage, "getItem" | "setItem" | "removeItem">;

function getAuthReturnStorage(): AuthReturnStorage | null {
  try {
    return typeof sessionStorage === "undefined" ? null : sessionStorage;
  } catch {
    return null;
  }
}

export function sanitizeAuthReturnPath(path: string): string | null {
  if (!path.startsWith("/") || path.startsWith("//")) return null;
  if (
    path === "/login" ||
    path.startsWith("/login?") ||
    path.startsWith("/login#")
  ) {
    return null;
  }
  return path;
}

export function saveAuthReturnPath(
  path: string,
  storage: AuthReturnStorage | null = getAuthReturnStorage(),
): void {
  const safePath = sanitizeAuthReturnPath(path);
  if (!safePath || !storage) return;
  try {
    storage.setItem(AUTH_RETURN_KEY, safePath);
  } catch {
    // Authentication can continue even when storage is unavailable.
  }
}

export function consumeAuthReturnPath(
  storage: AuthReturnStorage | null = getAuthReturnStorage(),
): string | null {
  if (!storage) return null;
  try {
    const path = storage.getItem(AUTH_RETURN_KEY);
    storage.removeItem(AUTH_RETURN_KEY);
    return path ? sanitizeAuthReturnPath(path) : null;
  } catch {
    return null;
  }
}
