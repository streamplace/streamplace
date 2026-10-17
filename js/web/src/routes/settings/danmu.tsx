import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Slider } from "@/components/ui/slider";
import { Switch } from "@/components/ui/switch";
import { createFileRoute } from "@tanstack/react-router";
import { useTranslation } from "react-i18next";
import { useStore } from "../../lib/store";

// Danmu settings: enable/disable, opacity, speed, lane count, max messages.
// Ported from js/app/components/settings/danmu-category-settings.tsx.
export const Route = createFileRoute("/settings/danmu")({
  component: DanmuSettings,
});

function DanmuSettings() {
  const { t } = useTranslation("settings");
  const danmuEnabled = useStore((s) => s.danmuEnabled);
  const danmuOpacity = useStore((s) => s.danmuOpacity);
  const danmuSpeed = useStore((s) => s.danmuSpeed);
  const danmuLaneCount = useStore((s) => s.danmuLaneCount);
  const danmuMaxMessages = useStore((s) => s.danmuMaxMessages);
  const setDanmuEnabled = useStore((s) => s.setDanmuEnabled);
  const setDanmuOpacity = useStore((s) => s.setDanmuOpacity);
  const setDanmuSpeed = useStore((s) => s.setDanmuSpeed);
  const setDanmuLaneCount = useStore((s) => s.setDanmuLaneCount);
  const setDanmuMaxMessages = useStore((s) => s.setDanmuMaxMessages);

  return (
    <div className="space-y-6">
      <div>
        <h1 className="font-display text-xl font-semibold">{t("danmu")}</h1>
        <p className="mt-1 text-sm text-(--color-fg-muted)">
          {t("danmu-enabled-description")}
        </p>
      </div>

      <Card className="space-y-6 p-4">
        {/* Enable/Disable Danmu */}
        <div className="flex items-center justify-between">
          <div className="pr-4">
            <div className="text-sm font-medium">{t("danmu-enabled")}</div>
            <div className="mt-0.5 text-xs text-(--color-fg-muted)">
              {t("danmu-enabled-description")}
            </div>
          </div>
          <Switch checked={danmuEnabled} onCheckedChange={setDanmuEnabled} />
        </div>

        <NumberSetting
          label={t("danmu-opacity")}
          unit="%"
          value={danmuOpacity}
          onChange={setDanmuOpacity}
          min={0}
          max={100}
          presets={[0, 25, 50, 75, 100]}
          step={5}
        />

        <NumberSetting
          label={t("danmu-speed")}
          unit="×"
          value={danmuSpeed}
          onChange={setDanmuSpeed}
          min={0.1}
          max={3}
          presets={[0.5, 1, 1.5, 2]}
          step={0.1}
        />

        <NumberSetting
          label={t("danmu-lane-count")}
          value={danmuLaneCount}
          onChange={setDanmuLaneCount}
          min={4}
          max={20}
          presets={[6, 8, 10, 12, 15]}
          step={1}
        />

        <NumberSetting
          label={t("danmu-max-messages")}
          value={danmuMaxMessages}
          onChange={setDanmuMaxMessages}
          min={5}
          max={200}
          presets={[10, 25, 50, 100]}
          step={5}
        />
      </Card>
    </div>
  );
}

function NumberSetting({
  label,
  unit,
  value,
  onChange,
  min,
  max,
  presets,
  step,
}: {
  label: string;
  unit?: string;
  value: number;
  onChange: (v: number) => void;
  min: number;
  max: number;
  presets: number[];
  step: number;
}) {
  return (
    <div className="space-y-2">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <div className="flex items-center gap-2">
          <span className="text-sm font-medium">
            {label}: {value}
            {unit}
          </span>
        </div>
        <div className="flex flex-wrap gap-1.5">
          {presets.map((preset) => (
            <Button
              key={preset}
              type="button"
              onClick={() => onChange(preset)}
              variant={value === preset ? "default" : "secondary"}
              size="sm"
            >
              {preset}
              {unit}
            </Button>
          ))}
        </div>
      </div>
      <Slider
        value={[value]}
        min={min}
        max={max}
        step={step}
        onValueChange={(vals) => onChange(vals[0])}
        aria-label={label}
      />
    </div>
  );
}
