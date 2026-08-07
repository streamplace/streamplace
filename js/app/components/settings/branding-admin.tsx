import {
  Button,
  Input,
  MenuContainer,
  MenuGroup,
  MenuInfo,
  MenuItem,
  MenuLabel,
  MenuSeparator,
  Text,
  useStreamplaceStore,
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
import { ActivityIndicator, Platform, ScrollView } from "react-native";
import { place } from "streamplace";
import { SettingsRowItem } from "./components/settings-navigation-item";

export function BrandingAdmin() {
  const { t } = useTranslation("settings");
  const agent = usePDSAgent();
  const fetchBranding = useFetchBranding();
  const toast = useToast();
  const currentBroadcasterDID = useStreamplaceStore((s) => s.broadcasterDID);

  // state for form inputs
  const [siteTitle, setSiteTitle] = useState("");
  const [siteDescription, setSiteDescription] = useState("");
  const [primaryColor, setPrimaryColor] = useState("");
  const [accentColor, setAccentColor] = useState("");
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
  const currentDefaultStreamer = useBrandingAsset("defaultStreamer");
  const currentLogo = useBrandingAsset("mainLogo");
  const currentFavicon = useBrandingAsset("favicon");
  const currentSidebarBg = useSidebarBackgroundImage();
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
      const base64Data = btoa(String.fromCharCode(...textBytes));

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
      const base64Data = btoa(String.fromCharCode(...uint8Array));

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
                        value: currentPrimaryColor?.data || "#6366f1",
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
                        value: currentAccentColor?.data || "#8b5cf6",
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
