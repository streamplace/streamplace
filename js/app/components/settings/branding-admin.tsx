import {
  Button,
  DEFAULT_CHROME,
  Input,
  MenuContainer,
  MenuGroup,
  MenuInfo,
  MenuItem,
  MenuLabel,
  MenuSeparator,
  SegmentedTabs,
  Text,
  useStreamplaceStore,
  useTheme,
  useToast,
  useTranslation,
  View,
  zero,
} from "@streamplace/components";
import {
  useBrandingAsset,
  useFetchBranding,
  useSidebarBackgroundImage,
} from "@streamplace/components/src/streamplace-store/branding";
import { usePDSAgent } from "@streamplace/components/src/streamplace-store/xrpc";
import { Image } from "expo-image";
import { useEffect, useState } from "react";
import {
  ActivityIndicator,
  Platform,
  Pressable,
  ScrollView,
  TextInput,
} from "react-native";
import { place } from "streamplace";
import { SettingsRowItem } from "./components/settings-navigation-item";

const NAV_LINKS_EXAMPLE =
  '[{"label": "Home", "url": "https://example.com", "icon": "home"}, {"label": "Live", "url": "/", "icon": "play"}]';
const NAV_CTA_EXAMPLE =
  '{"label": "New post", "url": "https://example.com/compose"}';
const SOCIAL_LINKS_EXAMPLE =
  '[{"label": "Bluesky", "url": "https://bsky.app/profile/example.com", "icon": "bluesky"}, {"label": "Forum", "url": "https://example.com/forum", "icon": "socialIcon1"}]';
const BOTTOM_LINKS_EXAMPLE =
  '[{"label": "Help", "url": "https://example.com/help", "icon": "book"}]';
const SOCIAL_ICON_SLOTS = [
  "socialIcon1",
  "socialIcon2",
  "socialIcon3",
  "socialIcon4",
];

// Base64 without spreading the whole buffer into one call: a 2MB link
// banner spread into String.fromCharCode overflows the call stack.
function bytesToBase64(bytes: Uint8Array): string {
  let binary = "";
  const chunk = 0x8000;
  for (let i = 0; i < bytes.length; i += chunk) {
    binary += String.fromCharCode(...bytes.subarray(i, i + chunk));
  }
  return btoa(binary);
}

export function BrandingAdmin() {
  const { t } = useTranslation("settings");
  const { theme } = useTheme();
  const agent = usePDSAgent();
  const fetchBranding = useFetchBranding();
  const toast = useToast();
  const currentBroadcasterDID = useStreamplaceStore((s) => s.broadcasterDID);

  // state for form inputs
  const [siteTitle, setSiteTitle] = useState("");
  const [siteDescription, setSiteDescription] = useState("");
  const [primaryColor, setPrimaryColor] = useState("");
  const [accentColor, setAccentColor] = useState("");
  const [chromeInputs, setChromeInputs] = useState<Record<string, string>>({});
  const [defaultStreamer, setDefaultStreamer] = useState("");
  const [broadcasterDID, setBroadcasterDID] = useState("");
  const [uploading, setUploading] = useState(false);
  const [legalLinkText, setLegalLinkText] = useState("");
  const [legalLinkUrl, setLegalLinkUrl] = useState("");
  const [editingLinkIndex, setEditingLinkIndex] = useState<number | null>(null);

  // get current values
  const currentTitle = useBrandingAsset("siteTitle");
  const currentDescription = useBrandingAsset("siteDescription");
  const currentPrimaryColor = useBrandingAsset("primaryColor");
  const currentAccentColor = useBrandingAsset("accentColor");
  const branding = useStreamplaceStore((st) => st.branding);
  const brandingValue = (key: string) => branding?.[key]?.data || "";

  // Bundles: everything set on the node as a zip (branding.yaml + images).
  // Import previews with a dry run first, then applies on confirmation.
  const [bundleBusy, setBundleBusy] = useState(false);
  const [bundleMerge, setBundleMerge] = useState(false);
  const [bundlePreview, setBundlePreview] = useState<{
    bytes: Uint8Array;
    name: string;
    changes: { key: string; action: string; detail?: string }[];
    warnings: string[];
  } | null>(null);

  const exportBundle = async () => {
    if (!agent) return;
    setBundleBusy(true);
    try {
      const bytes = (await agent.client.call(
        place.stream.branding.exportBundle,
        { broadcaster: (broadcasterDID || undefined) as any },
      )) as unknown as Uint8Array;
      const blob = new Blob([bytes as any], { type: "application/zip" });
      const url = URL.createObjectURL(blob);
      const a = document.createElement("a");
      a.href = url;
      a.download = "branding.zip";
      document.body.appendChild(a);
      a.click();
      a.remove();
      setTimeout(() => URL.revokeObjectURL(url), 10_000);
    } catch (e: any) {
      toast.show(t("branding-bundle-export-failed"), e?.message, {
        variant: "error",
      });
    } finally {
      setBundleBusy(false);
    }
  };

  const runImport = async (
    bytes: Uint8Array,
    dryRun: boolean,
  ): Promise<{
    applied: boolean;
    changes: { key: string; action: string; detail?: string }[];
    warnings?: string[];
  }> => {
    if (!agent) throw new Error("not logged in");
    return (await agent.client.call(
      place.stream.branding.importBundle,
      bytes as any,
      {
        params: {
          broadcaster: (broadcasterDID || undefined) as any,
          dryRun,
          merge: bundleMerge,
        },
      } as any,
    )) as any;
  };

  const pickBundle = () => {
    if (Platform.OS !== "web") return;
    const input = document.createElement("input");
    input.type = "file";
    input.accept = ".zip,application/zip";
    input.onchange = async () => {
      const file = input.files?.[0];
      if (!file) return;
      setBundleBusy(true);
      try {
        const bytes = new Uint8Array(await file.arrayBuffer());
        const res = await runImport(bytes, true);
        setBundlePreview({
          bytes,
          name: file.name,
          changes: res.changes,
          warnings: res.warnings ?? [],
        });
      } catch (e: any) {
        toast.show(t("branding-bundle-invalid"), e?.message, {
          variant: "error",
        });
      } finally {
        setBundleBusy(false);
      }
    };
    input.click();
  };

  const applyBundle = async () => {
    if (!bundlePreview) return;
    setBundleBusy(true);
    try {
      const res = await runImport(bundlePreview.bytes, false);
      toast.show(
        t("branding-bundle-imported", { count: res.changes.length }),
        undefined,
        { variant: "success" },
      );
      setBundlePreview(null);
      await fetchBranding();
    } catch (e: any) {
      toast.show(t("branding-bundle-import-failed"), e?.message, {
        variant: "error",
      });
    } finally {
      setBundleBusy(false);
    }
  };
  const currentBgDark = useBrandingAsset("backgroundColor");
  const currentFgDark = useBrandingAsset("foregroundColor");
  const currentBgLight = useBrandingAsset("backgroundColorLight");
  const currentFgLight = useBrandingAsset("foregroundColorLight");
  const currentDefaultStreamer = useBrandingAsset("defaultStreamer");
  const currentLogo = useBrandingAsset("mainLogo");
  const currentFavicon = useBrandingAsset("favicon");
  const currentSidebarBg = useSidebarBackgroundImage();
  const currentLinkBanner = useBrandingAsset("linkBanner");
  const currentLegalLinks = useBrandingAsset("legalLinks");

  // parse legal links
  const legalLinks: { text: string; url: string }[] = currentLegalLinks?.data
    ? JSON.parse(currentLegalLinks.data)
    : [];

  // load current branding on mount
  useEffect(() => {
    fetchBranding();
  }, []);

  useEffect(() => {
    setBroadcasterDID(currentBroadcasterDID || "");
  }, [currentBroadcasterDID]);

  const uploadText = async (key: string, value: string) => {
    if (!agent) {
      toast.show(
        t("branding-not-authenticated"),
        t("branding-not-authenticated"),
        {
          variant: "error",
        },
      );
      return;
    }

    if (!value.trim()) {
      toast.show(t("branding-empty-value"), t("branding-empty-value"), {
        variant: "error",
      });
      return;
    }

    setUploading(true);
    try {
      const textBytes = new TextEncoder().encode(value.trim());
      const base64Data = bytesToBase64(textBytes);

      await agent.client.call(place.stream.branding.updateBlob, {
        key,
        broadcaster: (broadcasterDID || undefined) as any,
        data: base64Data,
        mimeType: "text/plain",
      });

      toast.show(
        t("branding-update-success", { key }),
        t("branding-update-success", { key }),
        {
          variant: "success",
        },
      );

      // clear input based on key
      switch (key) {
        case "siteTitle":
          setSiteTitle("");
          break;
        case "siteDescription":
          setSiteDescription("");
          break;
        case "primaryColor":
          setPrimaryColor("");
          break;
        case "accentColor":
          setAccentColor("");
          break;
        case "backgroundColor":
        case "foregroundColor":
        case "backgroundColorLight":
        case "foregroundColorLight":
        case "dangerColor":
        case "successColor":
        case "warningColor":
        case "infoColor":
        case "liveColor":
        case "navLinks":
        case "navCta":
        case "streamLayout":
        case "typeface":
        case "chatLayout":
        case "socialHeading":
        case "socialLinks":
        case "bottomLinks":
        case "networkName":
        case "loginPlaceholder":
          setChromeInputs((prev) => ({ ...prev, [key]: "" }));
          break;
        case "defaultStreamer":
          setDefaultStreamer("");
          break;
      }

      // reload branding
      setTimeout(() => fetchBranding({ force: true }), 500);
    } catch (err: any) {
      toast.show(
        t("branding-upload-failed"),
        err.message || t("branding-upload-failed"),
        {
          variant: "error",
        },
      );
    } finally {
      setUploading(false);
    }
  };

  const uploadFile = async (key: string, file: File) => {
    if (!agent) {
      toast.show(
        t("branding-not-authenticated"),
        t("branding-not-authenticated"),
        {
          variant: "error",
        },
      );
      return;
    }

    setUploading(true);
    try {
      const arrayBuffer = await file.arrayBuffer();
      const uint8Array = new Uint8Array(arrayBuffer);
      const base64Data = bytesToBase64(uint8Array);

      // detect image dimensions if it's an image
      let width: number | undefined;
      let height: number | undefined;

      if (file.type.startsWith("image/") && Platform.OS === "web") {
        const img = new window.Image();
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

      await agent.client.call(place.stream.branding.updateBlob, {
        key,
        broadcaster: (broadcasterDID || undefined) as any,
        data: base64Data,
        mimeType: file.type,
        width,
        height,
      });

      toast.show(
        t("branding-update-success", { key }),
        t("branding-upload-success", { key }),
        {
          variant: "success",
        },
      );

      // reload branding
      setTimeout(() => fetchBranding({ force: true }), 500);
    } catch (err: any) {
      toast.show(
        t("branding-upload-failed"),
        err.message || t("branding-upload-failed"),
        {
          variant: "error",
        },
      );
    } finally {
      setUploading(false);
    }
  };

  const handleFileSelect = (key: string, accept: string) => {
    if (Platform.OS !== "web") {
      toast.show(t("branding-not-available"), t("branding-not-available"), {
        variant: "error",
      });
      return;
    }

    // TypeScript doesn't know about document in react-native-web context
    // @ts-ignore - document exists on web
    const input = document.createElement("input");
    input.type = "file";
    input.accept = accept;
    input.onchange = (e) => {
      const file = (e.target as HTMLInputElement).files?.[0];
      if (file) {
        uploadFile(key, file);
      }
    };
    input.click();
  };

  const deleteBlob = async (key: string) => {
    if (!agent) {
      toast.show(
        t("branding-not-authenticated"),
        t("branding-not-authenticated"),
        {
          variant: "error",
        },
      );
      return;
    }

    setUploading(true);
    try {
      await agent.client.call(place.stream.branding.deleteBlob, {
        key,
        broadcaster: (broadcasterDID || undefined) as any,
      });

      toast.show(
        t("branding-update-success", { key }),
        t("branding-delete-success", { key }),
        {
          variant: "success",
        },
      );

      // reload branding
      setTimeout(() => fetchBranding(), 500);
    } catch (err: any) {
      toast.show(
        t("branding-delete-failed"),
        err.message || t("branding-delete-failed"),
        {
          variant: "error",
        },
      );
    } finally {
      setUploading(false);
    }
  };

  const saveLegalLink = async () => {
    if (!legalLinkText.trim() || !legalLinkUrl.trim()) {
      toast.show(t("branding-empty-value"), t("branding-empty-value"), {
        variant: "error",
      });
      return;
    }

    const updatedLinks = [...legalLinks];
    const newLink = { text: legalLinkText.trim(), url: legalLinkUrl.trim() };

    if (editingLinkIndex !== null) {
      updatedLinks[editingLinkIndex] = newLink;
    } else {
      updatedLinks.push(newLink);
    }

    await uploadText("legalLinks", JSON.stringify(updatedLinks));
    setLegalLinkText("");
    setLegalLinkUrl("");
    setEditingLinkIndex(null);
  };

  const deleteLegalLink = async (index: number) => {
    const updatedLinks = legalLinks.filter((_, i) => i !== index);
    if (updatedLinks.length === 0) {
      await deleteBlob("legalLinks");
    } else {
      await uploadText("legalLinks", JSON.stringify(updatedLinks));
    }
  };

  const startEditingLink = (index: number) => {
    setEditingLinkIndex(index);
    setLegalLinkText(legalLinks[index].text);
    setLegalLinkUrl(legalLinks[index].url);
  };

  const cancelEditingLink = () => {
    setEditingLinkIndex(null);
    setLegalLinkText("");
    setLegalLinkUrl("");
  };

  if (!agent) {
    return (
      <View style={[zero.layout.flex.align.center, zero.px[16], zero.py[24]]}>
        <Text>{t("branding-login-required")}</Text>
      </View>
    );
  }

  return (
    <ScrollView>
      <View style={[zero.layout.flex.align.center, zero.px[2], zero.py[2]]}>
        <View style={{ maxWidth: 500, width: "100%" }}>
          <MenuContainer>
            <View style={[zero.gap.all[2]]}>
              <Text size="2xl" weight="bold">
                {t("branding-admin")}
              </Text>
              <Text color="muted">{t("branding-admin-description")}</Text>
            </View>

            {uploading && (
              <View style={[zero.layout.flex.align.center, zero.py[16]]}>
                <ActivityIndicator />
              </View>
            )}

            <MenuLabel>{t("branding-configuration")}</MenuLabel>
            <MenuGroup>
              <MenuItem>
                <SettingsRowItem>
                  <View style={[zero.gap.all[2], { flex: 1 }]}>
                    <Text size="sm" weight="semibold">
                      {t("branding-broadcaster-did")}
                    </Text>
                    <Input
                      placeholder={t("branding-default-streamer-placeholder")}
                      value={broadcasterDID}
                      onChangeText={setBroadcasterDID}
                    />
                    <MenuInfo
                      description={t("branding-broadcaster-did-description")}
                    />
                  </View>
                </SettingsRowItem>
              </MenuItem>
            </MenuGroup>

            <MenuLabel>{t("branding-text-settings")}</MenuLabel>
            <MenuGroup>
              <MenuItem>
                <SettingsRowItem>
                  <View style={[zero.gap.all[2], { flex: 1 }]}>
                    <Text size="sm" weight="semibold">
                      {t("branding-site-title")}
                    </Text>
                    <Text size="xs" color="muted">
                      {t("branding-current", {
                        value: currentTitle?.data || "Streamplace",
                      })}
                    </Text>
                    <View
                      style={[zero.layout.flex.direction.row, zero.gap.all[2]]}
                    >
                      <View style={{ flex: 1 }}>
                        <Input
                          placeholder={t("branding-site-title-placeholder")}
                          value={siteTitle}
                          onChangeText={setSiteTitle}
                        />
                      </View>
                      <Button
                        onPress={() => uploadText("siteTitle", siteTitle)}
                        disabled={uploading || !siteTitle.trim()}
                        width="min"
                        style={{ height: 42 }}
                      >
                        {t("update")}
                      </Button>
                    </View>
                  </View>
                </SettingsRowItem>
              </MenuItem>
              <MenuSeparator />
              <MenuItem>
                <SettingsRowItem>
                  <View style={[zero.gap.all[2], { flex: 1 }]}>
                    <Text size="sm" weight="semibold">
                      {t("branding-site-description")}
                    </Text>
                    <Text size="xs" color="muted">
                      {t("branding-current", {
                        value:
                          currentDescription?.data || "Live streaming platform",
                      })}
                    </Text>
                    <View
                      style={[zero.layout.flex.direction.row, zero.gap.all[2]]}
                    >
                      <View style={{ flex: 1 }}>
                        <Input
                          placeholder={t(
                            "branding-site-description-placeholder",
                          )}
                          value={siteDescription}
                          onChangeText={setSiteDescription}
                        />
                      </View>
                      <Button
                        onPress={() =>
                          uploadText("siteDescription", siteDescription)
                        }
                        disabled={uploading || !siteDescription.trim()}
                        width="min"
                        style={{ height: 42 }}
                      >
                        {t("update")}
                      </Button>
                    </View>
                  </View>
                </SettingsRowItem>
              </MenuItem>
              <MenuSeparator />
              <MenuItem>
                <SettingsRowItem>
                  <View style={[zero.gap.all[2], { flex: 1 }]}>
                    <Text size="sm" weight="semibold">
                      {t("branding-network-name")}
                    </Text>
                    <Text size="xs" color="muted">
                      {t("branding-network-name-description", {
                        network: "Bluesky",
                      })}
                    </Text>
                    <View
                      style={[zero.layout.flex.direction.row, zero.gap.all[2]]}
                    >
                      <View style={{ flex: 1 }}>
                        <Input
                          placeholder="Bluesky"
                          value={
                            chromeInputs["networkName"] ??
                            brandingValue("networkName")
                          }
                          onChangeText={(v) =>
                            setChromeInputs((prev) => ({
                              ...prev,
                              networkName: v,
                            }))
                          }
                        />
                      </View>
                      <Button
                        onPress={() =>
                          uploadText(
                            "networkName",
                            chromeInputs["networkName"] ?? "",
                          )
                        }
                        disabled={
                          uploading ||
                          !(chromeInputs["networkName"] ?? "").trim()
                        }
                        width="min"
                        style={{ height: 42 }}
                      >
                        {t("update")}
                      </Button>
                      <Button
                        variant="secondary"
                        onPress={() => deleteBlob("networkName")}
                        disabled={uploading || !brandingValue("networkName")}
                        width="min"
                        style={{ height: 42 }}
                      >
                        {t("branding-reset")}
                      </Button>
                    </View>
                  </View>
                </SettingsRowItem>
              </MenuItem>
              <MenuSeparator />
              <MenuItem>
                <SettingsRowItem>
                  <View style={[zero.gap.all[2], { flex: 1 }]}>
                    <Text size="sm" weight="semibold">
                      {t("branding-login-placeholder")}
                    </Text>
                    <Text size="xs" color="muted">
                      {t("branding-login-placeholder-description", {
                        network: "Bluesky",
                      })}
                    </Text>
                    <View
                      style={[zero.layout.flex.direction.row, zero.gap.all[2]]}
                    >
                      <View style={{ flex: 1 }}>
                        <Input
                          placeholder="you.example.com"
                          value={
                            chromeInputs["loginPlaceholder"] ??
                            brandingValue("loginPlaceholder")
                          }
                          onChangeText={(v) =>
                            setChromeInputs((prev) => ({
                              ...prev,
                              loginPlaceholder: v,
                            }))
                          }
                        />
                      </View>
                      <Button
                        onPress={() =>
                          uploadText(
                            "loginPlaceholder",
                            chromeInputs["loginPlaceholder"] ?? "",
                          )
                        }
                        disabled={
                          uploading ||
                          !(chromeInputs["loginPlaceholder"] ?? "").trim()
                        }
                        width="min"
                        style={{ height: 42 }}
                      >
                        {t("update")}
                      </Button>
                      <Button
                        variant="secondary"
                        onPress={() => deleteBlob("loginPlaceholder")}
                        disabled={
                          uploading || !brandingValue("loginPlaceholder")
                        }
                        width="min"
                        style={{ height: 42 }}
                      >
                        {t("branding-reset")}
                      </Button>
                    </View>
                  </View>
                </SettingsRowItem>
              </MenuItem>
              <MenuSeparator />
              <MenuItem>
                <SettingsRowItem>
                  <View style={[zero.gap.all[2], { flex: 1 }]}>
                    <Text size="sm" weight="semibold">
                      {t("branding-default-streamer")}
                    </Text>
                    <Text size="xs" color="muted">
                      {t("branding-current", {
                        value:
                          currentDefaultStreamer?.data ||
                          t("branding-default-streamer-none"),
                      })}
                    </Text>
                    <View
                      style={[zero.layout.flex.direction.row, zero.gap.all[2]]}
                    >
                      <View style={{ flex: 1 }}>
                        <Input
                          placeholder={t(
                            "branding-default-streamer-placeholder",
                          )}
                          value={defaultStreamer}
                          onChangeText={setDefaultStreamer}
                        />
                      </View>
                      <Button
                        onPress={() =>
                          uploadText("defaultStreamer", defaultStreamer)
                        }
                        disabled={uploading || !defaultStreamer.trim()}
                        width="min"
                        style={{ height: 42 }}
                      >
                        {t("update")}
                      </Button>
                    </View>
                    <Button
                      variant="danger"
                      onPress={() => deleteBlob("defaultStreamer")}
                      disabled={uploading}
                    >
                      {t("branding-clear-default-streamer")}
                    </Button>
                  </View>
                </SettingsRowItem>
              </MenuItem>
            </MenuGroup>

            <MenuLabel>{t("branding-colors")}</MenuLabel>
            <MenuGroup>
              <MenuItem>
                <SettingsRowItem>
                  <View style={[zero.gap.all[2], { flex: 1 }]}>
                    <Text size="sm" weight="semibold">
                      {t("branding-primary-color")}
                    </Text>
                    <Text size="xs" color="muted">
                      {t("branding-current", {
                        value: currentPrimaryColor?.data || "#6366f1", // token-ok: branding default
                      })}
                    </Text>
                    <View
                      style={[zero.layout.flex.direction.row, zero.gap.all[2]]}
                    >
                      <View style={{ flex: 1 }}>
                        <Input
                          placeholder={t("branding-primary-color-placeholder")}
                          value={primaryColor}
                          onChangeText={setPrimaryColor}
                        />
                      </View>
                      <Button
                        onPress={() => uploadText("primaryColor", primaryColor)}
                        disabled={uploading || !primaryColor.trim()}
                        width="min"
                        style={{ height: 42 }}
                      >
                        {t("update")}
                      </Button>
                    </View>
                  </View>
                </SettingsRowItem>
              </MenuItem>
              <MenuSeparator />
              <MenuItem>
                <SettingsRowItem>
                  <View style={[zero.gap.all[2], { flex: 1 }]}>
                    <Text size="sm" weight="semibold">
                      {t("branding-accent-color")}
                    </Text>
                    <Text size="xs" color="muted">
                      {t("branding-current", {
                        value: currentAccentColor?.data || "#8b5cf6", // token-ok: branding default
                      })}
                    </Text>
                    <View
                      style={[zero.layout.flex.direction.row, zero.gap.all[2]]}
                    >
                      <View style={{ flex: 1 }}>
                        <Input
                          placeholder={t("branding-accent-color-placeholder")}
                          value={accentColor}
                          onChangeText={setAccentColor}
                        />
                      </View>
                      <Button
                        onPress={() => uploadText("accentColor", accentColor)}
                        disabled={uploading || !accentColor.trim()}
                        width="min"
                        style={{ height: 42 }}
                      >
                        {t("update")}
                      </Button>
                    </View>
                  </View>
                </SettingsRowItem>
              </MenuItem>
            </MenuGroup>

            <MenuLabel>{t("branding-status-colors")}</MenuLabel>
            <MenuGroup>
              <MenuItem>
                <SettingsRowItem>
                  <Text size="xs" color="muted">
                    {t("branding-status-colors-description")}
                  </Text>
                </SettingsRowItem>
              </MenuItem>
              <MenuSeparator />
              <MenuItem>
                <SettingsRowItem>
                  <View style={[zero.gap.all[2], { flex: 1 }]}>
                    <Text size="sm" weight="semibold">
                      {t("branding-danger-color")}
                    </Text>
                    <Text size="xs" color="muted">
                      {t("branding-current", {
                        value:
                          brandingValue("dangerColor") || t("branding-default"),
                      })}
                    </Text>
                    <View
                      style={[zero.layout.flex.direction.row, zero.gap.all[2]]}
                    >
                      <View style={{ flex: 1 }}>
                        <Input
                          placeholder="#rrggbb"
                          value={chromeInputs["dangerColor"] ?? ""}
                          onChangeText={(v) =>
                            setChromeInputs((prev) => ({
                              ...prev,
                              dangerColor: v,
                            }))
                          }
                        />
                      </View>
                      <Button
                        onPress={() =>
                          uploadText(
                            "dangerColor",
                            chromeInputs["dangerColor"] ?? "",
                          )
                        }
                        disabled={
                          uploading ||
                          !(chromeInputs["dangerColor"] ?? "").trim()
                        }
                        width="min"
                        style={{ height: 42 }}
                      >
                        {t("update")}
                      </Button>
                      <Button
                        variant="secondary"
                        onPress={() => deleteBlob("dangerColor")}
                        disabled={uploading || !brandingValue("dangerColor")}
                        width="min"
                        style={{ height: 42 }}
                      >
                        {t("branding-reset")}
                      </Button>
                    </View>
                  </View>
                </SettingsRowItem>
              </MenuItem>
              <MenuSeparator />
              <MenuItem>
                <SettingsRowItem>
                  <View style={[zero.gap.all[2], { flex: 1 }]}>
                    <Text size="sm" weight="semibold">
                      {t("branding-success-color")}
                    </Text>
                    <Text size="xs" color="muted">
                      {t("branding-current", {
                        value:
                          brandingValue("successColor") ||
                          t("branding-default"),
                      })}
                    </Text>
                    <View
                      style={[zero.layout.flex.direction.row, zero.gap.all[2]]}
                    >
                      <View style={{ flex: 1 }}>
                        <Input
                          placeholder="#rrggbb"
                          value={chromeInputs["successColor"] ?? ""}
                          onChangeText={(v) =>
                            setChromeInputs((prev) => ({
                              ...prev,
                              successColor: v,
                            }))
                          }
                        />
                      </View>
                      <Button
                        onPress={() =>
                          uploadText(
                            "successColor",
                            chromeInputs["successColor"] ?? "",
                          )
                        }
                        disabled={
                          uploading ||
                          !(chromeInputs["successColor"] ?? "").trim()
                        }
                        width="min"
                        style={{ height: 42 }}
                      >
                        {t("update")}
                      </Button>
                      <Button
                        variant="secondary"
                        onPress={() => deleteBlob("successColor")}
                        disabled={uploading || !brandingValue("successColor")}
                        width="min"
                        style={{ height: 42 }}
                      >
                        {t("branding-reset")}
                      </Button>
                    </View>
                  </View>
                </SettingsRowItem>
              </MenuItem>
              <MenuSeparator />
              <MenuItem>
                <SettingsRowItem>
                  <View style={[zero.gap.all[2], { flex: 1 }]}>
                    <Text size="sm" weight="semibold">
                      {t("branding-warning-color")}
                    </Text>
                    <Text size="xs" color="muted">
                      {t("branding-current", {
                        value:
                          brandingValue("warningColor") ||
                          t("branding-default"),
                      })}
                    </Text>
                    <View
                      style={[zero.layout.flex.direction.row, zero.gap.all[2]]}
                    >
                      <View style={{ flex: 1 }}>
                        <Input
                          placeholder="#rrggbb"
                          value={chromeInputs["warningColor"] ?? ""}
                          onChangeText={(v) =>
                            setChromeInputs((prev) => ({
                              ...prev,
                              warningColor: v,
                            }))
                          }
                        />
                      </View>
                      <Button
                        onPress={() =>
                          uploadText(
                            "warningColor",
                            chromeInputs["warningColor"] ?? "",
                          )
                        }
                        disabled={
                          uploading ||
                          !(chromeInputs["warningColor"] ?? "").trim()
                        }
                        width="min"
                        style={{ height: 42 }}
                      >
                        {t("update")}
                      </Button>
                      <Button
                        variant="secondary"
                        onPress={() => deleteBlob("warningColor")}
                        disabled={uploading || !brandingValue("warningColor")}
                        width="min"
                        style={{ height: 42 }}
                      >
                        {t("branding-reset")}
                      </Button>
                    </View>
                  </View>
                </SettingsRowItem>
              </MenuItem>
              <MenuSeparator />
              <MenuItem>
                <SettingsRowItem>
                  <View style={[zero.gap.all[2], { flex: 1 }]}>
                    <Text size="sm" weight="semibold">
                      {t("branding-info-color")}
                    </Text>
                    <Text size="xs" color="muted">
                      {t("branding-current", {
                        value:
                          brandingValue("infoColor") || t("branding-default"),
                      })}
                    </Text>
                    <View
                      style={[zero.layout.flex.direction.row, zero.gap.all[2]]}
                    >
                      <View style={{ flex: 1 }}>
                        <Input
                          placeholder="#rrggbb"
                          value={chromeInputs["infoColor"] ?? ""}
                          onChangeText={(v) =>
                            setChromeInputs((prev) => ({
                              ...prev,
                              infoColor: v,
                            }))
                          }
                        />
                      </View>
                      <Button
                        onPress={() =>
                          uploadText(
                            "infoColor",
                            chromeInputs["infoColor"] ?? "",
                          )
                        }
                        disabled={
                          uploading || !(chromeInputs["infoColor"] ?? "").trim()
                        }
                        width="min"
                        style={{ height: 42 }}
                      >
                        {t("update")}
                      </Button>
                      <Button
                        variant="secondary"
                        onPress={() => deleteBlob("infoColor")}
                        disabled={uploading || !brandingValue("infoColor")}
                        width="min"
                        style={{ height: 42 }}
                      >
                        {t("branding-reset")}
                      </Button>
                    </View>
                  </View>
                </SettingsRowItem>
              </MenuItem>
              <MenuSeparator />
              <MenuItem>
                <SettingsRowItem>
                  <View style={[zero.gap.all[2], { flex: 1 }]}>
                    <Text size="sm" weight="semibold">
                      {t("branding-live-color")}
                    </Text>
                    <Text size="xs" color="muted">
                      {t("branding-current", {
                        value:
                          brandingValue("liveColor") || t("branding-default"),
                      })}
                    </Text>
                    <View
                      style={[zero.layout.flex.direction.row, zero.gap.all[2]]}
                    >
                      <View style={{ flex: 1 }}>
                        <Input
                          placeholder="#rrggbb"
                          value={chromeInputs["liveColor"] ?? ""}
                          onChangeText={(v) =>
                            setChromeInputs((prev) => ({
                              ...prev,
                              liveColor: v,
                            }))
                          }
                        />
                      </View>
                      <Button
                        onPress={() =>
                          uploadText(
                            "liveColor",
                            chromeInputs["liveColor"] ?? "",
                          )
                        }
                        disabled={
                          uploading || !(chromeInputs["liveColor"] ?? "").trim()
                        }
                        width="min"
                        style={{ height: 42 }}
                      >
                        {t("update")}
                      </Button>
                      <Button
                        variant="secondary"
                        onPress={() => deleteBlob("liveColor")}
                        disabled={uploading || !brandingValue("liveColor")}
                        width="min"
                        style={{ height: 42 }}
                      >
                        {t("branding-reset")}
                      </Button>
                    </View>
                  </View>
                </SettingsRowItem>
              </MenuItem>
            </MenuGroup>

            <MenuLabel>{t("branding-bundle")}</MenuLabel>
            <MenuGroup>
              <MenuItem>
                <SettingsRowItem>
                  <View style={[zero.gap.all[2], { flex: 1 }]}>
                    <Text size="xs" color="muted">
                      {t("branding-bundle-description")}
                    </Text>
                    <View
                      style={[zero.layout.flex.direction.row, zero.gap.all[2]]}
                    >
                      <Button
                        onPress={exportBundle}
                        disabled={bundleBusy || Platform.OS !== "web"}
                        width="min"
                        style={{ height: 42 }}
                      >
                        {t("branding-bundle-export")}
                      </Button>
                      <Button
                        variant="secondary"
                        onPress={pickBundle}
                        disabled={bundleBusy || Platform.OS !== "web"}
                        width="min"
                        style={{ height: 42 }}
                      >
                        {t("branding-bundle-import")}
                      </Button>
                    </View>
                    <Pressable
                      onPress={() => setBundleMerge((v) => !v)}
                      style={[zero.layout.flex.direction.row, zero.gap.all[2]]}
                    >
                      <Text size="sm">
                        {bundleMerge ? "☑" : "☐"} {t("branding-bundle-merge")}
                      </Text>
                    </Pressable>
                    {bundlePreview && (
                      <View style={[zero.gap.all[2], { marginTop: 8 }]}>
                        <Text size="sm" weight="semibold">
                          {t("branding-bundle-preview", {
                            name: bundlePreview.name,
                          })}
                        </Text>
                        {bundlePreview.changes
                          .filter((c) => c.action !== "unchanged")
                          .map((c) => (
                            <Text key={c.key} size="xs">
                              {c.action === "added"
                                ? "+"
                                : c.action === "removed"
                                  ? "−"
                                  : "~"}{" "}
                              {c.key}
                              {c.detail ? `: ${c.detail}` : ""}
                            </Text>
                          ))}
                        {bundlePreview.changes.every(
                          (c) => c.action === "unchanged",
                        ) && (
                          <Text size="xs" color="muted">
                            {t("branding-bundle-no-changes")}
                          </Text>
                        )}
                        {bundlePreview.warnings.map((w) => (
                          <Text key={w} size="xs" color="muted">
                            {w}
                          </Text>
                        ))}
                        <View
                          style={[
                            zero.layout.flex.direction.row,
                            zero.gap.all[2],
                          ]}
                        >
                          <Button
                            variant="primary"
                            onPress={applyBundle}
                            disabled={bundleBusy}
                            width="min"
                            style={{ height: 42 }}
                          >
                            {t("branding-bundle-apply")}
                          </Button>
                          <Button
                            variant="secondary"
                            onPress={() => setBundlePreview(null)}
                            disabled={bundleBusy}
                            width="min"
                            style={{ height: 42 }}
                          >
                            {t("cancel")}
                          </Button>
                        </View>
                      </View>
                    )}
                  </View>
                </SettingsRowItem>
              </MenuItem>
            </MenuGroup>

            <MenuLabel>{t("branding-layout")}</MenuLabel>
            <MenuGroup>
              <MenuItem>
                <SettingsRowItem>
                  <View style={[zero.gap.all[2], { flex: 1 }]}>
                    <Text size="sm" weight="semibold">
                      {t("branding-app-layout")}
                    </Text>
                    <Text size="xs" color="muted">
                      {t("branding-app-layout-description")}
                    </Text>
                    <SegmentedTabs
                      options={[
                        {
                          value: "classic",
                          label: t("branding-layout-classic"),
                        },
                        {
                          value: "social",
                          label: t("branding-app-layout-social"),
                        },
                      ]}
                      value={brandingValue("appLayout") || "classic"}
                      onChange={(v) => uploadText("appLayout", v)}
                    />
                  </View>
                </SettingsRowItem>
              </MenuItem>
              <MenuSeparator />
              <MenuItem>
                <SettingsRowItem>
                  <View style={[zero.gap.all[2], { flex: 1 }]}>
                    <Text size="sm" weight="semibold">
                      {t("branding-stream-layout")}
                    </Text>
                    <Text size="xs" color="muted">
                      {t("branding-stream-layout-description")}
                    </Text>
                    <SegmentedTabs
                      size="sm"
                      options={[
                        {
                          value: "classic",
                          label: t("branding-layout-classic"),
                        },
                        { value: "card", label: t("branding-layout-card") },
                      ]}
                      value={brandingValue("streamLayout") || "classic"}
                      onChange={(v) => uploadText("streamLayout", v)}
                    />
                  </View>
                </SettingsRowItem>
              </MenuItem>
              <MenuSeparator />
              <MenuItem>
                <SettingsRowItem>
                  <View style={[zero.gap.all[2], { flex: 1 }]}>
                    <Text size="sm" weight="semibold">
                      {t("branding-typeface")}
                    </Text>
                    <Text size="xs" color="muted">
                      {t("branding-typeface-description")}
                    </Text>
                    <SegmentedTabs
                      size="sm"
                      options={[
                        { value: "geist", label: "Geist" },
                        { value: "inter", label: "Inter" },
                      ]}
                      value={brandingValue("typeface") || "geist"}
                      onChange={(v) => uploadText("typeface", v)}
                    />
                  </View>
                </SettingsRowItem>
              </MenuItem>
              <MenuSeparator />
              <MenuItem>
                <SettingsRowItem>
                  <View style={[zero.gap.all[2], { flex: 1 }]}>
                    <Text size="sm" weight="semibold">
                      {t("branding-chat-layout")}
                    </Text>
                    <Text size="xs" color="muted">
                      {t("branding-chat-layout-description")}
                    </Text>
                    <SegmentedTabs
                      size="sm"
                      options={[
                        { value: "compact", label: t("branding-chat-compact") },
                        { value: "avatar", label: t("branding-chat-avatar") },
                      ]}
                      value={brandingValue("chatLayout") || "compact"}
                      onChange={(v) => uploadText("chatLayout", v)}
                    />
                  </View>
                </SettingsRowItem>
              </MenuItem>
              <MenuSeparator />
              <MenuItem>
                <SettingsRowItem>
                  <View style={[zero.gap.all[2], { flex: 1 }]}>
                    <Text size="sm" weight="semibold">
                      {t("branding-nav-links")}
                    </Text>
                    <Text size="xs" color="muted">
                      {t("branding-nav-links-description")}
                    </Text>
                    <TextInput
                      multiline
                      numberOfLines={6}
                      placeholderTextColor={theme.colors.text3}
                      style={{
                        minHeight: 120,
                        padding: 12,
                        borderRadius: 10,
                        borderWidth: 1,
                        borderColor: theme.colors.border,
                        backgroundColor: theme.colors.surface1,
                        color: theme.colors.text1,
                        fontFamily: theme.fonts.monoRegular,
                        fontSize: 12,
                        textAlignVertical: "top",
                      }}
                      placeholder={NAV_LINKS_EXAMPLE}
                      value={
                        chromeInputs["navLinks"] ?? brandingValue("navLinks")
                      }
                      onChangeText={(v) =>
                        setChromeInputs((prev) => ({ ...prev, navLinks: v }))
                      }
                    />
                    <View
                      style={[zero.layout.flex.direction.row, zero.gap.all[2]]}
                    >
                      <Button
                        onPress={() =>
                          uploadText("navLinks", chromeInputs["navLinks"] ?? "")
                        }
                        disabled={
                          uploading || !(chromeInputs["navLinks"] ?? "").trim()
                        }
                        width="min"
                        style={{ height: 42 }}
                      >
                        {t("update")}
                      </Button>
                      <Button
                        variant="secondary"
                        onPress={() => deleteBlob("navLinks")}
                        disabled={uploading || !brandingValue("navLinks")}
                        width="min"
                        style={{ height: 42 }}
                      >
                        {t("branding-reset")}
                      </Button>
                    </View>
                  </View>
                </SettingsRowItem>
              </MenuItem>
              <MenuSeparator />
              <MenuItem>
                <SettingsRowItem>
                  <View style={[zero.gap.all[2], { flex: 1 }]}>
                    <Text size="sm" weight="semibold">
                      {t("branding-nav-cta")}
                    </Text>
                    <Text size="xs" color="muted">
                      {t("branding-nav-cta-description")}
                    </Text>
                    <Input
                      placeholder={NAV_CTA_EXAMPLE}
                      value={chromeInputs["navCta"] ?? brandingValue("navCta")}
                      onChangeText={(v) =>
                        setChromeInputs((prev) => ({ ...prev, navCta: v }))
                      }
                    />
                    <View
                      style={[zero.layout.flex.direction.row, zero.gap.all[2]]}
                    >
                      <Button
                        onPress={() =>
                          uploadText("navCta", chromeInputs["navCta"] ?? "")
                        }
                        disabled={
                          uploading || !(chromeInputs["navCta"] ?? "").trim()
                        }
                        width="min"
                        style={{ height: 42 }}
                      >
                        {t("update")}
                      </Button>
                      <Button
                        variant="secondary"
                        onPress={() => deleteBlob("navCta")}
                        disabled={uploading || !brandingValue("navCta")}
                        width="min"
                        style={{ height: 42 }}
                      >
                        {t("branding-reset")}
                      </Button>
                    </View>
                  </View>
                </SettingsRowItem>
              </MenuItem>
              <MenuSeparator />
              <MenuItem>
                <SettingsRowItem>
                  <View style={[zero.gap.all[2], { flex: 1 }]}>
                    <Text size="sm" weight="semibold">
                      {t("branding-download-link")}
                    </Text>
                    <Text size="xs" color="muted">
                      {t("branding-download-link-description")}
                    </Text>
                    <SegmentedTabs
                      options={[
                        { value: "", label: t("branding-download-default") },
                        { value: "on", label: t("branding-download-on") },
                        { value: "off", label: t("branding-download-off") },
                      ]}
                      value={brandingValue("showDownloadLink") || ""}
                      onChange={(v) =>
                        v
                          ? uploadText("showDownloadLink", v)
                          : deleteBlob("showDownloadLink")
                      }
                    />
                  </View>
                </SettingsRowItem>
              </MenuItem>
              <MenuSeparator />
              <MenuItem>
                <SettingsRowItem>
                  <View style={[zero.gap.all[2], { flex: 1 }]}>
                    <Text size="sm" weight="semibold">
                      {t("branding-bottom-links")}
                    </Text>
                    <Text size="xs" color="muted">
                      {t("branding-bottom-links-description")}
                    </Text>
                    <TextInput
                      multiline
                      numberOfLines={3}
                      placeholderTextColor={theme.colors.text3}
                      style={{
                        minHeight: 70,
                        padding: 12,
                        borderRadius: 10,
                        borderWidth: 1,
                        borderColor: theme.colors.border,
                        backgroundColor: theme.colors.surface1,
                        color: theme.colors.text1,
                        fontFamily: theme.fonts.monoRegular,
                        fontSize: 12,
                        textAlignVertical: "top",
                      }}
                      placeholder={BOTTOM_LINKS_EXAMPLE}
                      value={
                        chromeInputs["bottomLinks"] ??
                        brandingValue("bottomLinks")
                      }
                      onChangeText={(v) =>
                        setChromeInputs((prev) => ({ ...prev, bottomLinks: v }))
                      }
                    />
                    <View
                      style={[zero.layout.flex.direction.row, zero.gap.all[2]]}
                    >
                      <Button
                        onPress={() =>
                          uploadText(
                            "bottomLinks",
                            chromeInputs["bottomLinks"] ?? "",
                          )
                        }
                        disabled={
                          uploading ||
                          !(chromeInputs["bottomLinks"] ?? "").trim()
                        }
                        width="min"
                        style={{ height: 42 }}
                      >
                        {t("update")}
                      </Button>
                      <Button
                        variant="secondary"
                        onPress={() => uploadText("bottomLinks", "[]")}
                        disabled={uploading}
                        width="min"
                        style={{ height: 42 }}
                      >
                        {t("branding-social-hide")}
                      </Button>
                      <Button
                        variant="secondary"
                        onPress={() => deleteBlob("bottomLinks")}
                        disabled={uploading || !brandingValue("bottomLinks")}
                        width="min"
                        style={{ height: 42 }}
                      >
                        {t("branding-reset")}
                      </Button>
                    </View>
                  </View>
                </SettingsRowItem>
              </MenuItem>
              <MenuSeparator />
              <MenuItem>
                <SettingsRowItem>
                  <View style={[zero.gap.all[2], { flex: 1 }]}>
                    <Text size="sm" weight="semibold">
                      {t("branding-social-links")}
                    </Text>
                    <Text size="xs" color="muted">
                      {t("branding-social-links-description")}
                    </Text>
                    <Input
                      placeholder={t("branding-social-heading-placeholder")}
                      value={
                        chromeInputs["socialHeading"] ??
                        brandingValue("socialHeading")
                      }
                      onChangeText={(v) =>
                        setChromeInputs((prev) => ({
                          ...prev,
                          socialHeading: v,
                        }))
                      }
                    />
                    <TextInput
                      multiline
                      numberOfLines={4}
                      placeholderTextColor={theme.colors.text3}
                      style={{
                        minHeight: 90,
                        padding: 12,
                        borderRadius: 10,
                        borderWidth: 1,
                        borderColor: theme.colors.border,
                        backgroundColor: theme.colors.surface1,
                        color: theme.colors.text1,
                        fontFamily: theme.fonts.monoRegular,
                        fontSize: 12,
                        textAlignVertical: "top",
                      }}
                      placeholder={SOCIAL_LINKS_EXAMPLE}
                      value={
                        chromeInputs["socialLinks"] ??
                        brandingValue("socialLinks")
                      }
                      onChangeText={(v) =>
                        setChromeInputs((prev) => ({
                          ...prev,
                          socialLinks: v,
                        }))
                      }
                    />
                    <View
                      style={[zero.layout.flex.direction.row, zero.gap.all[2]]}
                    >
                      <Button
                        onPress={() => {
                          const heading = chromeInputs["socialHeading"];
                          const links = chromeInputs["socialLinks"];
                          if (heading !== undefined) {
                            uploadText("socialHeading", heading);
                          }
                          if (links !== undefined) {
                            uploadText("socialLinks", links);
                          }
                        }}
                        disabled={
                          uploading ||
                          (chromeInputs["socialHeading"] === undefined &&
                            chromeInputs["socialLinks"] === undefined)
                        }
                        width="min"
                        style={{ height: 42 }}
                      >
                        {t("update")}
                      </Button>
                      <Button
                        variant="secondary"
                        onPress={() => uploadText("socialLinks", "[]")}
                        disabled={uploading}
                        width="min"
                        style={{ height: 42 }}
                      >
                        {t("branding-social-hide")}
                      </Button>
                      <Button
                        variant="secondary"
                        onPress={() => {
                          deleteBlob("socialHeading");
                          deleteBlob("socialLinks");
                        }}
                        disabled={
                          uploading ||
                          (!brandingValue("socialLinks") &&
                            !brandingValue("socialHeading"))
                        }
                        width="min"
                        style={{ height: 42 }}
                      >
                        {t("branding-reset")}
                      </Button>
                    </View>
                    <Text size="xs" color="muted">
                      {t("branding-social-icons-description")}
                    </Text>
                    <View
                      style={[
                        zero.layout.flex.direction.row,
                        zero.gap.all[3],
                        { flexWrap: "wrap" },
                      ]}
                    >
                      {SOCIAL_ICON_SLOTS.map((slot) => (
                        <View
                          key={slot}
                          style={[
                            zero.layout.flex.direction.row,
                            zero.gap.all[2],
                            { alignItems: "center" },
                          ]}
                        >
                          <View
                            style={{
                              width: 32,
                              height: 32,
                              borderRadius: 8,
                              borderWidth: 1,
                              borderColor: theme.colors.border,
                              alignItems: "center",
                              justifyContent: "center",
                            }}
                          >
                            {branding?.[slot]?.data ? (
                              <Image
                                source={{ uri: branding[slot].data }}
                                contentFit="contain"
                                style={{ width: 20, height: 20 }}
                              />
                            ) : (
                              <Text size="xs" color="muted">
                                {slot.slice(-1)}
                              </Text>
                            )}
                          </View>
                          <Text
                            size="xs"
                            style={{ fontFamily: theme.fonts.monoRegular }}
                          >
                            {slot}
                          </Text>
                          <Button
                            variant="secondary"
                            onPress={() =>
                              handleFileSelect(
                                slot,
                                "image/svg+xml,image/png,image/webp",
                              )
                            }
                            disabled={uploading || Platform.OS !== "web"}
                            width="min"
                            style={{ height: 32 }}
                          >
                            {t("branding-upload")}
                          </Button>
                          {!!branding?.[slot]?.data && (
                            <Button
                              variant="danger"
                              onPress={() => deleteBlob(slot)}
                              disabled={uploading}
                              width="min"
                              style={{ height: 32 }}
                            >
                              {t("branding-remove")}
                            </Button>
                          )}
                        </View>
                      ))}
                    </View>
                  </View>
                </SettingsRowItem>
              </MenuItem>
            </MenuGroup>

            <MenuLabel>{t("branding-theme")}</MenuLabel>
            <MenuGroup>
              <MenuItem>
                <SettingsRowItem>
                  <Text size="xs" color="muted">
                    {t("branding-theme-description")}
                  </Text>
                </SettingsRowItem>
              </MenuItem>
              <MenuSeparator />
              <MenuItem>
                <SettingsRowItem>
                  <View style={[zero.gap.all[2], { flex: 1 }]}>
                    <Text size="sm" weight="semibold">
                      {t("branding-background-dark")}
                    </Text>
                    <Text size="xs" color="muted">
                      {t("branding-current", {
                        value:
                          currentBgDark?.data || DEFAULT_CHROME.dark.background,
                      })}
                    </Text>
                    <View
                      style={[zero.layout.flex.direction.row, zero.gap.all[2]]}
                    >
                      <View style={{ flex: 1 }}>
                        <Input
                          placeholder={DEFAULT_CHROME.dark.background}
                          value={chromeInputs["backgroundColor"] ?? ""}
                          onChangeText={(v) =>
                            setChromeInputs((prev) => ({
                              ...prev,
                              backgroundColor: v,
                            }))
                          }
                        />
                      </View>
                      <Button
                        onPress={() =>
                          uploadText(
                            "backgroundColor",
                            chromeInputs["backgroundColor"] ?? "",
                          )
                        }
                        disabled={
                          uploading ||
                          !(chromeInputs["backgroundColor"] ?? "").trim()
                        }
                        width="min"
                        style={{ height: 42 }}
                      >
                        {t("update")}
                      </Button>
                      <Button
                        variant="secondary"
                        onPress={() => deleteBlob("backgroundColor")}
                        disabled={uploading || !currentBgDark?.data}
                        width="min"
                        style={{ height: 42 }}
                      >
                        {t("branding-reset")}
                      </Button>
                    </View>
                  </View>
                </SettingsRowItem>
              </MenuItem>
              <MenuSeparator />
              <MenuItem>
                <SettingsRowItem>
                  <View style={[zero.gap.all[2], { flex: 1 }]}>
                    <Text size="sm" weight="semibold">
                      {t("branding-foreground-dark")}
                    </Text>
                    <Text size="xs" color="muted">
                      {t("branding-current", {
                        value:
                          currentFgDark?.data || DEFAULT_CHROME.dark.foreground,
                      })}
                    </Text>
                    <View
                      style={[zero.layout.flex.direction.row, zero.gap.all[2]]}
                    >
                      <View style={{ flex: 1 }}>
                        <Input
                          placeholder={DEFAULT_CHROME.dark.foreground}
                          value={chromeInputs["foregroundColor"] ?? ""}
                          onChangeText={(v) =>
                            setChromeInputs((prev) => ({
                              ...prev,
                              foregroundColor: v,
                            }))
                          }
                        />
                      </View>
                      <Button
                        onPress={() =>
                          uploadText(
                            "foregroundColor",
                            chromeInputs["foregroundColor"] ?? "",
                          )
                        }
                        disabled={
                          uploading ||
                          !(chromeInputs["foregroundColor"] ?? "").trim()
                        }
                        width="min"
                        style={{ height: 42 }}
                      >
                        {t("update")}
                      </Button>
                      <Button
                        variant="secondary"
                        onPress={() => deleteBlob("foregroundColor")}
                        disabled={uploading || !currentFgDark?.data}
                        width="min"
                        style={{ height: 42 }}
                      >
                        {t("branding-reset")}
                      </Button>
                    </View>
                  </View>
                </SettingsRowItem>
              </MenuItem>
              <MenuSeparator />
              <MenuItem>
                <SettingsRowItem>
                  <View style={[zero.gap.all[2], { flex: 1 }]}>
                    <Text size="sm" weight="semibold">
                      {t("branding-background-light")}
                    </Text>
                    <Text size="xs" color="muted">
                      {t("branding-current", {
                        value:
                          currentBgLight?.data ||
                          DEFAULT_CHROME.light.background,
                      })}
                    </Text>
                    <View
                      style={[zero.layout.flex.direction.row, zero.gap.all[2]]}
                    >
                      <View style={{ flex: 1 }}>
                        <Input
                          placeholder={DEFAULT_CHROME.light.background}
                          value={chromeInputs["backgroundColorLight"] ?? ""}
                          onChangeText={(v) =>
                            setChromeInputs((prev) => ({
                              ...prev,
                              backgroundColorLight: v,
                            }))
                          }
                        />
                      </View>
                      <Button
                        onPress={() =>
                          uploadText(
                            "backgroundColorLight",
                            chromeInputs["backgroundColorLight"] ?? "",
                          )
                        }
                        disabled={
                          uploading ||
                          !(chromeInputs["backgroundColorLight"] ?? "").trim()
                        }
                        width="min"
                        style={{ height: 42 }}
                      >
                        {t("update")}
                      </Button>
                      <Button
                        variant="secondary"
                        onPress={() => deleteBlob("backgroundColorLight")}
                        disabled={uploading || !currentBgLight?.data}
                        width="min"
                        style={{ height: 42 }}
                      >
                        {t("branding-reset")}
                      </Button>
                    </View>
                  </View>
                </SettingsRowItem>
              </MenuItem>
              <MenuSeparator />
              <MenuItem>
                <SettingsRowItem>
                  <View style={[zero.gap.all[2], { flex: 1 }]}>
                    <Text size="sm" weight="semibold">
                      {t("branding-foreground-light")}
                    </Text>
                    <Text size="xs" color="muted">
                      {t("branding-current", {
                        value:
                          currentFgLight?.data ||
                          DEFAULT_CHROME.light.foreground,
                      })}
                    </Text>
                    <View
                      style={[zero.layout.flex.direction.row, zero.gap.all[2]]}
                    >
                      <View style={{ flex: 1 }}>
                        <Input
                          placeholder={DEFAULT_CHROME.light.foreground}
                          value={chromeInputs["foregroundColorLight"] ?? ""}
                          onChangeText={(v) =>
                            setChromeInputs((prev) => ({
                              ...prev,
                              foregroundColorLight: v,
                            }))
                          }
                        />
                      </View>
                      <Button
                        onPress={() =>
                          uploadText(
                            "foregroundColorLight",
                            chromeInputs["foregroundColorLight"] ?? "",
                          )
                        }
                        disabled={
                          uploading ||
                          !(chromeInputs["foregroundColorLight"] ?? "").trim()
                        }
                        width="min"
                        style={{ height: 42 }}
                      >
                        {t("update")}
                      </Button>
                      <Button
                        variant="secondary"
                        onPress={() => deleteBlob("foregroundColorLight")}
                        disabled={uploading || !currentFgLight?.data}
                        width="min"
                        style={{ height: 42 }}
                      >
                        {t("branding-reset")}
                      </Button>
                    </View>
                  </View>
                </SettingsRowItem>
              </MenuItem>
            </MenuGroup>

            <MenuLabel>{t("branding-legal-links")}</MenuLabel>
            <MenuGroup>
              <MenuItem>
                <SettingsRowItem>
                  <View style={[zero.gap.all[2], { flex: 1 }]}>
                    <Text size="sm" weight="semibold">
                      {editingLinkIndex !== null
                        ? t("branding-edit-legal-link")
                        : t("branding-add-legal-link")}
                    </Text>
                    <Input
                      placeholder={t("branding-legal-link-text-placeholder")}
                      value={legalLinkText}
                      onChangeText={setLegalLinkText}
                    />
                    <Input
                      placeholder={t("branding-legal-link-url-placeholder")}
                      value={legalLinkUrl}
                      onChangeText={setLegalLinkUrl}
                    />
                    <View
                      style={[zero.layout.flex.direction.row, zero.gap.all[2]]}
                    >
                      <Button
                        onPress={saveLegalLink}
                        disabled={
                          uploading ||
                          !legalLinkText.trim() ||
                          !legalLinkUrl.trim()
                        }
                        width="min"
                        style={{ height: 42 }}
                      >
                        {editingLinkIndex !== null ? t("update") : t("add")}
                      </Button>
                      {editingLinkIndex !== null && (
                        <Button
                          variant="secondary"
                          onPress={cancelEditingLink}
                          disabled={uploading}
                          width="min"
                          style={{ height: 42 }}
                        >
                          {t("cancel")}
                        </Button>
                      )}
                    </View>
                  </View>
                </SettingsRowItem>
              </MenuItem>
              {legalLinks.length > 0 && (
                <>
                  <MenuSeparator />
                  {legalLinks.map((link, index) => (
                    <View key={index}>
                      <MenuItem>
                        <SettingsRowItem>
                          <View style={[zero.gap.all[2], { flex: 1 }]}>
                            <Text size="sm" weight="semibold">
                              {link.text}
                            </Text>
                            <Text size="xs" color="muted">
                              {link.url}
                            </Text>
                            <View
                              style={[
                                zero.layout.flex.direction.row,
                                zero.gap.all[2],
                              ]}
                            >
                              <Button
                                variant="secondary"
                                onPress={() => startEditingLink(index)}
                                disabled={uploading}
                                width="min"
                                style={{ height: 42 }}
                              >
                                {t("edit")}
                              </Button>
                              <Button
                                variant="danger"
                                onPress={() => deleteLegalLink(index)}
                                disabled={uploading}
                                width="min"
                                style={{ height: 42 }}
                              >
                                {t("delete")}
                              </Button>
                            </View>
                          </View>
                        </SettingsRowItem>
                      </MenuItem>
                      {index < legalLinks.length - 1 && <MenuSeparator />}
                    </View>
                  ))}
                </>
              )}
            </MenuGroup>

            <MenuLabel>{t("branding-images")}</MenuLabel>
            <MenuGroup>
              <MenuItem>
                <SettingsRowItem>
                  <View style={[zero.gap.all[2], { flex: 1 }]}>
                    <Text size="sm" weight="semibold">
                      {t("branding-main-logo")}
                    </Text>
                    <MenuInfo
                      description={t("branding-main-logo-description")}
                    />
                    {currentLogo?.data && (
                      <Image
                        source={{ uri: currentLogo.data }}
                        contentFit="contain"
                        style={{
                          width: 200,
                          height: 100,
                        }}
                      />
                    )}
                    <View
                      style={[zero.layout.flex.direction.row, zero.gap.all[2]]}
                    >
                      <Button
                        onPress={() =>
                          handleFileSelect(
                            "mainLogo",
                            "image/svg+xml,image/png,image/jpeg,image/webp",
                          )
                        }
                        disabled={uploading || Platform.OS !== "web"}
                        width="min"
                        style={{ height: 42 }}
                      >
                        {t("branding-upload-logo")}
                      </Button>
                      <Button
                        variant="danger"
                        onPress={() => deleteBlob("mainLogo")}
                        disabled={uploading}
                        width="min"
                        style={{ height: 42 }}
                      >
                        {t("branding-delete-logo")}
                      </Button>
                    </View>
                  </View>
                </SettingsRowItem>
              </MenuItem>
              <MenuSeparator />
              <MenuItem>
                <SettingsRowItem>
                  <View style={[zero.gap.all[2], { flex: 1 }]}>
                    <Text size="sm" weight="semibold">
                      {t("branding-favicon")}
                    </Text>
                    <MenuInfo description={t("branding-favicon-description")} />
                    {currentFavicon?.data && (
                      <Image
                        source={{ uri: currentFavicon.data }}
                        contentFit="contain"
                        style={{ width: 64, height: 64 }}
                      />
                    )}
                    <View
                      style={[zero.layout.flex.direction.row, zero.gap.all[2]]}
                    >
                      <Button
                        onPress={() =>
                          handleFileSelect(
                            "favicon",
                            "image/svg+xml,image/png,image/x-icon",
                          )
                        }
                        disabled={uploading || Platform.OS !== "web"}
                        width="min"
                        style={{ height: 42 }}
                      >
                        {t("branding-upload-favicon")}
                      </Button>
                      <Button
                        variant="danger"
                        onPress={() => deleteBlob("favicon")}
                        disabled={uploading}
                        width="min"
                        style={{ height: 42 }}
                      >
                        {t("branding-delete-favicon")}
                      </Button>
                    </View>
                  </View>
                </SettingsRowItem>
              </MenuItem>
              <MenuSeparator />
              <MenuItem>
                <View style={[zero.gap.all[2], { flex: 1 }]}>
                  <Text size="sm" weight="semibold">
                    {t("branding-sidebar-bg")}
                  </Text>
                  <MenuInfo
                    description={t("branding-sidebar-bg-description")}
                  />
                  {currentSidebarBg?.data && (
                    <>
                      <Image
                        source={{ uri: currentSidebarBg.data }}
                        contentFit="contain"
                        style={{
                          width: 200,
                          height: 200,
                        }}
                      />
                      <Text size="xs" color="muted">
                        {currentSidebarBg?.height || "unknown"} x{" "}
                        {currentSidebarBg?.width || "unknown"}
                      </Text>
                    </>
                  )}
                  <View
                    style={[zero.layout.flex.direction.row, zero.gap.all[2]]}
                  >
                    <Button
                      onPress={() =>
                        handleFileSelect(
                          "sidebarBackgroundImage",
                          "image/svg+xml,image/png,image/jpeg,image/webp",
                        )
                      }
                      disabled={uploading || Platform.OS !== "web"}
                      width="min"
                      style={{ height: 42 }}
                    >
                      {t("branding-upload-background")}
                    </Button>
                    <Button
                      variant="danger"
                      onPress={() => deleteBlob("sidebarBackgroundImage")}
                      disabled={uploading}
                      width="min"
                      style={{ height: 42 }}
                    >
                      {t("branding-delete-background")}
                    </Button>
                  </View>
                </View>
              </MenuItem>
              <MenuSeparator />
              <MenuItem>
                <View style={[zero.gap.all[2], { flex: 1 }]}>
                  <Text size="sm" weight="semibold">
                    {t("branding-link-banner")}
                  </Text>
                  <MenuInfo
                    description={t("branding-link-banner-description")}
                  />
                  {currentLinkBanner?.data && (
                    <>
                      <Image
                        source={{ uri: currentLinkBanner.data }}
                        contentFit="contain"
                        style={{
                          width: 300,
                          height: 158,
                        }}
                      />
                      <Text size="xs" color="muted">
                        {currentLinkBanner?.width || "unknown"} x{" "}
                        {currentLinkBanner?.height || "unknown"}
                      </Text>
                    </>
                  )}
                  <View
                    style={[zero.layout.flex.direction.row, zero.gap.all[2]]}
                  >
                    <Button
                      onPress={() =>
                        handleFileSelect(
                          "linkBanner",
                          "image/png,image/jpeg,image/webp",
                        )
                      }
                      disabled={uploading || Platform.OS !== "web"}
                      width="min"
                      style={{ height: 42 }}
                    >
                      {t("branding-upload-link-banner")}
                    </Button>
                    <Button
                      variant="danger"
                      onPress={() => deleteBlob("linkBanner")}
                      disabled={uploading || !currentLinkBanner?.data}
                      width="min"
                      style={{ height: 42 }}
                    >
                      {t("branding-delete-link-banner")}
                    </Button>
                  </View>
                </View>
              </MenuItem>
              <MenuSeparator />
              {Platform.OS !== "web" && (
                <MenuItem>
                  <SettingsRowItem>
                    <Text size="sm" color="muted">
                      {t("branding-web-only")}
                    </Text>
                  </SettingsRowItem>
                </MenuItem>
              )}
            </MenuGroup>
          </MenuContainer>
        </View>
      </View>
    </ScrollView>
  );
}
