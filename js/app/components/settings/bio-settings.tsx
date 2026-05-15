import type { LeafletFlatBlock, PanelRange } from "@streamplace/components";
import {
  autoSplitRanges,
  BioViewer,
  Button,
  extractLeafletBlocks,
  Input,
  LeafletPanelRangeSelector,
  MenuContainer,
  MenuGroup,
  resolveDIDDocument,
  Text,
  useBio,
  useDID,
  useFetchLeafletDoc,
  useGetBio,
  useImportBioFromLeaflet,
  useImportBioFromRanges,
  usePutBio,
  useTheme,
  useTranslation,
  View,
  zero,
} from "@streamplace/components";
import {
  ArrowLeft,
  Check,
  Download,
  Plus,
  Save,
  Trash2,
  X,
} from "lucide-react-native";
import { useEffect, useState } from "react";
import { Modal, Pressable, ScrollView } from "react-native";
import { parseAtUriPath } from "src/linking-config";
import type { PlaceStreamBioDefs, PlaceStreamBioPage } from "streamplace";

function parseSocialUrl(
  url: string,
): Pick<PlaceStreamBioDefs.Social, "platform" | "handle"> {
  try {
    const u = new URL(url.trim());
    const host = u.hostname.replace(/^www\./, "");
    const segments = u.pathname.replace(/^\//, "").split("/").filter(Boolean);
    const first = segments[0] ?? "";

    if (host === "bsky.app" && first === "profile" && segments[1])
      return { platform: "bluesky", handle: `@${segments[1]}` };
    if (host === "twitter.com" || host === "x.com")
      return { platform: "twitter", handle: first ? `@${first}` : "" };
    if (host === "youtube.com") {
      if (first.startsWith("@")) return { platform: "youtube", handle: first };
      if ((first === "c" || first === "user") && segments[1])
        return { platform: "youtube", handle: segments[1] };
      return { platform: "youtube", handle: "" };
    }
    if (host === "twitch.tv") return { platform: "twitch", handle: first };
    if (host === "kick.com") return { platform: "kick", handle: first };
    if (host === "discord.gg" || host === "discord.com")
      return { platform: "discord", handle: "" };
    if (host === "instagram.com")
      return { platform: "instagram", handle: first ? `@${first}` : "" };
    if (host === "tiktok.com")
      return {
        platform: "tiktok",
        handle: first.startsWith("@") ? first : first ? `@${first}` : "",
      };
    if (host === "github.com") return { platform: "github", handle: first };
    if (host === "cash.app")
      return {
        platform: "cashapp",
        handle: first.startsWith("$") ? first : first ? `$${first}` : "",
      };
    if (host === "ko-fi.com") return { platform: "ko-fi", handle: first };
    if (host === "patreon.com") return { platform: "patreon", handle: first };
  } catch {}
  return { platform: "website", handle: "" };
}

export function BioSettings() {
  const { t } = useTranslation("settings");
  const bio = useBio();
  const getBio = useGetBio();
  const putBio = usePutBio();
  const importBioFromLeaflet = useImportBioFromLeaflet();
  const did = useDID();
  const { theme, zero: z } = useTheme();

  const [leafletSource, setLeafletSource] = useState("");
  const [importing, setImporting] = useState(false);
  const [warnings, setWarnings] = useState<string[]>([]);
  const [importError, setImportError] = useState<string | null>(null);

  const [description, setDescription] = useState("");
  const [socials, setSocials] = useState<PlaceStreamBioDefs.Social[]>([]);
  const [saving, setSaving] = useState(false);
  const [edited, setEdited] = useState(false);

  const [rangeBlocks, setRangeBlocks] = useState<LeafletFlatBlock[] | null>(
    null,
  );
  const [ranges, setRanges] = useState<PanelRange[]>([]);
  const [rangeDoc, setRangeDoc] = useState<object | null>(null);
  const [rangeSource, setRangeSource] = useState("");
  const [rangeImporting, setRangeImporting] = useState(false);
  const [showConfirmModal, setShowConfirmModal] = useState(false);

  const fetchLeafletDoc = useFetchLeafletDoc();
  const importBioFromRanges = useImportBioFromRanges();

  useEffect(() => {
    getBio();
  }, []);

  useEffect(() => {
    if (bio) {
      setDescription(bio.description?.plaintext ?? "");
      setSocials(bio.socials ?? []);
      setEdited(false);
    }
  }, [bio]);

  useEffect(() => {
    if (!bio?.importedFrom || !did) return;
    const source = bio.importedFrom;
    setLeafletSource(source);

    const preload = async () => {
      try {
        const doc = await fetchLeafletDoc(source);
        const { blocks, warnings: w } = extractLeafletBlocks(doc);
        if (blocks.length > 0) {
          setRangeBlocks(blocks);
          setRangeDoc(doc);
          setRangeSource(source);
          setRanges(autoSplitRanges(blocks));
          setWarnings(w);
        }
      } catch {
        // we can disregard errors here
      }
    };
    preload();
  }, [bio?.importedFrom, did]);

  const handleImport = async () => {
    if (!leafletSource.trim()) return;
    setImporting(true);
    setImportError(null);
    setWarnings([]);
    let parsedUri = parseAtUriPath(leafletSource);
    if (parsedUri === null || parsedUri === undefined || !did) {
      setImportError("Invalid DID");
      return;
    }
    if (parsedUri?.authority != did) {
      setImportError("This is not your record");
    }
    resolveDIDDocument(did);
    try {
      const result = await importBioFromLeaflet({
        source: leafletSource.trim(),
      });
      setWarnings(result.warnings);
      setLeafletSource("");
    } catch (e: any) {
      setImportError(e?.message ?? "Import failed");
    } finally {
      setImporting(false);
    }
  };

  const handleOpenRangeSelector = async () => {
    if (!leafletSource.trim()) return;
    setImportError(null);
    setImporting(true);
    let parsedUri = parseAtUriPath(leafletSource);
    if (parsedUri === null || parsedUri === undefined || !did) {
      setImportError("Invalid DID");
      setImporting(false);
      return;
    }
    if (parsedUri?.authority != did) {
      setImportError("This is not your record");
      setImporting(false);
      return;
    }

    if (leafletSource.trim() === rangeSource && rangeBlocks) {
      setLeafletSource("");
      setImporting(false);
      return;
    }

    try {
      resolveDIDDocument(did);
      const doc = await fetchLeafletDoc(leafletSource.trim());
      const { blocks, warnings: w } = extractLeafletBlocks(doc);
      if (blocks.length === 0) {
        setImportError("No importable blocks found in this leaflet document.");
        setImporting(false);
        return;
      }
      setRangeBlocks(blocks);
      setRangeDoc(doc);
      setRangeSource(leafletSource.trim());
      setRanges(autoSplitRanges(blocks));
      setWarnings(w);
      setLeafletSource("");
    } catch (e: any) {
      setImportError(e?.message ?? "Failed to fetch leaflet document");
    } finally {
      setImporting(false);
    }
  };

  const selectedBlockCount = ranges.reduce(
    (sum, r) => sum + (r.endIdx - r.startIdx + 1),
    0,
  );
  const totalBlocks = rangeBlocks?.length ?? 0;
  const unselectedCount = totalBlocks - selectedBlockCount;

  const handleConfirmRanges = () => {
    if (unselectedCount > 0) {
      setShowConfirmModal(true);
      return;
    }
    doRangeImport();
  };

  const doRangeImport = async () => {
    setShowConfirmModal(false);
    if (!rangeDoc || !rangeBlocks || ranges.length === 0) return;
    setRangeImporting(true);
    try {
      const result = await importBioFromRanges(
        rangeSource,
        rangeDoc,
        rangeBlocks,
        ranges,
      );
      setWarnings(result.warnings);
      closeRangeSelector();
    } catch (e: any) {
      setImportError(e?.message ?? "Import failed");
    } finally {
      setRangeImporting(false);
    }
  };

  const closeRangeSelector = () => {
    setRangeBlocks(null);
    setRangeDoc(null);
    setRangeSource("");
    setRanges([]);
  };

  const handleSave = async () => {
    setSaving(true);
    try {
      const now = new Date().toISOString();
      const updated: PlaceStreamBioPage.Record = {
        $type: "place.stream.bio.page",
        ...bio,
        description: description
          ? { plaintext: description, facets: bio?.description?.facets }
          : undefined,
        socials: socials.length > 0 ? socials : undefined,
        createdAt: bio?.createdAt ?? now,
        updatedAt: now,
      };
      await putBio(updated);
      setEdited(false);
    } catch (e: any) {
      console.error("Failed to save bio", e);
    } finally {
      setSaving(false);
    }
  };

  const handleDescriptionChange = (text: string) => {
    setDescription(text);
    setEdited(true);
  };

  const addSocial = () => {
    setSocials([...socials, { platform: "website", url: "", handle: "" }]);
    setEdited(true);
  };

  const updateSocialUrl = (idx: number, value: string) => {
    const next = [...socials];
    next[idx] = { ...next[idx], url: value, ...parseSocialUrl(value) };
    setSocials(next);
    setEdited(true);
  };

  const removeSocial = (idx: number) => {
    setSocials(socials.filter((_, i) => i !== idx));
    setEdited(true);
  };

  return (
    <ScrollView>
      <View style={[zero.layout.flex.align.center, zero.px[2], zero.py[2]]}>
        <View style={{ maxWidth: 500, width: "100%" }}>
          <MenuContainer>
            <MenuGroup>
              <View style={[zero.p[4]]}>
                <Text size="xl" weight="bold">
                  {t("import-from-leaflet", "Import from Leaflet")}
                </Text>
                <Text color="muted" size="sm" style={{ marginTop: 4 }}>
                  {t(
                    "import-from-leaflet-desc",
                    "Paste a leaflet-flavored site.standard.document AT-uri",
                  )}
                </Text>
                <View
                  direction="row"
                  style={[zero.gap.all[2], { marginTop: 12 }]}
                >
                  <View style={{ flex: 1 }}>
                    <Input
                      placeholder="at://did:plc:.../site.standard.document/abc123"
                      value={leafletSource}
                      onChangeText={setLeafletSource}
                    />
                  </View>
                  <View style={[zero.layout.flex.center]}>
                    <Button
                      onPress={handleImport}
                      loading={importing}
                      disabled={!leafletSource.trim() || !did}
                      size="md"
                      width="min"
                      style={[]}
                      leftIcon={
                        <Download
                          size={16}
                          color={theme.colors.primaryForeground}
                        />
                      }
                    >
                      {t("import", "Import")}
                    </Button>
                  </View>
                  <View style={[zero.layout.flex.center]}>
                    <Button
                      onPress={handleOpenRangeSelector}
                      loading={importing}
                      disabled={!leafletSource.trim() || !did}
                      variant="secondary"
                      size="md"
                      width="min"
                    >
                      {t("select-panels", "Select Panels")}
                    </Button>
                  </View>
                </View>
                {importError && (
                  <Text color="destructive" size="sm" style={{ marginTop: 8 }}>
                    {importError}
                  </Text>
                )}
                {warnings.length > 0 && (
                  <View style={{ marginTop: 8 }}>
                    {warnings.map((w, i) => (
                      <Text key={i} size="sm" color="warning">
                        {w}
                      </Text>
                    ))}
                  </View>
                )}
              </View>
            </MenuGroup>

            {rangeBlocks !== null && (
              <MenuGroup>
                <View style={[zero.p[4]]}>
                  <View
                    direction="row"
                    align="center"
                    justify="between"
                    style={{ marginBottom: 12 }}
                  >
                    <View
                      direction="row"
                      align="center"
                      style={[zero.gap.all[2]]}
                    >
                      <Pressable onPress={closeRangeSelector}>
                        <ArrowLeft size={18} color={theme.colors.foreground} />
                      </Pressable>
                      <Text size="lg" weight="bold">
                        {t("select-panels", "Select Panels")}
                      </Text>
                    </View>
                    <Button
                      onPress={handleConfirmRanges}
                      loading={rangeImporting}
                      disabled={ranges.length === 0}
                      size="sm"
                      width="min"
                      leftIcon={
                        <Check
                          size={14}
                          color={theme.colors.primaryForeground}
                        />
                      }
                    >
                      {t("confirm-import", "Confirm Import")}
                    </Button>
                  </View>

                  {unselectedCount > 0 && ranges.length > 0 && (
                    <Text size="sm" color="muted" style={{ marginBottom: 8 }}>
                      {unselectedCount} of {totalBlocks} blocks will be dropped
                    </Text>
                  )}

                  <LeafletPanelRangeSelector
                    blocks={rangeBlocks}
                    ranges={ranges}
                    onRangesChange={setRanges}
                  />
                </View>
              </MenuGroup>
            )}

            {bio && (
              <MenuGroup>
                <View style={[zero.p[4]]}>
                  <Text size="xl" weight="bold">
                    {t("bio-preview", "Preview")}
                  </Text>
                </View>
                <BioViewer bio={bio} did={did} />
              </MenuGroup>
            )}

            <MenuGroup>
              <View style={[zero.p[4]]}>
                <Text size="xl" weight="bold">
                  {t("edit-description", "Edit Description")}
                </Text>
                <View style={{ marginTop: 8 }}>
                  <Input
                    multiline
                    numberOfLines={4}
                    placeholder={t(
                      "description-placeholder",
                      "Write something about yourself...",
                    )}
                    value={description}
                    onChangeText={handleDescriptionChange}
                  />
                </View>
              </View>
            </MenuGroup>

            <MenuGroup>
              <View style={[zero.p[4]]}>
                <View
                  direction="row"
                  justify="between"
                  align="center"
                  style={{ marginBottom: 12 }}
                >
                  <Text size="xl" weight="bold">
                    {t("social-links", "Social Links")}
                  </Text>
                  <Button
                    variant="secondary"
                    size="pill"
                    width="min"
                    onPress={addSocial}
                    leftIcon={
                      <Plus
                        size={14}
                        style={{ marginRight: 2 }}
                        color={theme.colors.primaryForeground}
                      />
                    }
                  >
                    {t("add", "Add")}
                  </Button>
                </View>
                {socials.map((social, idx) => (
                  <View
                    key={idx}
                    direction="row"
                    align="center"
                    style={[zero.gap.all[2], { marginBottom: 8 }]}
                  >
                    <Text
                      size="xs"
                      color="muted"
                      style={{ minWidth: 64, textAlign: "right" }}
                    >
                      {social.platform || "website"}
                    </Text>
                    <View style={{ flex: 1 }}>
                      <Input
                        placeholder="https://..."
                        value={social.url}
                        size="sm"
                        onChangeText={(v) => updateSocialUrl(idx, v)}
                        autoCapitalize="none"
                        autoCorrect={false}
                      />
                    </View>
                    <Button
                      variant="destructive"
                      size="sm"
                      width="min"
                      onPress={() => removeSocial(idx)}
                    >
                      <Trash2 size={14} />
                    </Button>
                  </View>
                ))}
              </View>
            </MenuGroup>

            {edited && (
              <MenuGroup>
                <View style={[zero.p[4]]}>
                  <Button onPress={handleSave} loading={saving}>
                    <Save size={16} style={{ marginRight: 4 }} />
                    {t("save-bio", "Save Bio")}
                  </Button>
                </View>
              </MenuGroup>
            )}
          </MenuContainer>
        </View>
      </View>
      <Modal
        visible={showConfirmModal}
        transparent
        animationType="fade"
        onRequestClose={() => setShowConfirmModal(false)}
      >
        <View
          style={{
            flex: 1,
            backgroundColor: "rgba(0,0,0,0.5)",
            alignItems: "center",
            justifyContent: "center",
            padding: 24,
          }}
        >
          <View
            style={{
              backgroundColor: theme.colors.card,
              borderRadius: 12,
              padding: 24,
              maxWidth: 400,
              width: "100%",
            }}
          >
            <View
              direction="row"
              align="center"
              justify="between"
              style={{ marginBottom: 16 }}
            >
              <Text size="lg" weight="bold">
                {t("confirm-import-title", "Confirm Import")}
              </Text>
              <Pressable onPress={() => setShowConfirmModal(false)}>
                <X size={18} color={theme.colors.mutedForeground} />
              </Pressable>
            </View>
            <Text size="sm" color="muted" style={{ marginBottom: 16 }}>
              {unselectedCount} block(s) are not in any panel and will be
              dropped. Continue?
            </Text>
            <View direction="row" style={[zero.gap.all[2]]}>
              <Button
                variant="secondary"
                size="md"
                width="min"
                onPress={() => setShowConfirmModal(false)}
              >
                {t("cancel", "Cancel")}
              </Button>
              <Button
                size="md"
                onPress={doRangeImport}
                loading={rangeImporting}
              >
                {t("import-anyway", "Import Anyway")}
              </Button>
            </View>
          </View>
        </View>
      </Modal>
    </ScrollView>
  );
}
