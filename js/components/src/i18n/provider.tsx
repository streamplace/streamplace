// Simple i18n provider wrapper for consistent setup
import type { i18n } from "i18next";
import { ReactNode, useEffect, useState } from "react";
import { I18nextProvider } from "react-i18next";

interface I18nProviderProps {
  children: ReactNode;
  i18n: i18n;
}

/**
 * Simple wrapper around I18nextProvider for consistent setup
 * The actual i18n instance should be configured at the app level
 * Waits for i18next to be initialized AND language loaded to prevent FOUC
 */
export function I18nProvider({ children, i18n }: I18nProviderProps) {
  const [isReady, setIsReady] = useState(false);

  useEffect(() => {
    // Check if both initialized and language is loaded
    const checkReady = () => {
      const initialized = i18n.isInitialized;
      const hasCommon = i18n.hasLoadedNamespace("common");
      console.log("i18n ready check:", {
        initialized,
        hasCommon,
        language: i18n.language,
      });

      if (initialized && hasCommon) {
        setIsReady(true);
      }
    };

    checkReady();

    // Fallback: if not ready after 300ms, just proceed anyway
    const fallbackTimeout = setTimeout(() => {
      if (!isReady) {
        console.log("i18n ready timeout - proceeding anyway");
        setIsReady(true);
      }
    }, 300);

    // Listen for both initialization and language loaded events
    const handleInitialized = () => {
      checkReady();
    };
    const handleLanguageLoaded = () => {
      checkReady();
    };

    i18n.on("initialized", handleInitialized);
    i18n.on("languageChanged", handleLanguageLoaded);
    i18n.on("loaded", handleLanguageLoaded);

    return () => {
      clearTimeout(fallbackTimeout);
      i18n.off("initialized", handleInitialized);
      i18n.off("languageChanged", handleLanguageLoaded);
      i18n.off("loaded", handleLanguageLoaded);
    };
  }, [i18n, isReady]);

  if (!isReady) {
    return null;
  }

  return <I18nextProvider i18n={i18n}>{children}</I18nextProvider>;
}
