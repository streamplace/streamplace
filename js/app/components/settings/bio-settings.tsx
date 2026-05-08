import {
  BioViewer,
  Button,
  Input,
  MenuContainer,
  MenuGroup,
  Text,
  useBio,
  useDID,
  useGetBio,
  useImportBioFromLeaflet,
  usePutBio,
  useTheme,
  useTranslation,
  View,
  zero,
} from "@streamplace/components";
import { Download, Plus, Save, Trash2 } from "lucide-react-native";
import { useEffect, useState } from "react";
import { ScrollView } from "react-native";
import type { PlaceStreamBioDefs, PlaceStreamBioPage } from "streamplace";

const PLATFORM_OPTIONS: PlaceStreamBioDefs.Social["platform"][] = [
  "bluesky",
  "twitter",
  "youtube",
  "twitch",
  "kick",
  "discord",
  "instagram",
  "tiktok",
  "github",
  "cashapp",
  "ko-fi",
  "patreon",
  "website",
];

export function BioSettings() {
  const { t } = useTranslation("settings");
  const bio = useBio();
  const getBio = useGetBio();
  const putBio = usePutBio();
  const importBioFromLeaflet = useImportBioFromLeaflet();
  const did = useDID();
  const { theme } = useTheme();

  const [leafletSource, setLeafletSource] = useState("");
  const [importing, setImporting] = useState(false);
  const [warnings, setWarnings] = useState<string[]>([]);
  const [importError, setImportError] = useState<string | null>(null);

  const [description, setDescription] = useState("");
  const [socials, setSocials] = useState<PlaceStreamBioDefs.Social[]>([]);
  const [saving, setSaving] = useState(false);
  const [edited, setEdited] = useState(false);

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

  const handleImport = async () => {
    if (!leafletSource.trim()) return;
    setImporting(true);
    setImportError(null);
    setWarnings([]);
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

  const handleSave = async () => {
    if (!bio) return;
    setSaving(true);
    try {
      const updated: PlaceStreamBioPage.Record = {
        ...bio,
        description: description
          ? { plaintext: description, facets: bio.description?.facets }
          : undefined,
        socials: socials.length > 0 ? socials : undefined,
        updatedAt: new Date().toISOString(),
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

  const updateSocial = (
    idx: number,
    field: keyof PlaceStreamBioDefs.Social,
    value: string,
  ) => {
    const next = [...socials];
    next[idx] = { ...next[idx], [field]: value };
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
                    "Paste a pub.leaflet.document AT-URI or rkey to import your leaflet page into a Streamplace bio.",
                  )}
                </Text>
                <View
                  direction="row"
                  style={[zero.gap.all[2], { marginTop: 12 }]}
                >
                  <View style={{ flex: 1 }}>
                    <Input
                      placeholder="at://did:plc:.../pub.leaflet.document/abc123"
                      value={leafletSource}
                      onChangeText={setLeafletSource}
                    />
                  </View>
                  <Button
                    onPress={handleImport}
                    loading={importing}
                    disabled={!leafletSource.trim() || !did}
                  >
                    <Download size={16} style={{ marginRight: 4 }} />
                    {t("import", "Import")}
                  </Button>
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

            {bio && (
              <>
                <MenuGroup>
                  <View style={[zero.p[4]]}>
                    <Text size="xl" weight="bold">
                      {t("bio-preview", "Preview")}
                    </Text>
                  </View>
                  <BioViewer bio={bio} did={did} />
                </MenuGroup>

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
                      <Button variant="secondary" size="sm" onPress={addSocial}>
                        <Plus size={14} style={{ marginRight: 2 }} />
                        {t("add", "Add")}
                      </Button>
                    </View>
                    {socials.map((social, idx) => (
                      <View key={idx} style={{ marginBottom: 8 }}>
                        <View direction="row" style={[zero.gap.all[2]]}>
                          <View style={{ flex: 1 }}>
                            <Input
                              placeholder="URL"
                              value={social.url}
                              onChangeText={(v) => updateSocial(idx, "url", v)}
                            />
                          </View>
                          <View style={{ flex: 0.5 }}>
                            <Input
                              placeholder="Handle"
                              value={social.handle ?? ""}
                              onChangeText={(v) =>
                                updateSocial(idx, "handle", v)
                              }
                            />
                          </View>
                          <View style={{ flex: 0.5 }}>
                            <Input
                              placeholder="Platform"
                              value={social.platform}
                              onChangeText={(v) =>
                                updateSocial(idx, "platform", v)
                              }
                            />
                          </View>
                          <Button
                            variant="destructive"
                            size="sm"
                            onPress={() => removeSocial(idx)}
                          >
                            <Trash2 size={14} />
                          </Button>
                        </View>
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
              </>
            )}
          </MenuContainer>
        </View>
      </View>
    </ScrollView>
  );
}
