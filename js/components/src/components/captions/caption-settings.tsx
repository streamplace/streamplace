import {
  CAPTION_COLORS,
  CAPTION_EDGES,
  CAPTION_FONTS,
  CAPTION_OPACITIES,
  CAPTION_SIZES,
  captionColor,
  CaptionColor,
  CaptionDisplayPrefs,
  DEFAULT_CAPTION_PREFS,
} from "@streamplace/core";
import { useTranslation } from "react-i18next";
import { Platform, Pressable, View } from "react-native";
import { useTheme } from "../../lib/theme/theme";
import { typeScale } from "../../lib/theme/tokens";
import {
  useCaptionPrefs,
  useCaptionsEnabled,
  useSetCaptionPrefs,
  useSetCaptionsEnabled,
} from "../../streamplace-store";
import { Button } from "../ui/button";
import { Switch } from "../ui/switch";
import { Text } from "../ui/text";
import { CaptionLines } from "./caption-overlay";

const COLOR_NAMES = Object.keys(CAPTION_COLORS) as CaptionColor[];

// kebab-case i18n key suffixes for the camelCase pref values.
const kebab = (value: string) =>
  value.replace(/[A-Z]/g, (c) => `-${c.toLowerCase()}`);

interface Choice<T> {
  value: T;
  label: string;
  swatch?: string;
}

/** A labeled, wrapping group of radio chips for one caption setting. */
function ChoiceGroup<T extends string | number>({
  title,
  choices,
  value,
  onChange,
  testID,
}: {
  title: string;
  choices: Choice<T>[];
  value: T;
  onChange: (value: T) => void;
  testID: string;
}) {
  const { theme } = useTheme();
  const c = theme.colors;
  return (
    <View
      accessibilityRole="radiogroup"
      accessibilityLabel={title}
      testID={testID}
      style={{ gap: theme.spacing[2] }}
    >
      <Text weight="semibold">{title}</Text>
      <View
        style={{
          flexDirection: "row",
          flexWrap: "wrap",
          gap: theme.spacing[2],
        }}
      >
        {choices.map((choice) => {
          const selected = choice.value === value;
          return (
            <Pressable
              key={String(choice.value)}
              testID={`${testID}-${choice.value}`}
              accessibilityRole="radio"
              accessibilityLabel={choice.label}
              aria-checked={selected}
              onPress={() => onChange(choice.value)}
              style={{
                flexDirection: "row",
                alignItems: "center",
                gap: theme.spacing[2],
                minHeight:
                  Platform.OS === "web"
                    ? undefined
                    : theme.touchTargets.minimum,
                paddingHorizontal: theme.spacing[3],
                paddingVertical: theme.spacing[1],
                borderRadius: theme.borderRadius.full,
                borderWidth: 1,
                borderColor: selected ? c.primary : c.border,
                backgroundColor: selected ? c.primary : c.surface2,
              }}
            >
              {choice.swatch && (
                <View
                  style={{
                    width: theme.spacing[4],
                    height: theme.spacing[4],
                    borderRadius: theme.borderRadius.full,
                    borderWidth: 1,
                    borderColor: c.borderStrong,
                    backgroundColor: choice.swatch,
                  }}
                />
              )}
              <Text style={{ color: selected ? c.primaryForeground : c.text1 }}>
                {choice.label}
              </Text>
            </Pressable>
          );
        })}
      </View>
    </View>
  );
}

/**
 * Viewer caption settings: on/off plus the display options of 47 CFR
 * 79.103(c), with a live preview. Used by the app's Captions settings
 * page and the player's caption menu.
 */
export function CaptionSettings() {
  const { t } = useTranslation("settings");
  const { theme } = useTheme();
  const prefs = useCaptionPrefs();
  const setPrefs = useSetCaptionPrefs();
  const enabled = useCaptionsEnabled();
  const setEnabled = useSetCaptionsEnabled();

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
    <View style={{ gap: theme.spacing[6] }}>
      <View
        style={{
          flexDirection: "row",
          alignItems: "center",
          justifyContent: "space-between",
          gap: theme.spacing[4],
        }}
      >
        <View style={{ flex: 1, gap: theme.spacing[1] }}>
          <Text weight="semibold">{t("captions-enabled")}</Text>
          <Text muted>{t("captions-enabled-description")}</Text>
        </View>
        <Switch
          testID="settings-captions-enabled"
          accessibilityLabel={t("captions-enabled")}
          value={enabled}
          onValueChange={setEnabled}
        />
      </View>
      {Platform.OS !== "web" && (
        <Text size="sm" muted>
          {t("captions-native-style-description")}
        </Text>
      )}

      <View
        testID="settings-captions-preview"
        accessibilityElementsHidden
        importantForAccessibility="no-hide-descendants"
        style={{
          alignItems: "center",
          justifyContent: "flex-end",
          padding: theme.spacing[6],
          borderRadius: theme.borderRadius.lg,
          borderWidth: 1,
          borderColor: theme.colors.border,
          backgroundColor: theme.colors.surface3,
        }}
      >
        <CaptionLines
          lines={[t("captions-preview")]}
          prefs={prefs}
          fontSize={Math.round((typeScale.lg.fontSize * prefs.size) / 100)}
        />
      </View>

      <ChoiceGroup
        title={t("captions-text-size")}
        testID="settings-captions-size"
        choices={CAPTION_SIZES.map((value) => ({ value, label: `${value}%` }))}
        value={prefs.size}
        onChange={(v) => update("size", v)}
      />
      <ChoiceGroup
        title={t("captions-font")}
        testID="settings-captions-font"
        choices={CAPTION_FONTS.map((value) => ({
          value,
          label: t(`captions-font-${kebab(value)}`),
        }))}
        value={prefs.font}
        onChange={(v) => update("font", v)}
      />
      <ChoiceGroup
        title={t("captions-text-color")}
        testID="settings-captions-text-color"
        choices={colors}
        value={prefs.textColor}
        onChange={(v) => update("textColor", v)}
      />
      <ChoiceGroup
        title={t("captions-text-opacity")}
        testID="settings-captions-text-opacity"
        choices={opacities.filter((o) => o.value > 0)}
        value={prefs.textOpacity}
        onChange={(v) => update("textOpacity", v)}
      />
      <ChoiceGroup
        title={t("captions-edge")}
        testID="settings-captions-edge"
        choices={CAPTION_EDGES.map((value) => ({
          value,
          label: t(`captions-edge-${kebab(value)}`),
        }))}
        value={prefs.edge}
        onChange={(v) => update("edge", v)}
      />
      <ChoiceGroup
        title={t("captions-background-color")}
        testID="settings-captions-background-color"
        choices={colors}
        value={prefs.backgroundColor}
        onChange={(v) => update("backgroundColor", v)}
      />
      <ChoiceGroup
        title={t("captions-background-opacity")}
        testID="settings-captions-background-opacity"
        choices={opacities}
        value={prefs.backgroundOpacity}
        onChange={(v) => update("backgroundOpacity", v)}
      />
      <ChoiceGroup
        title={t("captions-window-color")}
        testID="settings-captions-window-color"
        choices={colors}
        value={prefs.windowColor}
        onChange={(v) => update("windowColor", v)}
      />
      <ChoiceGroup
        title={t("captions-window-opacity")}
        testID="settings-captions-window-opacity"
        choices={opacities}
        value={prefs.windowOpacity}
        onChange={(v) => update("windowOpacity", v)}
      />

      <Button
        variant="secondary"
        width="min"
        testID="settings-captions-reset"
        onPress={() => setPrefs(DEFAULT_CAPTION_PREFS)}
      >
        {t("captions-reset")}
      </Button>
    </View>
  );
}
