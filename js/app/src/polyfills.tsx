import { initializeTimeSync } from "@streamplace/components";

// Install the Date monkeypatch before React mounts so that all subsequent
// `new Date()` / `Date.now()` calls account for server clock offset.
// No-op on non-web platforms.
initializeTimeSync();
