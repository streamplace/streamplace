import { BlobRef } from "@atproto/lexicon";
import {
  Button,
  hexToRgba,
  IconButton,
  Input,
  MenuContainer,
  MenuGroup,
  MenuSeparator,
  Text,
  useToast,
  useTranslation,
  View,
  zero,
} from "@streamplace/components";
import {
  fontFamilies,
  borderRadius as radiusTokens,
} from "@streamplace/components/src/lib/theme/tokens";
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

const { gap, p, px, py, layout } = zero;

type BadgeType =
  | "place.stream.badge.defs#vip"
  | "place.stream.badge.defs#event";

const BADGE_TYPE_OPTIONS: { label: string; value: BadgeType }[] = [
  { label: "VIP", value: "place.stream.badge.defs#vip" },
  { label: "Event", value: "place.stream.badge.defs#event" },
];

type PanelView = "main" | "create" | "issue";

interface BadgeDefItem {
  uri: string;
  cid: string;
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
  selected?: boolean;
  onPress?: () => void;
}) {
  const { theme } = zero.useTheme();

  const content = (
    <>
      {def.value.image ? (
        <Image
          source={{
            uri:
              "https://cdn.bsky.app/img/feed_fullsize/plain/" +
              getDidFromAtUri(def.uri) +
              "/" +
              def.value.image.ref.toString(),
          }}
          style={{ width: 28, height: 28, borderRadius: radiusTokens.sm }}
        />
      ) : (
        <View
          style={{
            width: 28,
            height: 28,
            borderRadius: radiusTokens.sm,
            backgroundColor: theme.colors.muted,
            alignItems: "center",
            justifyContent: "center",
          }}
        >
          <Text style={{ fontSize: 12 }}>🎭</Text>
        </View>
      )}
      <View style={[{ flex: 1 }, gap.all[0.5]]}>
        <Text style={{ fontSize: 14, fontWeight: "500" }}>
          {def.value.name}
        </Text>
        <Text muted style={{ fontSize: 11 }}>
          {def.value.badgeType.split("#")[1]}
        </Text>
      </View>
    </>
  );

  const rowStyle = [
    layout.flex.row,
    layout.flex.align.center,
    gap.all[3],
    py[3],
    px[4],
    {
      backgroundColor: selected
        ? hexToRgba(theme.colors.primary, 0.09)
        : "transparent",
      borderRadius: radiusTokens.md,
    },
  ];

  if (onPress) {
    return (
      <TouchableOpacity
        onPress={onPress}
        accessibilityRole="button"
        accessibilityLabel={`${def.value.name} badge definition`}
        style={rowStyle}
      >
        {content}
      </TouchableOpacity>
    );
  }

  return <View style={rowStyle}>{content}</View>;
}

function BackButton({
  label,
  onPress,
}: {
  label: string;
  onPress: () => void;
}) {
  const { theme } = zero.useTheme();

  return (
    <TouchableOpacity
      onPress={onPress}
      accessibilityRole="button"
      accessibilityLabel={`Back to ${label}`}
      style={[
        layout.flex.row,
        layout.flex.align.center,
        gap.all[2],
        py[2],
        { marginBottom: 4 },
      ]}
    >
      <ChevronLeft size={18} color={theme.colors.textMuted} />
      <Text muted style={{ fontSize: 14 }}>
        {label}
      </Text>
    </TouchableOpacity>
  );
}

export function BadgeIssuerPanel() {
  const agent = usePDSAgent();
  const { theme } = zero.useTheme();
  const toast = useToast();
  const { t } = useTranslation("settings");

  const [view, setView] = useState<PanelView>("main");
  const [working, setWorking] = useState(false);
  const [lastResult, setLastResult] = useState<{
    label: string;
    uri: string;
  } | null>(null);

  const [defs, setDefs] = useState<BadgeDefItem[]>([]);
  const [loadingDefs, setLoadingDefs] = useState(false);

  const [selectedDef, setSelectedDef] = useState<BadgeDefItem | null>(null);
  const [recipientDid, setRecipientDid] = useState("");

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
      setDefs(
        (
          res.records as {
            uri: string;
            cid: string;
            value: PlaceStreamBadgeDef.Record;
          }[]
        ).map(({ uri, cid, value }) => ({ uri, cid: cid ?? "", value })),
      );
    } catch (e: any) {
      toast.show(t("issue-badges-failed-load"), e?.message, {
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
      toast.show(t("issue-badges-image-web-only"), undefined, {
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
          toast.show(t("issue-badges-image-too-large"), undefined, {
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
        label: t("issue-badges-definition-created"),
        uri: createName.trim(),
      });
      setCreateName("");
      setCreateDescription("");
      setCreateImageUri(null);
      setCreateImageBlob(null);
      toast.show(t("issue-badges-definition-created"), undefined, {
        variant: "success",
      });
      loadDefs();
      setView("main");
    } catch (e: any) {
      toast.show(t("issue-badges-failed-create"), e?.message, {
        variant: "error",
      });
    } finally {
      setWorking(false);
    }
  };

  const handleIssueBadge = async () => {
    if (!agent?.did || !selectedDef || !recipientDid.trim() || working) return;
    setWorking(true);
    try {
      await agent.place.stream.badge.issuance.create(
        { repo: agent.did },
        {
          did: recipientDid.trim(),
          badge: { uri: selectedDef.uri, cid: selectedDef.cid },
          createdAt: new Date().toISOString(),
        },
      );
      setLastResult({
        label: t("issue-badges-issued-to", { did: recipientDid.trim() }),
        uri: recipientDid.trim(),
      });
      setRecipientDid("");
      toast.show(t("issue-badges-issued"), undefined, { variant: "success" });
      setSelectedDef(null);
      setView("main");
    } catch (e: any) {
      toast.show(t("issue-badges-failed-issue"), e?.message, {
        variant: "error",
      });
    } finally {
      setWorking(false);
    }
  };

  if (view === "issue" && selectedDef) {
    return (
      <ScrollView>
        <View style={[layout.flex.align.center, px[2], py[2]]}>
          <View style={{ maxWidth: 500, width: "100%" }}>
            <BackButton
              label={t("issue-badges-back-to-definitions")}
              onPress={() => {
                setSelectedDef(null);
                setView("main");
              }}
            />
            <Text style={{ fontSize: 18, fontWeight: "600" }}>
              {t("issue-badges-issue-badge")}
            </Text>
            <Text muted style={{ fontSize: 13, marginBottom: 16 }}>
              {t("issue-badges-issue-badge-description", {
                name: selectedDef.value.name,
              })}
            </Text>
            <View style={[gap.all[4]]}>
              <BadgeDefRow def={selectedDef} />
              <View style={[gap.all[2]]}>
                <Text style={{ fontSize: 13, fontWeight: "600" }}>
                  {t("issue-badges-recipient-did")}
                </Text>
                <Input
                  value={recipientDid}
                  onChangeText={setRecipientDid}
                  placeholder={t("issue-badges-recipient-did-placeholder")}
                  autoCapitalize="none"
                  autoCorrect={false}
                  accessibilityLabel={t("issue-badges-recipient-did")}
                />
              </View>
              <Button
                onPress={handleIssueBadge}
                disabled={!recipientDid.trim() || working}
                style={{ opacity: recipientDid.trim() && !working ? 1 : 0.5 }}
              >
                {working ? (
                  <ActivityIndicator
                    size="small"
                    color={theme.colors.primaryForeground}
                  />
                ) : (
                  t("issue-badges-issue-badge")
                )}
              </Button>
            </View>
          </View>
        </View>
      </ScrollView>
    );
  }

  if (view === "create") {
    return (
      <ScrollView>
        <View style={[layout.flex.align.center, px[2], py[2]]}>
          <View style={{ maxWidth: 500, width: "100%" }}>
            <BackButton
              label={t("issue-badges-back-to-definitions")}
              onPress={() => setView("main")}
            />
            <Text style={{ fontSize: 18, fontWeight: "600" }}>
              {t("issue-badges-create-definition")}
            </Text>
            <Text muted style={{ fontSize: 13, marginBottom: 16 }}>
              {t("issue-badges-create-definition-description")}
            </Text>

            <View style={[gap.all[4]]}>
              <View style={[gap.all[2]]}>
                <Text style={{ fontSize: 13, fontWeight: "600" }}>
                  {t("issue-badges-badge-type")}
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
                <Text style={{ fontSize: 13, fontWeight: "600" }}>
                  {t("issue-badges-badge-name")}
                </Text>
                <Input
                  value={createName}
                  onChangeText={setCreateName}
                  placeholder={t("issue-badges-badge-name-placeholder")}
                  maxLength={64}
                  accessibilityLabel={t("issue-badges-badge-name")}
                />
              </View>

              <View style={[gap.all[2]]}>
                <Text style={{ fontSize: 13, fontWeight: "600" }}>
                  {t("issue-badges-description-optional")}
                </Text>
                <Input
                  value={createDescription}
                  onChangeText={setCreateDescription}
                  placeholder={t("issue-badges-description-placeholder")}
                  maxLength={256}
                  accessibilityLabel={t("issue-badges-description-optional")}
                />
              </View>

              <View style={[gap.all[2]]}>
                <Text style={{ fontSize: 13, fontWeight: "600" }}>
                  {t("issue-badges-image-optional")}
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
                        style={{
                          width: 48,
                          height: 48,
                          borderRadius: radiusTokens.md,
                        }}
                      />
                      <IconButton
                        size="sm"
                        accessibilityLabel="Remove selected image"
                        onPress={() => {
                          setCreateImageUri(null);
                          setCreateImageBlob(null);
                        }}
                      >
                        <X size={16} color={theme.colors.textMuted} />
                      </IconButton>
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
                        <Text style={{ fontSize: 12 }}>
                          {t("issue-badges-choose-image")}
                        </Text>
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
                  t("issue-badges-create-definition")
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
          <Text muted style={{ fontSize: 13 }}>
            {t("issue-badges-manage-description")}
          </Text>

          <MenuContainer>
            <MenuGroup>
              <TouchableOpacity
                onPress={() => setView("create")}
                accessibilityRole="button"
                accessibilityLabel={t("issue-badges-create-definition")}
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
                    borderRadius: radiusTokens.xl,
                    backgroundColor: hexToRgba(theme.colors.primary, 0.13),
                    alignItems: "center",
                    justifyContent: "center",
                  }}
                >
                  <Plus size={16} color={theme.colors.primary} />
                </View>
                <View style={{ flex: 1 }}>
                  <Text style={{ fontSize: 15, fontWeight: "500" }}>
                    {t("issue-badges-create-definition")}
                  </Text>
                  <Text muted style={{ fontSize: 12 }}>
                    {t("issue-badges-create-definition-subtitle")}
                  </Text>
                </View>
              </TouchableOpacity>
            </MenuGroup>
          </MenuContainer>

          {lastResult && (
            <View
              accessibilityRole="alert"
              style={[
                { borderRadius: radiusTokens.md },
                p[3],
                gap.all[1],
                {
                  backgroundColor: hexToRgba(theme.colors.success, 0.13),
                  borderWidth: 1,
                  borderColor: hexToRgba(theme.colors.success, 0.27),
                },
              ]}
            >
              <View
                style={[layout.flex.row, layout.flex.align.center, gap.all[2]]}
              >
                <Check size={14} color={theme.colors.success} />
                <Text
                  color="success"
                  style={{ fontSize: 13, fontWeight: "600" }}
                >
                  {lastResult.label}
                </Text>
              </View>
              <Text
                muted
                numberOfLines={1}
                ellipsizeMode="middle"
                style={{ fontSize: 11, fontFamily: fontFamilies.monoRegular }}
              >
                {lastResult.uri}
              </Text>
            </View>
          )}

          {defs.length > 0 && (
            <>
              <Text style={{ fontSize: 15, fontWeight: "600", marginTop: 4 }}>
                {t("issue-badges-your-definitions")}
              </Text>
              <Text muted style={{ fontSize: 12, marginBottom: 4 }}>
                {t("issue-badges-tap-to-issue")}
              </Text>
              <MenuContainer>
                <MenuGroup>
                  {defs.map((def, i) => (
                    <View key={def.uri}>
                      {i > 0 && <MenuSeparator />}
                      <BadgeDefRow
                        def={def}
                        onPress={() => {
                          setSelectedDef(def);
                          setView("issue");
                        }}
                      />
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
