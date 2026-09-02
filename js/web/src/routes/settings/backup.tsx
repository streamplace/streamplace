import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Switch } from "@/components/ui/switch";
import { createFileRoute } from "@tanstack/react-router";
import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import { place } from "streamplace";
import { usePDSAgent } from "../../lib/store/hooks";

export const Route = createFileRoute("/settings/backup")({
  component: BackupSettings,
});

interface S3Config {
  endpoint: string;
  bucket: string;
  accessKey: string;
  secretKey: string;
}

function parseS3Url(url: string): S3Config | null {
  try {
    const match = url.match(/^s3\+https?:\/\/([^:]+):([^@]+)@([^/]+)\/(.+)$/);
    if (!match) return null;
    const [, accessKey, secretKey, endpoint, bucket] = match;
    return { endpoint, bucket, accessKey, secretKey };
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

function BackupSettings() {
  const { t } = useTranslation("settings");
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
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const isCensored = config.secretKey === "***";

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
      toast.error(error.message || "Failed to load storage settings");
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    if (agent) loadStorage();
  }, [agent]);

  useEffect(() => {
    setFullUrl(buildS3Url(config, showPassword));
  }, [showPassword]);

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
      toast.error(err.message || "Failed to update backup setting");
    }
  };

  const handleFullUrlChange = (url: string) => {
    setFullUrl(url);
    const parsed = parseS3Url(url);
    if (parsed) {
      if (parsed.secretKey === "[hidden]") {
        parsed.secretKey = config.secretKey;
      } else {
        setShowPassword(true);
      }
      setConfig(parsed);
    }
  };

  const handleConfigChange = (key: keyof S3Config, value: string) => {
    setConfig((prev) => {
      const next = { ...prev, [key]: value };
      setFullUrl(buildS3Url(next, showPassword));
      return next;
    });
  };

  const isComplete =
    !!config.endpoint &&
    !!config.bucket &&
    !!config.accessKey &&
    !!config.secretKey;

  const handleSave = async () => {
    if (!agent || !isComplete) return;
    try {
      setSaving(true);
      const realUrl = `s3+https://${config.accessKey}:${config.secretKey}@${config.endpoint}/${config.bucket}`;
      const payload: { url?: string } = {};

      if (config.secretKey !== "***") {
        if (realUrl !== originalUrl) {
          payload.url = realUrl;
        }
      } else {
        const parsedOriginal = parseS3Url(originalUrl);
        if (parsedOriginal) {
          if (
            parsedOriginal.endpoint !== config.endpoint ||
            parsedOriginal.bucket !== config.bucket ||
            parsedOriginal.accessKey !== config.accessKey
          ) {
            throw new Error(
              "Cannot save with masked secret key. Enter the full URL with the secret key.",
            );
          }
        }
      }

      await agent.client.call(place.stream.server.upsertStorage, payload);
      await loadStorage();
      toast.success("Backup settings saved");
    } catch (error: any) {
      console.error("Failed to save storage settings:", error);
      toast.error(error.message || "Failed to save storage settings");
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="space-y-6">
      <h1 className="font-display text-xl font-semibold">{t("backup")}</h1>

      {loading ? (
        <div className="text-sm text-(--color-fg-muted)">Loading…</div>
      ) : (
        <div className="space-y-4">
          {/* Enable toggle */}
          <div className="rounded-lg border border-(--color-border) bg-(--color-bg-elevated) p-4">
            <div className="flex items-center justify-between">
              <div>
                <div className="text-sm font-medium">{t("backup-enabled")}</div>
                <div className="mt-0.5 text-xs text-(--color-fg-muted)">
                  {t("backup-enabled-description")}
                </div>
              </div>
              <Switch checked={enabled} onCheckedChange={handleEnabledChange} />
            </div>
          </div>

          {/* S3 configuration (shown when enabled) */}
          {enabled && (
            <>
              <div className="space-y-4 rounded-lg border border-(--color-border) bg-(--color-bg-elevated) p-4">
                <div>
                  <label className="mb-1 block text-xs font-medium text-(--color-fg-muted)">
                    {t("backup-connection-url")}
                  </label>
                  <Input
                    value={fullUrl}
                    onChange={(e) => handleFullUrlChange(e.target.value)}
                    placeholder={buildS3Url(
                      {
                        endpoint: "s3.us-east-1.example.com",
                        bucket: "my-bucket",
                        accessKey: "ACCESS_KEY",
                        secretKey: "SECRET_KEY",
                      },
                      showPassword,
                    )}
                    className="font-mono text-xs"
                  />
                </div>

                <div className="flex items-center justify-between">
                  <span className="text-xs text-(--color-fg-muted)">
                    {t("show-password-in-url")}
                  </span>
                  <Switch
                    size="sm"
                    checked={showPassword}
                    onCheckedChange={setShowPassword}
                  />
                </div>
              </div>

              <div className="divide-y divide-(--color-border) rounded-lg border border-(--color-border) bg-(--color-bg-elevated)">
                <div className="flex items-center justify-between px-3 py-2.5">
                  <span className="shrink-0 text-sm">
                    {t("backup-endpoint")}
                  </span>
                  <Input
                    value={config.endpoint}
                    onChange={(e) =>
                      handleConfigChange("endpoint", e.target.value)
                    }
                    placeholder="s3.us-east-1.example.com"
                    className="max-w-[250px] text-right font-mono text-xs"
                  />
                </div>
                <div className="flex items-center justify-between px-3 py-2.5">
                  <span className="shrink-0 text-sm">{t("backup-bucket")}</span>
                  <Input
                    value={config.bucket}
                    onChange={(e) =>
                      handleConfigChange("bucket", e.target.value)
                    }
                    placeholder="my-bucket"
                    className="max-w-[250px] text-right font-mono text-xs"
                  />
                </div>
                <div className="flex items-center justify-between px-3 py-2.5">
                  <span className="shrink-0 text-sm">
                    {t("backup-access-key")}
                  </span>
                  <Input
                    value={config.accessKey}
                    onChange={(e) =>
                      handleConfigChange("accessKey", e.target.value)
                    }
                    placeholder="ACCESS_KEY"
                    className="max-w-[250px] text-right font-mono text-xs"
                  />
                </div>
                <div className="flex items-center justify-between px-3 py-2.5">
                  <span className="shrink-0 text-sm">
                    {t("backup-secret-key")}
                  </span>
                  <Input
                    type={
                      isCensored ? "text" : showPassword ? "text" : "password"
                    }
                    value={isCensored ? "" : config.secretKey}
                    onChange={(e) =>
                      handleConfigChange("secretKey", e.target.value)
                    }
                    placeholder={
                      isCensored
                        ? t("backup-secret-key-set-placeholder")
                        : t("backup-secret-key-placeholder")
                    }
                    className="max-w-[250px] text-right font-mono text-xs"
                  />
                </div>
              </div>

              <Button
                onClick={handleSave}
                disabled={!isComplete || saving}
                className="w-full"
              >
                {saving ? t("backup-saving") : t("backup-save")}
              </Button>
            </>
          )}
        </div>
      )}
    </div>
  );
}
