import type { CaptionPolicySettings } from "@streamplace/core";
import { useTranslation } from "react-i18next";
import { Switch } from "../ui/switch";

export function CaptionPolicyFields({
  value,
  onChange,
}: {
  value: CaptionPolicySettings;
  onChange: (value: CaptionPolicySettings) => void;
}) {
  const { t } = useTranslation();
  return (
    <section
      data-testid="dashboard-captions"
      className="border-border space-y-4 rounded-lg border p-4"
    >
      <label className="flex items-center justify-between gap-4">
        <span>
          <span className="block font-medium">{t("dashboard-captions")}</span>
          <span className="text-muted-foreground text-sm">
            {t("dashboard-captions-auto-description")}
          </span>
        </span>
        <Switch
          data-testid="dashboard-captions-auto"
          aria-label={t("dashboard-captions")}
          checked={value.mode !== "off"}
          onCheckedChange={(enabled) =>
            onChange({ ...value, mode: enabled ? "auto" : "off" })
          }
        />
      </label>
      <details>
        <summary className="cursor-pointer text-sm">
          {t("dashboard-captions-advanced")}
        </summary>
        <div className="mt-4 space-y-4">
          <label className="flex items-center justify-between gap-4">
            <span>
              {t("dashboard-captions-ingest")}
              <span className="text-muted-foreground block text-sm">
                {t("dashboard-captions-ingest-description")}
              </span>
            </span>
            <Switch
              data-testid="dashboard-captions-ingest"
              checked={value.mode === "ingest"}
              onCheckedChange={(ingest) =>
                onChange({ ...value, mode: ingest ? "ingest" : "auto" })
              }
            />
          </label>
          <label className="flex items-center justify-between gap-4">
            <span>
              {t("dashboard-captions-allow-nodes")}
              <span className="text-muted-foreground block text-sm">
                {t("dashboard-captions-allow-nodes-description")}
              </span>
            </span>
            <Switch
              data-testid="dashboard-captions-allow-nodes"
              checked={value.allowNodeCaptions}
              onCheckedChange={(allowNodeCaptions) =>
                onChange({ ...value, allowNodeCaptions })
              }
            />
          </label>
          <label className="grid gap-2 text-sm">
            {t("dashboard-captions-language")}
            <input
              className="focus-visible:ring-ring border-border bg-background rounded-md border px-3 py-2 focus-visible:ring-2"
              data-testid="dashboard-captions-language"
              value={value.languages.join(", ")}
              onChange={(event) =>
                onChange({
                  ...value,
                  languages: event.target.value
                    .split(",")
                    .map((language) => language.trim())
                    .filter(Boolean)
                    .slice(0, 4),
                })
              }
            />
          </label>
        </div>
      </details>
    </section>
  );
}
