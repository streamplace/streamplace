import { CaptionPolicySettings } from "@streamplace/core";
import { ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { View } from "react-native";
import { useTheme } from "../../lib/theme/theme";
import { Checkbox } from "../ui/checkbox";
import { Switch } from "../ui/switch";
import { Text } from "../ui/text";

function SwitchRow({
  title,
  description,
  value,
  onValueChange,
  disabled,
  testID,
}: {
  title: string;
  description: string;
  value: boolean;
  onValueChange: (value: boolean) => void;
  disabled?: boolean;
  testID: string;
}) {
  const { theme } = useTheme();
  return (
    <View
      style={{
        flexDirection: "row",
        alignItems: "center",
        gap: theme.spacing[4],
        opacity: disabled ? 0.5 : 1,
      }}
    >
      <View style={{ flex: 1, gap: theme.spacing[1] }}>
        <Text>{title}</Text>
        <Text size="sm" muted>
          {description}
        </Text>
      </View>
      <Switch
        testID={testID}
        accessibilityLabel={title}
        value={value}
        disabled={disabled}
        onValueChange={onValueChange}
      />
    </View>
  );
}

/**
 * The streamer's caption policy (place.stream.metadata.captionPolicy):
 * automatic captions, encoder-supplied captions, node captions for
 * accessibility, and the spoken language. `renderLanguagePicker` draws
 * the host app's language picker for the spoken-language hint.
 */
export function CaptionPolicyFields({
  value,
  onChange,
  renderLanguagePicker,
}: {
  value: CaptionPolicySettings;
  onChange: (value: CaptionPolicySettings) => void;
  renderLanguagePicker?: (
    language: string | null,
    onLanguageChange: (language: string | null) => void,
  ) => ReactNode;
}) {
  const { t } = useTranslation();
  const { theme } = useTheme();
  return (
    <View testID="dashboard-captions" style={{ gap: theme.spacing[6] }}>
      <Text size="lg">{t("dashboard-captions")}</Text>
      <SwitchRow
        testID="dashboard-captions-auto"
        title={t("dashboard-captions-auto")}
        description={t("dashboard-captions-auto-description")}
        value={value.mode === "auto"}
        disabled={value.mode === "ingest"}
        onValueChange={(on) =>
          onChange({ ...value, mode: on ? "auto" : "off" })
        }
      />
      <View testID="dashboard-captions-ingest">
        <Checkbox
          checked={value.mode === "ingest"}
          onCheckedChange={(checked) =>
            onChange({ ...value, mode: checked ? "ingest" : "auto" })
          }
          label={t("dashboard-captions-ingest")}
          description={t("dashboard-captions-ingest-description")}
        />
      </View>
      <SwitchRow
        testID="dashboard-captions-allow-nodes"
        title={t("dashboard-captions-allow-nodes")}
        description={t("dashboard-captions-allow-nodes-description")}
        value={value.allowNodeCaptions}
        onValueChange={(allowNodeCaptions) =>
          onChange({ ...value, allowNodeCaptions })
        }
      />
      {renderLanguagePicker && (
        <View
          testID="dashboard-captions-language"
          style={{ gap: theme.spacing[2] }}
        >
          <Text>{t("dashboard-captions-language")}</Text>
          <Text size="sm" muted>
            {t("dashboard-captions-language-description")}
          </Text>
          {renderLanguagePicker(value.languages[0] ?? null, (language) => {
            // The picker sets the primary language; keep any other hints.
            const rest = value.languages.slice(1).filter((l) => l !== language);
            onChange({
              ...value,
              languages: language ? [language, ...rest] : rest,
            });
          })}
        </View>
      )}
    </View>
  );
}
