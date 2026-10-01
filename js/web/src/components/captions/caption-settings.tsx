import { CaptionLines } from "@/components/player/caption-overlay";
import { Button } from "@/components/ui/button";
import { Switch } from "@/components/ui/switch";
import { useStore } from "@/lib/store";
import { cn } from "@/lib/utils";
import {
  CAPTION_COLORS,
  CAPTION_EDGES,
  CAPTION_FONTS,
  CAPTION_OPACITIES,
  CAPTION_SIZES,
  captionColor,
  type CaptionColor,
  type CaptionDisplayPrefs,
  DEFAULT_CAPTION_PREFS,
} from "@streamplace/core";
import { useId } from "react";
import { useTranslation } from "react-i18next";

const COLOR_NAMES = Object.keys(CAPTION_COLORS) as CaptionColor[];

// kebab-case i18n key suffixes for the camelCase pref values.
const kebab = (value: string) =>
  value.replace(/[A-Z]/g, (c) => `-${c.toLowerCase()}`);

interface Choice<T> {
  value: T;
  label: string;
  swatch?: string;
}

/** A labeled, wrapping radio group of chips for one caption setting. */
function ChoiceGroup<T extends string | number>({
  title,
  choices,
  value,
  onChange,
  testId,
}: {
  title: string;
  choices: Choice<T>[];
  value: T;
  onChange: (value: T) => void;
  testId: string;
}) {
  const labelId = useId();
  return (
    <div className="space-y-2">
      <div id={labelId} className="text-sm font-medium">
        {title}
      </div>
      <div
        role="radiogroup"
        aria-labelledby={labelId}
        data-testid={testId}
        className="flex flex-wrap gap-2"
      >
        {choices.map((choice) => {
          const selected = choice.value === value;
          return (
            <button
              key={String(choice.value)}
              type="button"
              role="radio"
              aria-checked={selected}
              data-testid={`${testId}-${choice.value}`}
              onClick={() => onChange(choice.value)}
              className={cn(
                "focus-visible:ring-ring flex items-center gap-2 rounded-full border px-3 py-1 text-sm transition-colors focus-visible:ring-2 focus-visible:ring-offset-2 focus-visible:outline-none",
                selected
                  ? "border-primary bg-primary text-primary-foreground"
                  : "border-(--color-border) hover:bg-(--color-bg-elevated)",
              )}
            >
              {choice.swatch && (
                <span
                  aria-hidden
                  className="size-4 rounded-full border border-(--color-border)"
                  style={{ backgroundColor: choice.swatch }}
                />
              )}
              {choice.label}
            </button>
          );
        })}
      </div>
    </div>
  );
}

/**
 * Viewer caption settings: on/off plus the display options of 47 CFR
 * 79.103(c), with a live preview. Used by Settings → Captions and the
 * player's caption menu.
 */
export function CaptionSettings() {
  const { t } = useTranslation("settings");
  const prefs = useStore((s) => s.captionPrefs);
  const setPrefs = useStore((s) => s.setCaptionPrefs);
  const enabled = useStore((s) => s.captionsEnabled);
  const setEnabled = useStore((s) => s.setCaptionsEnabled);
  const enabledLabelId = useId();

  const update = <K extends keyof CaptionDisplayPrefs>(
    key: K,
    value: CaptionDisplayPrefs[K],
  ) => setPrefs({ ...prefs, [key]: value });

  const colors: Choice<CaptionColor>[] = COLOR_NAMES.map((name) => ({
    value: name,
    label: t(`captions-color-${name}`),
    swatch: captionColor(name, 100),
  }));
  const opacities = CAPTION_OPACITIES.map((value) => ({
    value,
    label: `${value}%`,
  }));

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div className="pr-4">
          <div id={enabledLabelId} className="text-sm font-medium">
            {t("captions-enabled")}
          </div>
          <div className="mt-0.5 text-xs text-(--color-fg-muted)">
            {t("captions-enabled-description")}
          </div>
        </div>
        <Switch
          aria-labelledby={enabledLabelId}
          data-testid="settings-captions-enabled"
          checked={enabled}
          onCheckedChange={setEnabled}
        />
      </div>

      <div
        aria-hidden
        data-testid="settings-captions-preview"
        className="flex min-h-24 items-end justify-center rounded-lg border border-(--color-border) bg-(--color-bg-elevated) p-6"
      >
        <CaptionLines
          lines={[t("captions-preview")]}
          prefs={prefs}
          fontSize={`calc(var(--text-lg) * ${prefs.size / 100})`}
        />
      </div>

      <ChoiceGroup
        title={t("captions-text-size")}
        testId="settings-captions-size"
        choices={CAPTION_SIZES.map((value) => ({ value, label: `${value}%` }))}
        value={prefs.size}
        onChange={(v) => update("size", v)}
      />
      <ChoiceGroup
        title={t("captions-font")}
        testId="settings-captions-font"
        choices={CAPTION_FONTS.map((value) => ({
          value,
          label: t(`captions-font-${kebab(value)}`),
        }))}
        value={prefs.font}
        onChange={(v) => update("font", v)}
      />
      <ChoiceGroup
        title={t("captions-text-color")}
        testId="settings-captions-text-color"
        choices={colors}
        value={prefs.textColor}
        onChange={(v) => update("textColor", v)}
      />
      <ChoiceGroup
        title={t("captions-text-opacity")}
        testId="settings-captions-text-opacity"
        choices={opacities.filter((o) => o.value > 0)}
        value={prefs.textOpacity}
        onChange={(v) => update("textOpacity", v)}
      />
      <ChoiceGroup
        title={t("captions-edge")}
        testId="settings-captions-edge"
        choices={CAPTION_EDGES.map((value) => ({
          value,
          label: t(`captions-edge-${kebab(value)}`),
        }))}
        value={prefs.edge}
        onChange={(v) => update("edge", v)}
      />
      <ChoiceGroup
        title={t("captions-background-color")}
        testId="settings-captions-background-color"
        choices={colors}
        value={prefs.backgroundColor}
        onChange={(v) => update("backgroundColor", v)}
      />
      <ChoiceGroup
        title={t("captions-background-opacity")}
        testId="settings-captions-background-opacity"
        choices={opacities}
        value={prefs.backgroundOpacity}
        onChange={(v) => update("backgroundOpacity", v)}
      />
      <ChoiceGroup
        title={t("captions-window-color")}
        testId="settings-captions-window-color"
        choices={colors}
        value={prefs.windowColor}
        onChange={(v) => update("windowColor", v)}
      />
      <ChoiceGroup
        title={t("captions-window-opacity")}
        testId="settings-captions-window-opacity"
        choices={opacities}
        value={prefs.windowOpacity}
        onChange={(v) => update("windowOpacity", v)}
      />

      <Button
        variant="outline"
        data-testid="settings-captions-reset"
        onClick={() => setPrefs(DEFAULT_CAPTION_PREFS)}
      >
        {t("captions-reset")}
      </Button>
    </div>
  );
}
