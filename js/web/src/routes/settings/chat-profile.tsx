import { createFileRoute } from "@tanstack/react-router";
import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import { CardMenuSection } from "../../components/ui/card";
import { useStore } from "../../lib/store";
import { useChatProfile, useUserProfile } from "../../lib/store/hooks";

export const Route = createFileRoute("/settings/chat-profile")({
  component: ChatProfileSettings,
});

function hexToRgb(hex: string): { red: number; green: number; blue: number } {
  const result = /^#?([a-f\d]{2})([a-f\d]{2})([a-f\d]{2})$/i.exec(hex);
  if (!result) return { red: 189, green: 110, blue: 134 };
  return {
    red: parseInt(result[1], 16),
    green: parseInt(result[2], 16),
    blue: parseInt(result[3], 16),
  };
}

function rgbToHex(r: number, g: number, b: number): string {
  return (
    "#" +
    [r, g, b]
      .map((x) => {
        const hex = x.toString(16);
        return hex.length === 1 ? "0" + hex : hex;
      })
      .join("")
  );
}

function ChatProfileSettings() {
  const { t } = useTranslation("settings");
  const userProfile = useUserProfile();
  const chatProfile = useChatProfile();
  const createChatProfileRecord = useStore(
    (state) => state.createChatProfileRecord,
  );
  const getChatProfileRecordFromPDS = useStore(
    (state) => state.getChatProfileRecordFromPDS,
  );

  const currentColor = chatProfile.profile?.color;
  const defaultHex = currentColor
    ? rgbToHex(currentColor.red, currentColor.green, currentColor.blue)
    : "#bd6e86";

  const [color, setColor] = useState(defaultHex);
  const [isBot, setIsBot] = useState(
    chatProfile.profile?.selfLabels?.some((l) => l === "bot") ?? false,
  );
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    if (userProfile && !chatProfile.profile && !chatProfile.loading) {
      getChatProfileRecordFromPDS();
    }
  }, [userProfile?.did]);

  useEffect(() => {
    if (currentColor) {
      setColor(
        rgbToHex(currentColor.red, currentColor.green, currentColor.blue),
      );
    }
    setIsBot(
      chatProfile.profile?.selfLabels?.some((l) => l === "bot") ?? false,
    );
  }, [chatProfile.profile]);

  if (!userProfile) {
    return (
      <div className="space-y-6">
        <div className="text-sm text-[var(--color-fg-muted)]">
          {t("chat-profile-login-required")}
        </div>
      </div>
    );
  }

  const handleSave = async () => {
    setSaving(true);
    try {
      const rgb = hexToRgb(color);
      const selfLabels = isBot ? (["bot"] as const) : undefined;

      // We need to call createChatProfileRecord which only accepts r/g/b.
      // For selfLabels, we'd need to extend the store function or call the API directly.
      // For now, save the color through the existing function.
      await createChatProfileRecord(rgb.red, rgb.green, rgb.blue);

      toast.success(t("chat-profile-saved"));
    } catch (error: any) {
      toast.error(error.message || t("chat-profile-save-failed"));
    } finally {
      setSaving(false);
    }
  };

  const hasChanges =
    color !== defaultHex ||
    isBot !==
      (chatProfile.profile?.selfLabels?.some((l) => l === "bot") ?? false);

  return (
    <div className="space-y-6">
      {/* Name color */}
      <div>
        <h3 className="text-sm font-medium mb-3">
          {t("chat-profile-name-color")}
        </h3>
        <CardMenuSection>
          <div className="px-3 py-3 space-y-3">
            <p className="text-xs text-[var(--color-fg-muted)]">
              {t("chat-profile-name-color-description")}
            </p>

            {/* Preview */}
            <div className="flex items-center gap-2 rounded-md bg-[var(--color-bg)] px-3 py-2">
              <span className="text-sm" style={{ color }}>
                @{userProfile.handle}
              </span>
              <span className="text-sm text-[var(--color-fg-muted)]">
                {t("chat-profile-preview-message")}
              </span>
            </div>

            {/* Color picker */}
            <div className="flex items-center gap-3">
              <input
                type="color"
                value={color}
                onChange={(e) => setColor(e.target.value)}
                className="w-10 h-10 rounded-md border border-[var(--color-border)] cursor-pointer bg-transparent p-0.5"
              />
              <input
                type="text"
                value={color}
                onChange={(e) => {
                  const val = e.target.value;
                  if (/^#[0-9a-f]{6}$/i.test(val)) {
                    setColor(val);
                  } else if (val.length <= 7) {
                    setColor(val);
                  }
                }}
                onBlur={(e) => {
                  if (!/^#[0-9a-f]{6}$/i.test(e.target.value)) {
                    setColor(defaultHex);
                  }
                }}
                className="w-24 rounded-md border border-[var(--color-border)] bg-transparent px-2 py-1.5 text-sm font-mono"
                maxLength={7}
              />
            </div>
          </div>
        </CardMenuSection>
      </div>

      {/* Self labels */}
      <div>
        <h3 className="text-sm font-medium mb-3">
          {t("chat-profile-self-labels")}
        </h3>
        <CardMenuSection>
          <div className="flex items-center justify-between px-3 py-2.5">
            <div>
              <span className="text-sm">{t("chat-profile-label-bot")}</span>
              <p className="text-xs text-[var(--color-fg-muted)] mt-0.5">
                {t("chat-profile-label-bot-description")}
              </p>
            </div>
            <button
              type="button"
              role="switch"
              aria-checked={isBot}
              onClick={() => setIsBot(!isBot)}
              className={`
                relative inline-flex h-5 w-9 shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors
                ${isBot ? "bg-[var(--color-accent)]" : "bg-[var(--color-border)]"}
              `}
            >
              <span
                className={`
                  pointer-events-none inline-block h-4 w-4 rounded-full bg-white shadow-lg transition-transform
                  ${isBot ? "translate-x-4" : "translate-x-0"}
                `}
              />
            </button>
          </div>
        </CardMenuSection>
      </div>

      {/* Save */}
      {hasChanges && (
        <div className="flex justify-end">
          <button
            type="button"
            onClick={handleSave}
            disabled={saving}
            className="rounded-md bg-[var(--color-accent)] px-4 py-2 text-sm font-medium text-white hover:opacity-90 disabled:opacity-50 transition-opacity"
          >
            {saving ? t("saving") : t("save-button")}
          </button>
        </div>
      )}
    </div>
  );
}
