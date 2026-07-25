import {
  Input,
  Loader,
  MenuContainer,
  MenuGroup,
  MenuItem,
  MenuLabel,
  MenuSeparator,
  Text,
  useTheme,
  useToast,
  View,
  zero,
} from "@streamplace/components";
import { usePDSAgent } from "@streamplace/components/src/streamplace-store/xrpc";
import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { ScrollView } from "react-native";
import { place } from "streamplace";
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

function validateS3Config(
  config: S3Config,
  t: (key: string) => string,
): ValidationErrors {
  const errors: ValidationErrors = {};

  if (
    config.endpoint &&
    !/^[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$/.test(config.endpoint)
  ) {
    errors.endpoint = t("backup-error-invalid-endpoint");
  }

  if (config.bucket && !/^[a-z0-9][a-z0-9.-]*[a-z0-9]$/.test(config.bucket)) {
    errors.bucket = t("backup-error-invalid-bucket");
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

function buildS3Url(config: S3Config, showPassword: boolean): string {
  const secretKey =
    (showPassword && config.secretKey !== "***") || !config.secretKey
      ? config.secretKey
      : "[hidden]";
  return `s3+https://${config.accessKey}:${secretKey}@${config.endpoint}/${config.bucket}`;
}

export function BackupSettings() {
  const { t } = useTranslation(["settings", "common"]);
  const { theme, zero: z } = useTheme();
  const toast = useToast();
  const agent = usePDSAgent();
  const [enabled, setEnabled] = useState(false);
  const [showPassword, setShowPassword] = useState(false);
  const [fullUrl, setFullUrl] = useState("");
  const [originalUrl, setOriginalUrl] = useState("");
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
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const isCensored = config.secretKey === "***";

  useEffect(() => {
    loadStorage();
  }, [agent]);

  useEffect(() => {
    setFullUrl(buildS3Url(config, showPassword));
  }, [showPassword]);

  const loadStorage = async () => {
    if (!agent) return;

    try {
      setLoading(true);
      const response = await agent.client.call(place.stream.server.getStorage);
      if (response.storage) {
        setOriginalUrl(response.storage.url);
        setEnabled(response.storage.isActive);
        const parsed = parseS3Url(response.storage.url);
        if (parsed) {
          setConfig(parsed);
          setFullUrl(buildS3Url(parsed, showPassword));
        }
      }
    } catch (error: any) {
      console.error("Failed to load storage settings:", error);
      toast.show(
        t("common:error"),
        error.message || t("backup-error-load-failed"),
        {
          variant: "error",
        },
      );
    } finally {
      setLoading(false);
    }
  };

  const handleEnabledChange = async (value: boolean) => {
    if (!agent) return;
    const previous = enabled;
    setEnabled(value);
    try {
      await agent.client.call(place.stream.server.upsertStorage, {
        isActive: value,
      });
    } catch (err: any) {
      console.error("Failed to toggle backup:", err);
      setEnabled(previous);
      toast.show(
        t("common:error"),
        err.message || t("backup-error-update-failed"),
        {
          variant: "error",
        },
      );
    }
  };

  const handleFullUrlChange = (url: string) => {
    setFullUrl(url);
    const parsed = parseS3Url(url);
    if (parsed) {
      if (parsed.secretKey === "[hidden]") {
        parsed.secretKey = config.secretKey;
      } else {
        // toggle "show password" on if user manually edits the URL to include the password
        setShowPassword(true);
      }
      setConfig(parsed);
    }
  };

  const handleConfigChange = (key: keyof S3Config, value: string) => {
    const newConfig = { ...config, [key]: value };
    setConfig(newConfig);
    setFullUrl(buildS3Url(newConfig, showPassword));
    setValidationErrors((prev) => ({
      ...prev,
      ...validateS3Config(newConfig, t),
    }));
  };

  const isComplete =
    !!config.endpoint &&
    !!config.bucket &&
    !!config.accessKey &&
    !!config.secretKey;
  const hasErrors = Object.keys(validationErrors).length > 0;
  const canSave = isComplete && !hasErrors && !saving;

  const handleSave = async () => {
    if (!agent || !canSave) return;

    try {
      setSaving(true);
      const realUrl = `s3+https://${config.accessKey}:${config.secretKey}@${config.endpoint}/${config.bucket}`;
      const payload: {
        url?: string;
      } = {};

      if (config.secretKey !== "***") {
        if (realUrl !== originalUrl) {
          payload.url = realUrl;
        }
      } else {
        // If secret key is masked, we check if other parts changed.
        // If they changed, we can't save without the secret key.
        const parsedOriginal = parseS3Url(originalUrl);
        if (parsedOriginal) {
          if (
            parsedOriginal.endpoint !== config.endpoint ||
            parsedOriginal.bucket !== config.bucket ||
            parsedOriginal.accessKey !== config.accessKey
          ) {
            throw new Error(t("backup-error-missing-secret"));
          }
        }
      }

      await agent.client.call(place.stream.server.upsertStorage, payload);
      await loadStorage();
    } catch (error: any) {
      console.error("Failed to save storage settings:", error);
      toast.show(
        t("common:error"),
        error.message || t("backup-error-save-failed"),
        {
          variant: "error",
        },
      );
    } finally {
      setSaving(false);
    }
  };

  if (loading) {
    return (
      <View
        style={[
          zero.layout.flex.justify.center,
          zero.layout.flex.align.center,
          { height: 200 },
        ]}
      >
        <Loader />
      </View>
    );
  }

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
                onValueChange={handleEnabledChange}
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
                        parseS3Url(t("backup-connection-url-placeholder")) ?? {
                          endpoint: "s3.us-east-1.example.com",
                          bucket: "my-bucket",
                          accessKey: "ACCESS_KEY",
                          secretKey: "SECRET_KEY",
                        },
                        showPassword,
                      )}
                      autoCapitalize="none"
                      autoCorrect={false}
                      containerStyle={[
                        {
                          width: "100%",
                          backgroundColor: theme.colors.muted,
                        },
                        zero.r.md,
                        zero.borders.width.thin,
                        z.border.border,
                      ]}
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
                        value={isCensored ? "" : config.secretKey}
                        onChangeText={(value) =>
                          handleConfigChange("secretKey", value)
                        }
                        placeholder={
                          isCensored
                            ? t("backup-secret-key-set-placeholder")
                            : t("backup-secret-key-placeholder")
                        }
                        variant="underlined"
                        secureTextEntry={!isCensored}
                        autoCapitalize="none"
                        autoCorrect={false}
                        containerStyle={{ width: "100%" }}
                        inputStyle={{ textAlign: "right" }}
                      />
                    </View>
                  </MenuItem>
                </MenuGroup>
                <MenuGroup>
                  <SettingsRowItem onPress={canSave ? handleSave : undefined}>
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
                          opacity: canSave ? 1 : 0.5,
                        }}
                      >
                        {saving ? t("backup-saving") : t("backup-save")}
                      </Text>
                    </View>
                  </SettingsRowItem>
                </MenuGroup>
              </>
            )}
          </MenuContainer>
        </View>
      </View>
    </ScrollView>
  );
}
