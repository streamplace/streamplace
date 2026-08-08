import {
  ColorPicker,
  ColorPickerFormat,
  ColorPickerHue,
  ColorPickerOutput,
  ColorPickerSelection,
} from "@/components/ui/color-picker";
import { Switch } from "@/components/ui/switch";
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
        <div className="text-sm text-(--color-fg-muted)">
          {t("chat-profile-login-required")}
        </div>
      </div>
    );
  }

  const handleSave = async () => {
    setSaving(true);
    try {
      const rgb = hexToRgb(color);
      const selfLabels = isBot ? ["bot"] : undefined;

      await createChatProfileRecord(rgb.red, rgb.green, rgb.blue, selfLabels);

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
        <h3 className="mb-3 text-sm font-medium">
          {t("chat-profile-name-color")}
        </h3>
        <CardMenuSection>
          <div className="space-y-3 px-3 py-3">
            <p className="text-xs text-(--color-fg-muted)">
              {t("chat-profile-name-color-description")}
            </p>

            {/* Preview */}
            <div className="flex items-center gap-2 rounded-md bg-(--color-bg) px-3 py-2">
              <span className="text-sm" style={{ color }}>
                @{userProfile.handle}
              </span>
              <span className="text-sm text-(--color-fg-muted)">
                {t("chat-profile-preview-message")}
              </span>
            </div>
            <ColorPicker
              key={defaultHex}
              defaultValue={defaultHex}
              onChange={([red, green, blue]) =>
                setColor(
                  rgbToHex(
                    Math.round(red),
                    Math.round(green),
                    Math.round(blue),
                  ),
                )
              }
              className="bg-background max-w-sm rounded-md border p-4 shadow-sm"
            >
              <ColorPickerSelection />
              <div className="flex items-center gap-4">
                <ColorPickerHue />
              </div>
              <div className="flex items-center gap-2">
                <ColorPickerOutput />
                <ColorPickerFormat />
              </div>
            </ColorPicker>
          </div>
        </CardMenuSection>
      </div>

      {/* Self labels */}
      <div>
        <h3 className="mb-3 text-sm font-medium">
          {t("chat-profile-self-labels")}
        </h3>
        <CardMenuSection>
          <div className="flex items-center justify-between px-3 py-2.5">
            <div>
              <span className="text-sm">{t("chat-profile-label-bot")}</span>
              <p className="mt-0.5 text-xs text-(--color-fg-muted)">
                {t("chat-profile-label-bot-description")}
              </p>
            </div>
            <Switch checked={isBot} onCheckedChange={setIsBot} />
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
            className="rounded-md bg-(--color-accent) px-4 py-2 text-sm font-medium text-white transition-opacity hover:opacity-90 disabled:opacity-50"
          >
            {saving ? t("saving") : t("save-button")}
          </button>
        </div>
      )}
    </div>
  );
}
