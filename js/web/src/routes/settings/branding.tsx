import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { createFileRoute } from "@tanstack/react-router";
import { useState } from "react";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import { usePDSAgent } from "../../lib/store/hooks";

export const Route = createFileRoute("/settings/branding")({
  component: BrandingAdmin,
});

function BrandingAdmin() {
  const { t } = useTranslation("settings");
  const agent = usePDSAgent();

  const [uploading, setUploading] = useState(false);
  const [broadcasterDID, setBroadcasterDID] = useState("");

  // Text fields
  const [siteTitle, setSiteTitle] = useState("");
  const [siteDescription, setSiteDescription] = useState("");
  const [primaryColor, setPrimaryColor] = useState("");
  const [accentColor, setAccentColor] = useState("");
  const [defaultStreamer, setDefaultStreamer] = useState("");

  // Legal links
  const [legalLinkText, setLegalLinkText] = useState("");
  const [legalLinkUrl, setLegalLinkUrl] = useState("");
  const [editingLinkIndex, setEditingLinkIndex] = useState<number | null>(null);
  const [legalLinks, setLegalLinks] = useState<{ text: string; url: string }[]>(
    [],
  );

  const uploadText = async (key: string, value: string) => {
    if (!agent) return;
    if (!value.trim()) {
      toast.error("Value cannot be empty");
      return;
    }
    try {
      setUploading(true);
      const textBytes = new TextEncoder().encode(value.trim());
      const base64Data = btoa(String.fromCharCode(...textBytes));
      await agent.place.stream.branding.updateBlob({
        key,
        broadcaster: broadcasterDID || undefined,
        data: base64Data,
        mimeType: "text/plain",
      });
      toast.success(`${key} updated`);
    } catch (error: any) {
      toast.error(error.message || `Failed to update ${key}`);
    } finally {
      setUploading(false);
    }
  };

  const uploadFile = async (key: string, file: File) => {
    if (!agent) return;
    try {
      setUploading(true);
      const arrayBuffer = await file.arrayBuffer();
      const uint8Array = new Uint8Array(arrayBuffer);
      const base64Data = btoa(String.fromCharCode(...uint8Array));

      let width: number | undefined;
      let height: number | undefined;
      if (file.type.startsWith("image/")) {
        const img = new Image();
        const imageUrl = URL.createObjectURL(file);
        await new Promise<void>((resolve, reject) => {
          img.onload = () => {
            width = img.naturalWidth;
            height = img.naturalHeight;
            URL.revokeObjectURL(imageUrl);
            resolve();
          };
          img.onerror = () => {
            URL.revokeObjectURL(imageUrl);
            reject(new Error("Failed to load image"));
          };
          img.src = imageUrl;
        });
      }

      await agent.place.stream.branding.updateBlob({
        key,
        broadcaster: broadcasterDID || undefined,
        data: base64Data,
        mimeType: file.type,
        width,
        height,
      });
      toast.success(`${key} uploaded`);
    } catch (error: any) {
      toast.error(error.message || `Failed to upload ${key}`);
    } finally {
      setUploading(false);
    }
  };

  const deleteBlob = async (key: string) => {
    if (!agent) return;
    try {
      setUploading(true);
      await agent.place.stream.branding.deleteBlob({
        key,
        broadcaster: broadcasterDID || undefined,
      });
      toast.success(`${key} deleted`);
    } catch (error: any) {
      toast.error(error.message || `Failed to delete ${key}`);
    } finally {
      setUploading(false);
    }
  };

  const saveLegalLink = async () => {
    if (!legalLinkText.trim() || !legalLinkUrl.trim()) return;
    const updated = [...legalLinks];
    const newLink = { text: legalLinkText.trim(), url: legalLinkUrl.trim() };
    if (editingLinkIndex !== null) {
      updated[editingLinkIndex] = newLink;
    } else {
      updated.push(newLink);
    }
    setLegalLinks(updated);
    await uploadText("legalLinks", JSON.stringify(updated));
    setLegalLinkText("");
    setLegalLinkUrl("");
    setEditingLinkIndex(null);
  };

  const deleteLegalLink = async (index: number) => {
    const updated = legalLinks.filter((_, i) => i !== index);
    setLegalLinks(updated);
    if (updated.length === 0) {
      await deleteBlob("legalLinks");
    } else {
      await uploadText("legalLinks", JSON.stringify(updated));
    }
  };

  if (!agent) {
    return (
      <div className="text-sm text-[var(--color-fg-muted)]">
        {t("branding-login-required")}
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-xl font-semibold font-display">{t("branding-admin")}</h1>
        <p className="text-sm text-[var(--color-fg-muted)] mt-1">
          {t("branding-admin-description")}
        </p>
      </div>

      {uploading && (
        <div className="text-sm text-[var(--color-fg-muted)]">Uploading…</div>
      )}

      {/* Broadcaster DID */}
      <section className="space-y-3">
        <h2 className="text-sm font-medium uppercase tracking-wide text-[var(--color-fg-muted)]">
          {t("branding-configuration")}
        </h2>
        <div className="rounded-lg border border-[var(--color-border)] bg-[var(--color-bg-elevated)] p-4 space-y-2">
          <label className="text-xs font-medium text-[var(--color-fg-muted)] block">
            {t("branding-broadcaster-did")}
          </label>
          <Input
            value={broadcasterDID}
            onChange={(e) => setBroadcasterDID(e.target.value)}
            placeholder={t("branding-default-streamer-placeholder")}
          />
          <p className="text-xs text-[var(--color-fg-muted)]">
            {t("branding-broadcaster-did-description")}
          </p>
        </div>
      </section>

      {/* Text settings */}
      <section className="space-y-3">
        <h2 className="text-sm font-medium uppercase tracking-wide text-[var(--color-fg-muted)]">
          {t("branding-text-settings")}
        </h2>
        <div className="rounded-lg border border-[var(--color-border)] bg-[var(--color-bg-elevated)] divide-y divide-[var(--color-border)]">
          <BrandingTextField
            label={t("branding-site-title")}
            value={siteTitle}
            onChange={setSiteTitle}
            placeholder={t("branding-site-title-placeholder")}
            onSave={() => uploadText("siteTitle", siteTitle)}
            disabled={uploading}
          />
          <BrandingTextField
            label={t("branding-site-description")}
            value={siteDescription}
            onChange={setSiteDescription}
            placeholder={t("branding-site-description-placeholder")}
            onSave={() => uploadText("siteDescription", siteDescription)}
            disabled={uploading}
          />
          <BrandingTextField
            label={t("branding-default-streamer")}
            value={defaultStreamer}
            onChange={setDefaultStreamer}
            placeholder={t("branding-default-streamer-placeholder")}
            onSave={() => uploadText("defaultStreamer", defaultStreamer)}
            onDelete={() => deleteBlob("defaultStreamer")}
            disabled={uploading}
          />
        </div>
      </section>

      {/* Colors */}
      <section className="space-y-3">
        <h2 className="text-sm font-medium uppercase tracking-wide text-[var(--color-fg-muted)]">
          {t("branding-colors")}
        </h2>
        <div className="rounded-lg border border-[var(--color-border)] bg-[var(--color-bg-elevated)] divide-y divide-[var(--color-border)]">
          <BrandingTextField
            label={t("branding-primary-color")}
            value={primaryColor}
            onChange={setPrimaryColor}
            placeholder={t("branding-primary-color-placeholder")}
            onSave={() => uploadText("primaryColor", primaryColor)}
            disabled={uploading}
          />
          <BrandingTextField
            label={t("branding-accent-color")}
            value={accentColor}
            onChange={setAccentColor}
            placeholder={t("branding-accent-color-placeholder")}
            onSave={() => uploadText("accentColor", accentColor)}
            disabled={uploading}
          />
        </div>
      </section>

      {/* Images */}
      <section className="space-y-3">
        <h2 className="text-sm font-medium uppercase tracking-wide text-[var(--color-fg-muted)]">
          {t("branding-images")}
        </h2>
        <div className="rounded-lg border border-[var(--color-border)] bg-[var(--color-bg-elevated)] divide-y divide-[var(--color-border)]">
          <BrandingImageField
            label={t("branding-main-logo")}
            description={t("branding-main-logo-description")}
            accept="image/svg+xml,image/png,image/jpeg,image/webp"
            onUpload={(file) => uploadFile("mainLogo", file)}
            onDelete={() => deleteBlob("mainLogo")}
            disabled={uploading}
          />
          <BrandingImageField
            label={t("branding-favicon")}
            description={t("branding-favicon-description")}
            accept="image/svg+xml,image/png,image/x-icon"
            onUpload={(file) => uploadFile("favicon", file)}
            onDelete={() => deleteBlob("favicon")}
            disabled={uploading}
          />
          <BrandingImageField
            label={t("branding-sidebar-bg")}
            description={t("branding-sidebar-bg-description")}
            accept="image/svg+xml,image/png,image/jpeg,image/webp"
            onUpload={(file) => uploadFile("sidebarBackgroundImage", file)}
            onDelete={() => deleteBlob("sidebarBackgroundImage")}
            disabled={uploading}
          />
        </div>
      </section>

      {/* Legal links */}
      <section className="space-y-3">
        <h2 className="text-sm font-medium uppercase tracking-wide text-[var(--color-fg-muted)]">
          {t("branding-legal-links")}
        </h2>
        <div className="rounded-lg border border-[var(--color-border)] bg-[var(--color-bg-elevated)] p-4 space-y-3">
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-2">
            <Input
              value={legalLinkText}
              onChange={(e) => setLegalLinkText(e.target.value)}
              placeholder={t("branding-legal-link-text-placeholder")}
            />
            <Input
              value={legalLinkUrl}
              onChange={(e) => setLegalLinkUrl(e.target.value)}
              placeholder={t("branding-legal-link-url-placeholder")}
            />
          </div>
          <div className="flex gap-2">
            <Button
              onClick={saveLegalLink}
              disabled={
                uploading || !legalLinkText.trim() || !legalLinkUrl.trim()
              }
              size="sm"
            >
              {editingLinkIndex !== null ? t("update") : "Add"}
            </Button>
            {editingLinkIndex !== null && (
              <Button
                variant="secondary"
                onClick={() => {
                  setEditingLinkIndex(null);
                  setLegalLinkText("");
                  setLegalLinkUrl("");
                }}
                size="sm"
              >
                {t("cancel")}
              </Button>
            )}
          </div>
        </div>

        {legalLinks.length > 0 && (
          <div className="rounded-lg border border-[var(--color-border)] bg-[var(--color-bg-elevated)] divide-y divide-[var(--color-border)]">
            {legalLinks.map((link, index) => (
              <div
                key={index}
                className="flex items-center justify-between px-3 py-2.5"
              >
                <div className="min-w-0">
                  <div className="text-sm font-medium">{link.text}</div>
                  <div className="text-xs text-[var(--color-fg-muted)] truncate">
                    {link.url}
                  </div>
                </div>
                <div className="flex gap-1 shrink-0 ml-2">
                  <Button
                    variant="ghost"
                    size="xs"
                    onClick={() => {
                      setEditingLinkIndex(index);
                      setLegalLinkText(link.text);
                      setLegalLinkUrl(link.url);
                    }}
                  >
                    {t("edit")}
                  </Button>
                  <Button
                    variant="destructive"
                    size="xs"
                    onClick={() => deleteLegalLink(index)}
                    disabled={uploading}
                  >
                    {t("delete")}
                  </Button>
                </div>
              </div>
            ))}
          </div>
        )}
      </section>
    </div>
  );
}

function BrandingTextField({
  label,
  value,
  onChange,
  placeholder,
  onSave,
  onDelete,
  disabled,
}: {
  label: string;
  value: string;
  onChange: (v: string) => void;
  placeholder: string;
  onSave: () => void;
  onDelete?: () => void;
  disabled: boolean;
}) {
  const { t } = useTranslation("settings");
  return (
    <div className="p-4 space-y-2">
      <div className="text-sm font-medium">{label}</div>
      <div className="flex gap-2">
        <Input
          value={value}
          onChange={(e) => onChange(e.target.value)}
          placeholder={placeholder}
          className="flex-1"
        />
        <Button onClick={onSave} disabled={disabled || !value.trim()} size="sm">
          {t("update")}
        </Button>
      </div>
      {onDelete && (
        <Button
          variant="destructive"
          size="sm"
          onClick={onDelete}
          disabled={disabled}
        >
          {t("branding-clear-default-streamer")}
        </Button>
      )}
    </div>
  );
}

function BrandingImageField({
  label,
  description,
  accept,
  onUpload,
  onDelete,
  disabled,
}: {
  label: string;
  description: string;
  accept: string;
  onUpload: (file: File) => void;
  onDelete: () => void;
  disabled: boolean;
}) {
  const { t } = useTranslation("settings");

  const handleSelect = () => {
    const input = document.createElement("input");
    input.type = "file";
    input.accept = accept;
    input.onchange = (e) => {
      const file = (e.target as HTMLInputElement).files?.[0];
      if (file) onUpload(file);
    };
    input.click();
  };

  return (
    <div className="p-4 space-y-2">
      <div className="text-sm font-medium">{label}</div>
      <div className="text-xs text-[var(--color-fg-muted)]">{description}</div>
      <div className="flex gap-2">
        <Button onClick={handleSelect} disabled={disabled} size="sm">
          Upload
        </Button>
        <Button
          variant="destructive"
          onClick={onDelete}
          disabled={disabled}
          size="sm"
        >
          {t("delete")}
        </Button>
      </div>
    </div>
  );
}
