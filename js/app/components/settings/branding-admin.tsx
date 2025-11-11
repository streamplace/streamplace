import {
  Button,
  Input,
  Text,
  useToast,
  View,
  zero,
} from "@streamplace/components";
import {
  useBrandingAsset,
  useFetchBranding,
} from "@streamplace/components/src/streamplace-store/branding";
import { usePDSAgent } from "@streamplace/components/src/streamplace-store/xrpc";
import { useEffect, useState } from "react";
import { ActivityIndicator, Image, Platform, ScrollView } from "react-native";

export function BrandingAdmin() {
  const agent = usePDSAgent();
  const fetchBranding = useFetchBranding();
  const toast = useToast();

  // state for form inputs
  const [siteTitle, setSiteTitle] = useState("");
  const [siteDescription, setSiteDescription] = useState("");
  const [primaryColor, setPrimaryColor] = useState("");
  const [accentColor, setAccentColor] = useState("");
  const [defaultStreamer, setDefaultStreamer] = useState("");
  const [broadcasterDID, setBroadcasterDID] = useState("");
  const [uploading, setUploading] = useState(false);

  // get current values
  const currentTitle = useBrandingAsset("siteTitle");
  const currentDescription = useBrandingAsset("siteDescription");
  const currentPrimaryColor = useBrandingAsset("primaryColor");
  const currentAccentColor = useBrandingAsset("accentColor");
  const currentDefaultStreamer = useBrandingAsset("defaultStreamer");
  const currentLogo = useBrandingAsset("mainLogo");
  const currentFavicon = useBrandingAsset("favicon");

  // load current branding on mount
  useEffect(() => {
    fetchBranding();
  }, []);

  const uploadText = async (key: string, value: string) => {
    if (!agent) {
      toast.show("Not authenticated", "Please log in first", {
        variant: "error",
      });
      return;
    }

    if (!value.trim()) {
      toast.show("Empty value", "Please enter a value", { variant: "error" });
      return;
    }

    setUploading(true);
    try {
      const textBytes = new TextEncoder().encode(value.trim());
      const base64Data = btoa(String.fromCharCode(...textBytes));

      await agent.place.stream.branding.updateBlob({
        key,
        broadcaster: broadcasterDID || undefined,
        data: base64Data,
        mimeType: "text/plain",
      });

      toast.show("Success", `${key} updated successfully`, {
        variant: "success",
      });

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
      setTimeout(() => fetchBranding(), 500);
    } catch (err: any) {
      toast.show("Upload failed", err.message || "Failed to upload", {
        variant: "error",
      });
    } finally {
      setUploading(false);
    }
  };

  const uploadFile = async (key: string, file: File) => {
    if (!agent) {
      toast.show("Not authenticated", "Please log in first", {
        variant: "error",
      });
      return;
    }

    setUploading(true);
    try {
      const arrayBuffer = await file.arrayBuffer();
      const uint8Array = new Uint8Array(arrayBuffer);
      const base64Data = btoa(String.fromCharCode(...uint8Array));

      await agent.place.stream.branding.updateBlob({
        key,
        broadcaster: broadcasterDID || undefined,
        data: base64Data,
        mimeType: file.type,
      });

      toast.show("Success", `${key} uploaded successfully`, {
        variant: "success",
      });

      // reload branding
      setTimeout(() => fetchBranding(), 500);
    } catch (err: any) {
      toast.show("Upload failed", err.message || "Failed to upload", {
        variant: "error",
      });
    } finally {
      setUploading(false);
    }
  };

  const handleFileSelect = (key: string, accept: string) => {
    if (Platform.OS !== "web") {
      toast.show("Not available", "File uploads are only available on web", {
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
      toast.show("Not authenticated", "Please log in first", {
        variant: "error",
      });
      return;
    }

    setUploading(true);
    try {
      await agent.place.stream.branding.deleteBlob({
        key,
        broadcaster: broadcasterDID || undefined,
      });

      toast.show("Success", `${key} deleted successfully`, {
        variant: "success",
      });

      // reload branding
      setTimeout(() => fetchBranding(), 500);
    } catch (err: any) {
      toast.show("Delete failed", err.message || "Failed to delete", {
        variant: "error",
      });
    } finally {
      setUploading(false);
    }
  };

  if (!agent) {
    return (
      <View style={[zero.layout.flex.align.center, zero.px[16], zero.py[24]]}>
        <Text>Please log in to manage branding</Text>
      </View>
    );
  }

  return (
    <ScrollView>
      <View style={[zero.layout.flex.align.center, zero.px[16], zero.py[24]]}>
        <View
          style={[
            zero.gap.all[12],
            { paddingVertical: 24, maxWidth: 600, width: "100%" },
          ]}
        >
          <View>
            <Text size="2xl" weight="bold">
              Branding Administration
            </Text>
            <Text color="muted">Customize your Streamplace instance</Text>
          </View>

          {uploading && (
            <View style={[zero.layout.flex.align.center, zero.py[16]]}>
              <ActivityIndicator />
            </View>
          )}

          {/* Broadcaster DID */}
          <View style={[zero.gap.all[8]]}>
            <Text size="lg" weight="semibold">
              Broadcaster DID
            </Text>
            <Text size="sm" color="muted">
              Leave empty to use server default
            </Text>
            <Input
              placeholder="did:plc:..."
              value={broadcasterDID}
              onChangeText={setBroadcasterDID}
            />
          </View>

          {/* Site Title */}
          <View style={[zero.gap.all[8]]}>
            <Text size="lg" weight="semibold">
              Site Title
            </Text>
            <Text size="sm" color="muted">
              Current: {currentTitle?.data || "Streamplace"}
            </Text>
            <View style={[zero.layout.flex.direction.row, zero.gap.all[8]]}>
              <View style={{ flex: 1 }}>
                <Input
                  placeholder="Enter new site title"
                  value={siteTitle}
                  onChangeText={setSiteTitle}
                />
              </View>
              <Button
                onPress={() => uploadText("siteTitle", siteTitle)}
                disabled={uploading || !siteTitle.trim()}
              >
                Update
              </Button>
            </View>
          </View>

          {/* Site Description */}
          <View style={[zero.gap.all[8]]}>
            <Text size="lg" weight="semibold">
              Site Description
            </Text>
            <Text size="sm" color="muted">
              Current: {currentDescription?.data || "Live streaming platform"}
            </Text>
            <View style={[zero.layout.flex.direction.row, zero.gap.all[8]]}>
              <View style={{ flex: 1 }}>
                <Input
                  placeholder="Enter site description"
                  value={siteDescription}
                  onChangeText={setSiteDescription}
                />
              </View>
              <Button
                onPress={() => uploadText("siteDescription", siteDescription)}
                disabled={uploading || !siteDescription.trim()}
              >
                Update
              </Button>
            </View>
          </View>

          {/* Primary Color */}
          <View style={[zero.gap.all[8]]}>
            <Text size="lg" weight="semibold">
              Primary Color
            </Text>
            <Text size="sm" color="muted">
              Current: {currentPrimaryColor?.data || "#6366f1"}
            </Text>
            <View style={[zero.layout.flex.direction.row, zero.gap.all[8]]}>
              <View style={{ flex: 1 }}>
                <Input
                  placeholder="#6366f1"
                  value={primaryColor}
                  onChangeText={setPrimaryColor}
                />
              </View>
              <Button
                onPress={() => uploadText("primaryColor", primaryColor)}
                disabled={uploading || !primaryColor.trim()}
              >
                Update
              </Button>
            </View>
          </View>

          {/* Accent Color */}
          <View style={[zero.gap.all[8]]}>
            <Text size="lg" weight="semibold">
              Accent Color
            </Text>
            <Text size="sm" color="muted">
              Current: {currentAccentColor?.data || "#8b5cf6"}
            </Text>
            <View style={[zero.layout.flex.direction.row, zero.gap.all[8]]}>
              <View style={{ flex: 1 }}>
                <Input
                  placeholder="#8b5cf6"
                  value={accentColor}
                  onChangeText={setAccentColor}
                />
              </View>
              <Button
                onPress={() => uploadText("accentColor", accentColor)}
                disabled={uploading || !accentColor.trim()}
              >
                Update
              </Button>
            </View>
          </View>

          {/* Default Streamer */}
          <View style={[zero.gap.all[8]]}>
            <Text size="lg" weight="semibold">
              Default Streamer
            </Text>
            <Text size="sm" color="muted">
              Current: {currentDefaultStreamer?.data || "None"}
            </Text>
            <View style={[zero.layout.flex.direction.row, zero.gap.all[8]]}>
              <View style={{ flex: 1 }}>
                <Input
                  placeholder="did:plc:..."
                  value={defaultStreamer}
                  onChangeText={setDefaultStreamer}
                />
              </View>
              <Button
                onPress={() => uploadText("defaultStreamer", defaultStreamer)}
                disabled={uploading || !defaultStreamer.trim()}
              >
                Update
              </Button>
            </View>
            <Button
              variant="destructive"
              onPress={() => deleteBlob("defaultStreamer")}
              disabled={uploading}
            >
              Clear Default Streamer
            </Button>
          </View>

          {/* Main Logo */}
          <View style={[zero.gap.all[8]]}>
            <Text size="lg" weight="semibold">
              Main Logo
            </Text>
            <Text size="sm" color="muted">
              SVG, PNG, or JPEG (max 500KB) - Web only
            </Text>
            {currentLogo?.data && (
              <Image
                source={{ uri: currentLogo.data }}
                style={{ width: 200, height: 100, resizeMode: "contain" }}
              />
            )}
            <View style={[zero.layout.flex.direction.row, zero.gap.all[8]]}>
              <Button
                onPress={() =>
                  handleFileSelect(
                    "mainLogo",
                    "image/svg+xml,image/png,image/jpeg",
                  )
                }
                disabled={uploading || Platform.OS !== "web"}
              >
                Upload Logo
              </Button>
              <Button
                variant="destructive"
                onPress={() => deleteBlob("mainLogo")}
                disabled={uploading}
              >
                Delete Logo
              </Button>
            </View>
          </View>

          {/* Favicon */}
          <View style={[zero.gap.all[8]]}>
            <Text size="lg" weight="semibold">
              Favicon
            </Text>
            <Text size="sm" color="muted">
              SVG, PNG, or ICO (max 100KB) - Web only
            </Text>
            {currentFavicon?.data && (
              <Image
                source={{ uri: currentFavicon.data }}
                style={{ width: 64, height: 64, resizeMode: "contain" }}
              />
            )}
            <View style={[zero.layout.flex.direction.row, zero.gap.all[8]]}>
              <Button
                onPress={() =>
                  handleFileSelect(
                    "favicon",
                    "image/svg+xml,image/png,image/x-icon",
                  )
                }
                disabled={uploading || Platform.OS !== "web"}
              >
                Upload Favicon
              </Button>
              <Button
                variant="destructive"
                onPress={() => deleteBlob("favicon")}
                disabled={uploading}
              >
                Delete Favicon
              </Button>
            </View>
          </View>

          <Text size="sm" color="muted" style={{ marginTop: 16 }}>
            Note: You must be an authorized admin DID to make changes.
            {Platform.OS !== "web" &&
              " Image uploads are only available on web."}
          </Text>
        </View>
      </View>
    </ScrollView>
  );
}
