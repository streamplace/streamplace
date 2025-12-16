import {
  Input,
  MenuContainer,
  MenuGroup,
  MenuInfo,
  MenuItem,
  MenuLabel,
  MenuSeparator,
  Text,
  useTheme,
  View,
  zero,
} from "@streamplace/components";
import { Check, X } from "lucide-react-native";
import { useState } from "react";
import { useTranslation } from "react-i18next";
import { ScrollView } from "react-native";
import { SettingToggle } from "./components/setting-toggle";
import { SettingsRowItem } from "./components/settings-navigation-item";

interface S3Config {
  endpoint: string;
  bucket: string;
  accessKey: string;
  secretKey: string;
}

interface ValidationErrors {
  endpoint?: string;
  bucket?: string;
  accessKey?: string;
  secretKey?: string;
}

function validateS3Config(config: S3Config): ValidationErrors {
  const errors: ValidationErrors = {};

  if (
    config.endpoint &&
    !/^[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$/.test(config.endpoint)
  ) {
    errors.endpoint = "Must be a valid domain name";
  }

  if (config.bucket && !/^[a-z0-9][a-z0-9.-]*[a-z0-9]$/.test(config.bucket)) {
    errors.bucket =
      "Must contain only lowercase letters, numbers, dots, and hyphens";
  }

  return errors;
}

function parseS3Url(url: string): S3Config | null {
  try {
    // Format: s3+https://ACCESS_KEY_ID:SECRET_ACCESS_KEY@s3.example.com/bucket
    const match = url.match(/^s3\+https?:\/\/([^:]+):([^@]+)@([^\/]+)\/(.+)$/);
    if (!match) return null;

    const [, accessKey, secretKey, endpoint, bucket] = match;
    return {
      endpoint,
      bucket,
      accessKey,
      secretKey,
    };
  } catch {
    return null;
  }
}

function getRandomBlockElements(length: number): string {
  const blockEls = ["▚", "▞", "▛", "▜", "▟", "▙"];
  let result = "";
  for (let i = 0; i < length; i++) {
    result += blockEls[Math.floor(Math.random() * blockEls.length)];
  }
  return result;
}

function buildS3Url(config: S3Config, showPassword: boolean): string {
  const secretKey =
    showPassword || !config.secretKey
      ? config.secretKey
      : getRandomBlockElements(config.secretKey.length);
  return `s3+https://${config.accessKey}:${secretKey}@${config.endpoint}/${config.bucket}`;
}

export function BackupSettings() {
  const { t } = useTranslation("settings");
  const [enabled, setEnabled] = useState(false);
  const [showPassword, setShowPassword] = useState(false);
  const [fullUrl, setFullUrl] = useState("");
  const [config, setConfig] = useState<S3Config>({
    endpoint: "",
    bucket: "",
    accessKey: "",
    secretKey: "",
  });
  const [testing, setTesting] = useState(false);
  const [testResult, setTestResult] = useState<{
    success: boolean;
  } | null>(null);
  const [validationErrors, setValidationErrors] = useState<ValidationErrors>(
    {},
  );
  const [requestedSecondsPerSegment, setRequestedSecondsPerSegment] =
    useState("6");

  const theme = useTheme();

  const handleFullUrlChange = (url: string) => {
    setFullUrl(url);
    const parsed = parseS3Url(url);
    if (parsed) {
      setConfig(parsed);
    }
  };

  const handleConfigChange = (key: keyof S3Config, value: string) => {
    const newConfig = { ...config, [key]: value };
    setConfig(newConfig);
    setFullUrl(buildS3Url(newConfig, showPassword));
    setValidationErrors(validateS3Config(newConfig));
  };

  const handleTestConnection = async () => {
    setTesting(true);
    setTestResult(null);

    // TODO: implement actual test connection
    setTimeout(() => {
      setTesting(false);
      setTestResult({
        success: Math.random() > 0.5,
      });
    }, 1500);
  };

  return (
    <ScrollView>
      <View style={[zero.layout.flex.align.center, zero.px[2], zero.py[2]]}>
        <View style={{ maxWidth: 500, width: "100%" }}>
          <MenuContainer>
            <MenuGroup>
              <SettingToggle
                title={t("backup-enabled")}
                description={t("backup-enabled-description")}
                value={enabled}
                onValueChange={setEnabled}
              />
            </MenuGroup>

            {enabled && (
              <>
                <MenuLabel>{t("backup-connection-url")}</MenuLabel>
                <MenuGroup style={{ marginVertical: -4 }}>
                  <View style={{ flex: 1 }}>
                    <Input
                      variant="underlined"
                      value={fullUrl}
                      onChangeText={handleFullUrlChange}
                      placeholder={buildS3Url(
                        parseS3Url(
                          t("backup-connection-url-placeholder", {
                            context:
                              "Must be a valid S3 URL in the format s3+https://ACCESS_KEY:SECRET_KEY@region.endpoi.nt/bucket",
                          }),
                        ) ?? {
                          endpoint: "s3.us-east-1.example.com",
                          bucket: "my-bucket",
                          accessKey: "ACCESS_KEY",
                          secretKey: "SECRET_KEY",
                        },
                        showPassword,
                      )}
                      autoCapitalize="none"
                      autoCorrect={false}
                      containerStyle={{ width: "100%" }}
                    />
                  </View>
                  <MenuSeparator />
                  <SettingToggle
                    title={t("show-password-in-url")}
                    description={t("show-password-in-url-description")}
                    value={showPassword}
                    onValueChange={setShowPassword}
                  />
                </MenuGroup>

                <MenuGroup>
                  <MenuItem style={{ marginVertical: -4 }}>
                    <Text style={{ minWidth: 100 }}>
                      {t("backup-endpoint")}
                    </Text>
                    <View style={{ flex: 1, paddingVertical: -24 }}>
                      <Input
                        value={config.endpoint}
                        onChangeText={(value) =>
                          handleConfigChange("endpoint", value)
                        }
                        placeholder={t("backup-endpoint-placeholder")}
                        variant="underlined"
                        autoCapitalize="none"
                        autoCorrect={false}
                        containerStyle={{ width: "100%" }}
                        inputStyle={{ textAlign: "right" }}
                        error={validationErrors.endpoint}
                      />
                    </View>
                  </MenuItem>

                  <MenuSeparator />

                  <MenuItem style={{ marginVertical: -4 }}>
                    <Text style={{ minWidth: 100 }}>{t("backup-bucket")}</Text>
                    <View style={{ flex: 1 }}>
                      <Input
                        value={config.bucket}
                        onChangeText={(value) =>
                          handleConfigChange("bucket", value)
                        }
                        placeholder={t("backup-bucket-placeholder")}
                        variant="underlined"
                        autoCapitalize="none"
                        autoCorrect={false}
                        containerStyle={{ width: "100%" }}
                        inputStyle={{ textAlign: "right" }}
                        error={validationErrors.bucket}
                      />
                    </View>
                  </MenuItem>

                  <MenuSeparator />

                  <MenuItem style={{ marginVertical: -4 }}>
                    <Text style={{ minWidth: 100 }}>
                      {t("backup-access-key")}
                    </Text>
                    <View style={{ flex: 1 }}>
                      <Input
                        value={config.accessKey}
                        onChangeText={(value) =>
                          handleConfigChange("accessKey", value)
                        }
                        placeholder={t("backup-access-key-placeholder")}
                        variant="underlined"
                        autoCapitalize="none"
                        autoCorrect={false}
                        containerStyle={{ width: "100%" }}
                        inputStyle={{
                          textAlign: "right",
                        }}
                      />
                    </View>
                  </MenuItem>

                  <MenuSeparator />

                  <MenuItem style={{ marginVertical: -4 }}>
                    <Text style={{ minWidth: 100 }}>
                      {t("backup-secret-key")}
                    </Text>
                    <View style={{ flex: 1 }}>
                      <Input
                        value={config.secretKey}
                        onChangeText={(value) =>
                          handleConfigChange("secretKey", value)
                        }
                        placeholder={t("backup-secret-key-placeholder")}
                        variant="underlined"
                        secureTextEntry
                        autoCapitalize="none"
                        autoCorrect={false}
                        containerStyle={{ width: "100%" }}
                        inputStyle={{ textAlign: "right" }}
                      />
                    </View>
                  </MenuItem>
                </MenuGroup>

                <MenuGroup>
                  <SettingsRowItem
                    onPress={
                      !testing && fullUrl ? handleTestConnection : undefined
                    }
                  >
                    <View
                      style={[
                        zero.layout.flex.justify.between,
                        zero.layout.flex.alignCenter,
                        zero.layout.flex.row,
                        { flex: 1 },
                      ]}
                    >
                      <Text
                        style={{
                          opacity: !fullUrl || testing ? 0.5 : 1,
                        }}
                      >
                        {testing
                          ? t("backup-testing-connection")
                          : t("backup-test-connection")}
                      </Text>
                      {testResult !== null &&
                        (testResult.success ? (
                          <Check color={theme.theme.colors.success} />
                        ) : (
                          <X color={theme.theme.colors.destructive} />
                        ))}
                    </View>
                  </SettingsRowItem>
                </MenuGroup>
                <MenuGroup>
                  <MenuItem style={{ marginVertical: -4 }}>
                    <Text style={{ minWidth: 100 }}>
                      {t("requested-seconds-per-segment")}
                    </Text>
                    <View style={{ flex: 1, paddingVertical: -24 }}>
                      <Input
                        value={requestedSecondsPerSegment}
                        onChangeText={setRequestedSecondsPerSegment}
                        placeholder="6"
                        variant="underlined"
                        keyboardType="numeric"
                        containerStyle={{ width: "100%" }}
                        inputStyle={{ textAlign: "right" }}
                      />
                    </View>
                  </MenuItem>
                </MenuGroup>
                <MenuInfo
                  description={t("requested-seconds-per-segment-description")}
                />
              </>
            )}
          </MenuContainer>
        </View>
      </View>
    </ScrollView>
  );
}
