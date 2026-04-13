import { BlobRef } from "@atproto/lexicon";
import {
  Button,
  Input,
  MenuContainer,
  MenuGroup,
  MenuSeparator,
  Text,
  useToast,
  View,
  zero,
} from "@streamplace/components";
import { usePDSAgent } from "@streamplace/components/src/streamplace-store/xrpc";
import { Image } from "expo-image";
import { Check, ChevronLeft, ImagePlus, Plus, X } from "lucide-react-native";
import { useCallback, useEffect, useState } from "react";
import {
  ActivityIndicator,
  Platform,
  ScrollView,
  TouchableOpacity,
} from "react-native";
import type { PlaceStreamBadgeDef } from "streamplace";

const { gap, p, px, py, r, layout } = zero;

type BadgeType =
  | "place.stream.badge.defs#vip"
  | "place.stream.badge.defs#event";

const BADGE_TYPE_OPTIONS: { label: string; value: BadgeType }[] = [
  { label: "VIP", value: "place.stream.badge.defs#vip" },
  { label: "Event", value: "place.stream.badge.defs#event" },
];

type PanelView = "main" | "create";

interface BadgeDefItem {
  uri: string;
  value: PlaceStreamBadgeDef.Record;
}

function getDidFromAtUri(uri: string) {
  const parts = uri.split("/");
  if (parts.length >= 3) {
    return parts[2];
  }
  return null;
}

function BadgeDefRow({
  def,
  selected,
  onPress,
}: {
  def: BadgeDefItem;
  selected: boolean;
  onPress: () => void;
}) {
  const { theme } = zero.useTheme();

  return (
    <TouchableOpacity
      onPress={onPress}
      style={[
        layout.flex.row,
        layout.flex.align.center,
        gap.all[3],
        py[3],
        px[4],
        {
          backgroundColor: selected
            ? theme.colors.primary + "18"
            : "transparent",
          borderRadius: 8,
        },
      ]}
    >
      {def.value.image ? (
        <Image
          source={{
            uri:
              "https://cdn.bsky.app/img/feed_fullsize/plain/" +
              getDidFromAtUri(def.uri) +
              "/" +
              def.value.image.ref.toString(),
          }}
          style={{ width: 28, height: 28, borderRadius: 4 }}
        />
      ) : (
        <View
          style={{
            width: 28,
            height: 28,
            borderRadius: 4,
            backgroundColor: theme.colors.muted,
            alignItems: "center",
            justifyContent: "center",
          }}
        >
          <Text style={{ fontSize: 12 }}>🎭</Text>
        </View>
      )}
      <View style={[{ flex: 1 }, gap.all[0.5]]}>
        <Text
          style={{ color: theme.colors.text, fontSize: 14, fontWeight: "500" }}
        >
          {def.value.name}
        </Text>
        <Text style={{ color: theme.colors.textMuted, fontSize: 11 }}>
          {def.value.badgeType.split("#")[1]}
        </Text>
      </View>
    </TouchableOpacity>
  );
}

export function BadgeIssuerPanel() {
  const agent = usePDSAgent();
  const { theme } = zero.useTheme();
  const toast = useToast();

  const [view, setView] = useState<PanelView>("main");
  const [working, setWorking] = useState(false);
  const [lastResult, setLastResult] = useState<{
    label: string;
    uri: string;
  } | null>(null);

  const [defs, setDefs] = useState<BadgeDefItem[]>([]);
  const [loadingDefs, setLoadingDefs] = useState(false);
  const [selectedDefUri, setSelectedDefUri] = useState<string | null>(null);

  const [createName, setCreateName] = useState("");
  const [createDescription, setCreateDescription] = useState("");
  const [createBadgeType, setCreateBadgeType] = useState<BadgeType>(
    "place.stream.badge.defs#vip",
  );
  const [createImageUri, setCreateImageUri] = useState<string | null>(null);
  const [createImageBlob, setCreateImageBlob] = useState<Blob | null>(null);

  const loadDefs = useCallback(async () => {
    if (!agent?.did) return;
    setLoadingDefs(true);
    try {
      const res = await agent.place.stream.badge.def.list({
        repo: agent.did,
        limit: 100,
      });
      setDefs(res.records);
    } catch (e: any) {
      toast.show("Failed to load badge definitions", e?.message, {
        variant: "error",
      });
    } finally {
      setLoadingDefs(false);
    }
  }, [agent]);

  useEffect(() => {
    if (agent?.did) {
      loadDefs();
    }
  }, [agent, loadDefs]);

  const pickImage = useCallback(() => {
    if (Platform.OS !== "web") {
      toast.show("Image upload is only available on web", undefined, {
        variant: "error",
      });
      return;
    }
    // @ts-ignore document exists on web
    const input = document.createElement("input");
    input.type = "file";
    input.accept = "image/png,image/jpeg,image/gif,image/webp";
    input.onchange = (e: any) => {
      const file = e.target?.files?.[0];
      if (file) {
        if (file.size > 262144) {
          toast.show("Image must be under 256KB", undefined, {
            variant: "error",
          });
          return;
        }
        const blob = new Blob([file], { type: file.type });
        const url = URL.createObjectURL(blob);
        setCreateImageUri(url);
        setCreateImageBlob(blob);
      }
    };
    input.click();
  }, []);

  const handleCreateDef = async () => {
    if (!agent?.did || !createName.trim() || working) return;
    setWorking(true);
    try {
      let imageBlob: BlobRef | undefined;
      if (createImageBlob) {
        const uploaded = await agent.uploadBlob(createImageBlob, {
          encoding: createImageBlob.type,
        });
        imageBlob = uploaded.data.blob;
      }

      await agent.place.stream.badge.def.create(
        { repo: agent.did },
        {
          name: createName.trim(),
          description: createDescription.trim() || undefined,
          badgeType: createBadgeType,
          image: imageBlob,
          createdAt: new Date().toISOString(),
        },
      );

      setLastResult({
        label: "Badge definition created",
        uri: createName.trim(),
      });
      setCreateName("");
      setCreateDescription("");
      setCreateImageUri(null);
      setCreateImageBlob(null);
      toast.show("Badge definition created", undefined, {
        variant: "success",
      });
      loadDefs();
      setView("main");
    } catch (e: any) {
      toast.show("Failed to create badge definition", e?.message, {
        variant: "error",
      });
    } finally {
      setWorking(false);
    }
  };

  const renderBackButton = (label: string) => (
    <TouchableOpacity
      onPress={() => setView("main")}
      style={[
        layout.flex.row,
        layout.flex.align.center,
        gap.all[2],
        py[2],
        { marginBottom: 4 },
      ]}
    >
      <ChevronLeft size={18} color={theme.colors.textMuted} />
      <Text style={{ color: theme.colors.textMuted, fontSize: 14 }}>
        {label}
      </Text>
    </TouchableOpacity>
  );

  if (view === "create") {
    return (
      <ScrollView>
        <View style={[layout.flex.align.center, px[2], py[2]]}>
          <View style={{ maxWidth: 500, width: "100%" }}>
            {renderBackButton("Badge definitions")}
            <Text
              style={{
                color: theme.colors.text,
                fontSize: 18,
                fontWeight: "600",
              }}
            >
              Create Badge Definition
            </Text>
            <Text
              style={{
                color: theme.colors.textMuted,
                fontSize: 13,
                marginBottom: 16,
              }}
            >
              Create a reusable badge definition. You can then issue it to
              multiple users.
            </Text>

            <View style={[gap.all[4]]}>
              <View style={[gap.all[2]]}>
                <Text
                  style={[
                    {
                      color: theme.colors.text,
                      fontSize: 13,
                      fontWeight: "600",
                    },
                  ]}
                >
                  Badge type
                </Text>
                <View style={[layout.flex.row, gap.all[2]]}>
                  {BADGE_TYPE_OPTIONS.map(({ label, value }) => (
                    <Button
                      key={value}
                      variant={
                        createBadgeType === value ? "primary" : "secondary"
                      }
                      size="pill"
                      width="min"
                      onPress={() => setCreateBadgeType(value)}
                    >
                      {label}
                    </Button>
                  ))}
                </View>
              </View>

              <View style={[gap.all[2]]}>
                <Text
                  style={[
                    {
                      color: theme.colors.text,
                      fontSize: 13,
                      fontWeight: "600",
                    },
                  ]}
                >
                  Badge name
                </Text>
                <Input
                  value={createName}
                  onChangeText={setCreateName}
                  placeholder="e.g. VIP Member"
                  maxLength={64}
                />
              </View>

              <View style={[gap.all[2]]}>
                <Text
                  style={[
                    {
                      color: theme.colors.text,
                      fontSize: 13,
                      fontWeight: "600",
                    },
                  ]}
                >
                  Description (optional)
                </Text>
                <Input
                  value={createDescription}
                  onChangeText={setCreateDescription}
                  placeholder="e.g. Outstanding community support"
                  maxLength={256}
                />
              </View>

              <View style={[gap.all[2]]}>
                <Text
                  style={[
                    {
                      color: theme.colors.text,
                      fontSize: 13,
                      fontWeight: "600",
                    },
                  ]}
                >
                  Badge image (optional, max 256KB)
                </Text>
                <View
                  style={[
                    layout.flex.row,
                    gap.all[3],
                    layout.flex.align.center,
                  ]}
                >
                  {createImageUri ? (
                    <View
                      style={[
                        layout.flex.row,
                        layout.flex.align.center,
                        gap.all[2],
                      ]}
                    >
                      <Image
                        source={{ uri: createImageUri }}
                        style={{ width: 48, height: 48, borderRadius: 6 }}
                      />
                      <TouchableOpacity
                        onPress={() => {
                          setCreateImageUri(null);
                          setCreateImageBlob(null);
                        }}
                      >
                        <X size={16} color={theme.colors.textMuted} />
                      </TouchableOpacity>
                    </View>
                  ) : (
                    <Button
                      variant="secondary"
                      size="sm"
                      width="min"
                      onPress={pickImage}
                    >
                      <View
                        style={[
                          layout.flex.row,
                          layout.flex.align.center,
                          gap.all[2],
                        ]}
                      >
                        <ImagePlus size={14} color={theme.colors.textMuted} />
                        <Text style={{ fontSize: 12 }}>Choose image</Text>
                      </View>
                    </Button>
                  )}
                </View>
              </View>

              <Button
                onPress={handleCreateDef}
                disabled={!createName.trim() || working}
                style={[
                  {
                    opacity: createName.trim() && !working ? 1 : 0.5,
                  },
                ]}
              >
                {working ? (
                  <ActivityIndicator
                    size="small"
                    color={theme.colors.primaryForeground}
                  />
                ) : (
                  "Create Badge Definition"
                )}
              </Button>
            </View>
          </View>
        </View>
      </ScrollView>
    );
  }

  return (
    <ScrollView>
      <View style={[layout.flex.align.center, px[2], py[2]]}>
        <View style={{ maxWidth: 500, width: "100%" }}>
          <Text style={{ color: theme.colors.textMuted, fontSize: 13 }}>
            Manage badge definitions and issue badges to users.
          </Text>

          <MenuContainer>
            <MenuGroup>
              <TouchableOpacity
                onPress={() => setView("create")}
                style={[
                  layout.flex.row,
                  layout.flex.align.center,
                  gap.all[3],
                  py[3],
                  px[4],
                ]}
              >
                <View
                  style={{
                    width: 32,
                    height: 32,
                    borderRadius: 16,
                    backgroundColor: theme.colors.primary + "22",
                    alignItems: "center",
                    justifyContent: "center",
                  }}
                >
                  <Plus size={16} color={theme.colors.primary} />
                </View>
                <View style={{ flex: 1 }}>
                  <Text
                    style={{
                      color: theme.colors.text,
                      fontSize: 15,
                      fontWeight: "500",
                    }}
                  >
                    Create Badge Definition
                  </Text>
                  <Text style={{ color: theme.colors.textMuted, fontSize: 12 }}>
                    Define a reusable badge with name, type, and optional image
                  </Text>
                </View>
              </TouchableOpacity>
            </MenuGroup>
          </MenuContainer>

          {lastResult && (
            <View
              style={[
                r.md,
                p[3],
                gap.all[1],
                {
                  backgroundColor: theme.colors.success + "22",
                  borderWidth: 1,
                  borderColor: theme.colors.success + "44",
                },
              ]}
            >
              <View
                style={[layout.flex.row, layout.flex.align.center, gap.all[2]]}
              >
                <Check size={14} color={theme.colors.success} />
                <Text
                  style={[
                    {
                      color: theme.colors.success,
                      fontSize: 13,
                      fontWeight: "600",
                    },
                  ]}
                >
                  {lastResult.label}
                </Text>
              </View>
              <Text
                style={[
                  {
                    color: theme.colors.textMuted,
                    fontSize: 11,
                    fontFamily: "monospace",
                  },
                ]}
                numberOfLines={1}
                ellipsizeMode="middle"
              >
                {lastResult.uri}
              </Text>
            </View>
          )}

          {defs.length > 0 && (
            <>
              <Text
                style={{
                  color: theme.colors.text,
                  fontSize: 15,
                  fontWeight: "600",
                  marginTop: 4,
                }}
              >
                Your Badge Definitions
              </Text>
              <MenuContainer>
                <MenuGroup>
                  {defs.map((def, i) => (
                    <View key={def.uri}>
                      {i > 0 && <MenuSeparator />}
                      <View
                        style={[
                          layout.flex.row,
                          layout.flex.align.center,
                          gap.all[3],
                          py[3],
                          px[4],
                        ]}
                      >
                        {def.value.image ? (
                          <Image
                            source={{
                              uri:
                                "https://cdn.bsky.app/img/feed_fullsize/plain/" +
                                getDidFromAtUri(def.uri) +
                                "/" +
                                def.value.image.ref.toString(),
                            }}
                            style={{ width: 24, height: 24, borderRadius: 4 }}
                          />
                        ) : (
                          <View
                            style={{
                              width: 24,
                              height: 24,
                              borderRadius: 4,
                              backgroundColor: theme.colors.muted,
                              alignItems: "center",
                              justifyContent: "center",
                            }}
                          >
                            <Text style={{ fontSize: 10 }}>🎭</Text>
                          </View>
                        )}
                        <View style={{ flex: 1 }}>
                          <Text
                            style={{
                              color: theme.colors.text,
                              fontSize: 14,
                              fontWeight: "500",
                            }}
                          >
                            {def.value.name}
                          </Text>
                          <Text
                            style={{
                              color: theme.colors.textMuted,
                              fontSize: 11,
                            }}
                          >
                            {def.value.badgeType.split("#")[1]}
                          </Text>
                        </View>
                      </View>
                    </View>
                  ))}
                </MenuGroup>
              </MenuContainer>
            </>
          )}
        </View>
      </View>
    </ScrollView>
  );
}
