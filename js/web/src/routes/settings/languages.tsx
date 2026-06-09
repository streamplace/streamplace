import { BackLink } from "@/components/settings/back-link";
import { CardDivide } from "@/components/ui/card";
import { manifest } from "@streamplace/i18n";
import { createFileRoute } from "@tanstack/react-router";
import { Check } from "lucide-react";
import { useState } from "react";
import { useTranslation } from "react-i18next";

export const Route = createFileRoute("/settings/languages")({
  component: LanguagesSettings,
});

function LanguagesSettings() {
  const { t, i18n } = useTranslation("settings");
  const [searchQuery, setSearchQuery] = useState("");

  const filteredLanguages = Object.entries(manifest.languages).filter(
    ([code, info]) =>
      searchQuery === "" ||
      info.name.toLowerCase().includes(searchQuery.toLowerCase()) ||
      info.nativeName.toLowerCase().includes(searchQuery.toLowerCase()) ||
      code.toLowerCase().includes(searchQuery.toLowerCase()),
  );

  const currentLang =
    manifest.languages[i18n.language as keyof typeof manifest.languages];

  return (
    <div className="space-y-6">
      <BackLink to="/settings" label={t("settings-title")} />

      <div>
        <h1 className="text-xl font-semibold">{t("language-selection")}</h1>
        <p className="text-sm text-[var(--color-fg-muted)] mt-1">
          {t("language-selection-description")}
        </p>
      </div>

      <CardDivide>
        <div className="flex items-center gap-2 px-3 py-2.5">
          <span>{currentLang?.flag || "🌍"}</span>
          <span className="font-medium">
            {currentLang?.nativeName || i18n.language}
          </span>
        </div>
      </CardDivide>

      <input
        type="text"
        value={searchQuery}
        onChange={(e) => setSearchQuery(e.target.value)}
        placeholder={t("input-search-languages")}
        className="h-9 w-full rounded-lg border border-[var(--color-border)] bg-transparent px-3 text-sm outline-none focus:border-[var(--color-accent)]"
      />

      <CardDivide className="max-h-[60vh] overflow-y-auto">
        {filteredLanguages.map(([code, info]) => {
          const isSelected = i18n.language === code;
          return (
            <button
              key={code}
              type="button"
              onClick={() => {
                void i18n.changeLanguage(code);
                setSearchQuery("");
              }}
              className="flex items-center justify-between w-full px-3 py-2.5 hover:bg-[var(--color-bg)] transition-colors text-left"
            >
              <div className="flex items-center gap-2">
                <span>{info.flag}</span>
                <div>
                  <span
                    className="text-sm"
                    style={{ fontWeight: isSelected ? 600 : 400 }}
                  >
                    {info.nativeName}
                  </span>
                  {info.name !== info.nativeName && (
                    <span className="text-xs text-[var(--color-fg-muted)] ml-2">
                      {info.name}
                    </span>
                  )}
                </div>
              </div>
              {isSelected && (
                <Check size={16} className="text-[var(--color-accent)]" />
              )}
            </button>
          );
        })}
        {filteredLanguages.length === 0 && (
          <div className="px-3 py-4 text-center text-sm text-[var(--color-fg-muted)]">
            {t("no-languages-found")}
          </div>
        )}
      </CardDivide>
    </div>
  );
}
