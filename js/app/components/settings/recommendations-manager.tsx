import {
  Button,
  Input,
  MenuContainer,
  MenuGroup,
  MenuInfo,
  MenuItem,
  MenuSeparator,
  ResponsiveDialog,
  Text,
  useToast,
  zero,
} from "@streamplace/components";
import { usePDSAgent } from "@streamplace/components/src/streamplace-store/xrpc";
import Loading from "components/loading/loading";
import {
  Check,
  GripVertical,
  Pencil,
  Plus,
  RefreshCw,
  Search,
  X,
} from "lucide-react-native";
import { useCallback, useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { Pressable, ScrollView, View } from "react-native";
import Sortable from "react-native-sortables";
import { SettingsRowItem } from "./components/settings-navigation-item";

const { text, mt, mb, px, py, w, layout, gap, r, p } = zero;

interface ActorSearchResult {
  did: string;
  handle: string;
}

export default function RecommendationsManager() {
  const agent = usePDSAgent();
  const { theme } = zero.useTheme();
  const [streamers, setStreamers] = useState<string[]>([]);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [deleteDialog, setDeleteDialog] = useState<{
    isVisible: boolean;
    index: number | null;
  }>({ isVisible: false, index: null });
  const [errors, setErrors] = useState<Record<number, string>>({});
  const [editingIndex, setEditingIndex] = useState<number | null>(null);
  const [editValue, setEditValue] = useState("");

  const a = useToast();

  const [activeDrag, setActiveDrag] = useState("");

  // Search state
  const [searchQuery, setSearchQuery] = useState("");
  const [searchResults, setSearchResults] = useState<ActorSearchResult[]>([]);
  const [searching, setSearching] = useState(false);
  const [searchDebounceTimeout, setSearchDebounceTimeout] =
    useState<NodeJS.Timeout | null>(null);

  const { t } = useTranslation("settings");

  const loadRecommendations = async () => {
    if (!agent) return;

    try {
      setLoading(true);
      const userDID = agent.did;
      if (!userDID) {
        setStreamers([]);
        return;
      }

      // Get the record directly from the PDS for editing
      const response = await agent.com.atproto.repo.getRecord({
        repo: userDID,
        collection: "place.stream.live.recommendations",
        rkey: "self",
      });

      // todo: type this right
      let record = response.data.value as any;

      if (!response.success) {
        // Create a new empty record if not found
        const res = await agent.com.atproto.repo.createRecord({
          repo: userDID,
          collection: "place.stream.live.recommendations",
          record: {
            streamers: [],
            createdAt: new Date().toISOString(),
          },
        });
        if (!res.success) {
          throw new Error("Failed to create recommendations record");
        }
        record = res.data;
      }
      setStreamers(record.streamers || []);
    } catch (error: any) {
      console.error("Failed to load recommendations:", error);
      if (error.status !== 404) {
        a.show("Error", "Failed to load recommendations. Please try again.");
      }
      setStreamers([]);
    } finally {
      setLoading(false);
    }
  };

  const saveRecommendations = async (newStreamers: string[]) => {
    if (!agent || saving) return;

    try {
      if (!agent.did) {
        throw new Error("User DID not found");
      }
      setSaving(true);

      // Use putRecord to create or update the record
      await agent.com.atproto.repo.putRecord({
        repo: agent.did,
        collection: "place.stream.live.recommendations",
        rkey: "self",
        record: {
          streamers: newStreamers,
          createdAt: new Date().toISOString(),
        },
      });

      setStreamers(newStreamers);
    } catch (error: any) {
      console.error("Failed to save recommendations:", error);
      a.show(
        "Error",
        error.message || "Failed to save recommendations. Please try again.",
      );
      // Reload to get back to consistent state
      await loadRecommendations();
    } finally {
      setSaving(false);
    }
  };

  const searchActors = useCallback(
    async (query: string) => {
      if (!agent || !query.trim()) {
        setSearchResults([]);
        return;
      }

      try {
        setSearching(true);
        const response = await agent.place.stream.live.searchActorsTypeahead({
          q: query,
          limit: 10,
        });

        setSearchResults(
          response.data.actors.map((actor: any) => ({
            did: actor.did,
            handle: actor.handle,
          })),
        );
      } catch (error: any) {
        console.error("Failed to search actors:", error);
        setSearchResults([]);
      } finally {
        setSearching(false);
      }
    },
    [agent],
  );

  const handleSearchChange = (query: string) => {
    setSearchQuery(query);

    // Clear previous timeout
    if (searchDebounceTimeout) {
      clearTimeout(searchDebounceTimeout);
    }

    // Set new timeout for debounced search
    if (query.trim()) {
      const timeout = setTimeout(() => {
        searchActors(query);
      }, 300);
      setSearchDebounceTimeout(timeout);
    } else {
      setSearchResults([]);
    }
  };

  const handleSelectActor = async (actor: ActorSearchResult) => {
    if (streamers.length >= 8) {
      a.show("Maximum Reached", "You can only add up to 8 recommendations.");
      return;
    }

    if (streamers.includes(actor.did)) {
      a.show(
        "Already Added",
        "This streamer is already in your recommendations.",
      );
      return;
    }

    const newStreamers = [...streamers, actor.did];
    await saveRecommendations(newStreamers);

    // Clear search
    setSearchQuery("");
    setSearchResults([]);
  };

  const validateDID = (did: string, index: number): boolean => {
    const trimmed = did.trim();
    if (!trimmed) {
      setErrors((prev) => ({ ...prev, [index]: "DID is required" }));
      return false;
    }
    if (!trimmed.startsWith("did:")) {
      setErrors((prev) => ({
        ...prev,
        [index]: "DID must start with 'did:'",
      }));
      return false;
    }
    setErrors((prev) => {
      const newErrors = { ...prev };
      delete newErrors[index];
      return newErrors;
    });
    return true;
  };

  const handleEdit = (index: number) => {
    setEditingIndex(index);
    setEditValue(streamers[index]);
    setErrors({});
  };

  const handleCancelEdit = () => {
    setEditingIndex(null);
    setEditValue("");
    setErrors({});
  };

  const handleSaveEdit = async () => {
    if (editingIndex === null) return;

    const trimmed = editValue.trim();
    if (!trimmed) {
      setErrors({ [editingIndex]: "DID is required" });
      return;
    }
    if (!trimmed.startsWith("did:")) {
      setErrors({ [editingIndex]: "DID must start with 'did:'" });
      return;
    }

    const newStreamers = [...streamers];
    newStreamers[editingIndex] = trimmed;
    await saveRecommendations(newStreamers);
    setEditingIndex(null);
    setEditValue("");
    setErrors({});
  };

  const handleAddRecommendation = () => {
    if (streamers.length >= 8) {
      a.show("Maximum Reached", "You can only add up to 8 recommendations.");
      return;
    }
    const newIndex = streamers.length;
    setStreamers([...streamers, ""]);
    setEditingIndex(newIndex);
    setEditValue("");
  };

  const handleDelete = (index: number) => {
    setDeleteDialog({ isVisible: true, index });
  };

  const confirmDelete = async () => {
    if (deleteDialog.index === null) return;

    const newStreamers = streamers.filter((_, i) => i !== deleteDialog.index);
    await saveRecommendations(newStreamers);
    setDeleteDialog({ isVisible: false, index: null });
  };

  useEffect(() => {
    if (!agent) return;
    loadRecommendations();
  }, [agent]);

  // Cleanup timeout on unmount
  useEffect(() => {
    return () => {
      if (searchDebounceTimeout) {
        clearTimeout(searchDebounceTimeout);
      }
    };
  }, [searchDebounceTimeout]);

  if (!agent) {
    return <Loading />;
  }

  return (
    <>
      <ScrollView>
        <View style={[zero.layout.flex.align.center, zero.px[2], zero.py[2]]}>
          <View style={{ maxWidth: 800, width: "100%" }}>
            <MenuContainer>
              <View style={[mb[2]]}>
                <View
                  style={[
                    layout.flex.row,
                    layout.flex.justify.between,
                    layout.flex.alignCenter,
                  ]}
                >
                  <Text size="xl">{t("recommendations-to-others")}</Text>
                  <Button
                    onPress={loadRecommendations}
                    disabled={loading || saving}
                    leftIcon={<RefreshCw size={16} color={theme.colors.text} />}
                    size="pill"
                    width="min"
                    variant="secondary"
                  >
                    <Text size="sm">{t("refresh")}</Text>
                  </Button>
                </View>
              </View>

              <MenuInfo description={t("recommendations-description")} />
            </MenuContainer>

            {/* Search Bar */}
            {streamers.length < 8 && (
              <MenuContainer>
                <MenuGroup>
                  <View style={[px[3], py[2]]}>
                    <View
                      style={[
                        layout.flex.row,
                        layout.flex.alignCenter,
                        gap.all[2],
                      ]}
                    >
                      <Search size={20} color={theme.colors.textMuted} />
                      <Input
                        value={searchQuery}
                        onChangeText={handleSearchChange}
                        placeholder="Search for streamers..."
                      />
                    </View>
                  </View>

                  {searching && (
                    <>
                      <MenuSeparator />
                      <View style={[py[2], layout.flex.center]}>
                        <Text
                          size="sm"
                          style={{ color: theme.colors.textMuted }}
                        >
                          Searching...
                        </Text>
                      </View>
                    </>
                  )}

                  {!searching && searchResults.length > 0 && (
                    <>
                      <MenuSeparator />
                      {searchResults.map((actor, index) => {
                        const alreadyAdded = streamers.includes(actor.did);
                        return (
                          <View key={actor.did}>
                            {index > 0 && <MenuSeparator />}
                            <Pressable
                              onPress={() =>
                                !alreadyAdded && handleSelectActor(actor)
                              }
                              disabled={alreadyAdded}
                            >
                              {({ pressed }) => (
                                <View
                                  style={[
                                    px[3],
                                    py[2],
                                    layout.flex.row,
                                    layout.flex.alignCenter,
                                    gap.all[2],
                                    r.md,
                                    {
                                      backgroundColor:
                                        pressed && !alreadyAdded
                                          ? "#ffffff08"
                                          : "transparent",
                                      opacity: alreadyAdded ? 0.5 : 1,
                                    },
                                  ]}
                                >
                                  <View style={{ flex: 1 }}>
                                    <Text>@{actor.handle}</Text>
                                  </View>
                                  {alreadyAdded && (
                                    <Text
                                      size="xs"
                                      style={{ color: theme.colors.textMuted }}
                                    >
                                      Added
                                    </Text>
                                  )}
                                </View>
                              )}
                            </Pressable>
                          </View>
                        );
                      })}
                    </>
                  )}

                  {!searching &&
                    searchQuery.trim() &&
                    searchResults.length === 0 && (
                      <>
                        <MenuSeparator />
                        <View style={[py[2], layout.flex.center]}>
                          <Text
                            size="sm"
                            style={{ color: theme.colors.textMuted }}
                          >
                            No results found
                          </Text>
                        </View>
                      </>
                    )}
                </MenuGroup>

                {searchQuery.trim() === "" && (
                  <MenuInfo description="Search for streamers by handle or name, or enter DIDs manually below" />
                )}
              </MenuContainer>
            )}

            {loading ? (
              <Loading />
            ) : (
              <MenuContainer>
                <MenuGroup>
                  {streamers.length === 0 ? (
                    <View style={[py[4], layout.flex.center]}>
                      <Text size="sm" style={{ color: theme.colors.textMuted }}>
                        {t("no-recommendations-yet")}
                      </Text>
                    </View>
                  ) : (
                    <Sortable.Grid
                      columns={1}
                      activeItemOpacity={90}
                      activeItemScale={1}
                      onActiveItemDropped={() => {
                        saveRecommendations(streamers);
                      }}
                      data={streamers}
                      keyExtractor={(item: string) => `item-${item}`}
                      overDrag="vertical"
                      onDragStart={(e) => {
                        console.log("dragging", e.key);
                        setActiveDrag(e.key);
                      }}
                      onDragEnd={() => setActiveDrag("")}
                      renderItem={(params: any) => {
                        const streamer: string = params.item;
                        const index: number = params.index ?? 0;
                        const beforeSeparator =
                          index > 0 && "item-" + params.item !== activeDrag ? (
                            <MenuSeparator key={`sep-${index}`} />
                          ) : null;

                        return (
                          <>
                            {beforeSeparator}
                            <MenuItem key={`item-${index}`}>
                              <GripVertical
                                color={theme.colors.mutedForeground + "a0"}
                                size={18}
                                style={{
                                  marginLeft: -4,
                                  marginRight: 4,
                                }}
                              />
                              {editingIndex === index ? (
                                <>
                                  <View style={{ flex: 1 }}>
                                    <Input
                                      value={editValue}
                                      onChangeText={setEditValue}
                                      placeholder="did:plc:..."
                                      autoFocus
                                    />
                                    {errors[index] && (
                                      <Text
                                        size="xs"
                                        style={{
                                          color: theme.colors.destructive,
                                          marginTop: 4,
                                        }}
                                      >
                                        {errors[index]}
                                      </Text>
                                    )}
                                  </View>

                                  <Pressable
                                    onPress={handleSaveEdit}
                                    style={({ pressed }) => [
                                      {
                                        padding: 8,
                                        borderRadius: 6,
                                        backgroundColor: pressed
                                          ? "#ffffff08"
                                          : "transparent",
                                      },
                                    ]}
                                  >
                                    <Check
                                      size={18}
                                      color={theme.colors.text}
                                    />
                                  </Pressable>

                                  <Pressable
                                    onPress={handleCancelEdit}
                                    style={({ pressed }) => [
                                      {
                                        padding: 8,
                                        borderRadius: 6,
                                        backgroundColor: pressed
                                          ? "#ffffff08"
                                          : "transparent",
                                      },
                                    ]}
                                  >
                                    <X
                                      size={18}
                                      color={theme.colors.textMuted}
                                    />
                                  </Pressable>
                                </>
                              ) : (
                                <>
                                  <View style={{ flex: 1 }}>
                                    <Text
                                      numberOfLines={1}
                                      ellipsizeMode="middle"
                                    >
                                      {streamer || "(empty)"}
                                    </Text>
                                  </View>

                                  <Pressable
                                    onPress={() => handleEdit(index)}
                                    style={({ pressed }) => [
                                      {
                                        padding: 8,
                                        borderRadius: 6,
                                        backgroundColor: pressed
                                          ? "#ffffff08"
                                          : "transparent",
                                      },
                                    ]}
                                  >
                                    <Pencil
                                      size={18}
                                      color={theme.colors.textMuted}
                                    />
                                  </Pressable>

                                  <Pressable
                                    onPress={() => handleDelete(index)}
                                    style={({ pressed }) => [
                                      {
                                        padding: 8,
                                        borderRadius: 6,
                                        backgroundColor: pressed
                                          ? "#ffffff08"
                                          : "transparent",
                                      },
                                    ]}
                                  >
                                    <X
                                      size={18}
                                      color={theme.colors.destructive}
                                    />
                                  </Pressable>
                                </>
                              )}
                            </MenuItem>
                          </>
                        );
                      }}
                      onOrderChange={(params) => {
                        console.log(params);
                        // calculate new order from params
                        // duplicate streamers array
                        const newData = [...streamers];
                        const movedItem = newData.splice(
                          params.fromIndex,
                          1,
                        )[0];
                        newData.splice(params.toIndex, 0, movedItem);
                        setStreamers(newData);
                      }}
                      rowGap={0}
                      columnGap={0}
                    />
                  )}

                  {streamers.length > 0 && streamers.length < 8 && (
                    <MenuSeparator />
                  )}

                  {streamers.length < 8 && (
                    <SettingsRowItem onPress={handleAddRecommendation}>
                      <View
                        style={[
                          layout.flex.row,
                          layout.flex.alignCenter,
                          gap.all[2],
                        ]}
                      >
                        <Plus color={theme.colors.text} />
                        <Text>Add DID manually</Text>
                      </View>
                    </SettingsRowItem>
                  )}

                  {saving && (
                    <View style={[mt[2], layout.flex.center]}>
                      <Text size="sm" style={{ color: theme.colors.textMuted }}>
                        {t("saving")}
                      </Text>
                    </View>
                  )}
                </MenuGroup>
              </MenuContainer>
            )}
          </View>
        </View>
      </ScrollView>

      <ResponsiveDialog
        open={deleteDialog.isVisible}
        onOpenChange={(open) =>
          !open && setDeleteDialog({ isVisible: false, index: null })
        }
        title={t("delete")}
        dismissible={true}
        size="sm"
      >
        <View style={[w.percent[100], gap.all[2], mb[4]]}>
          <Text size="2xl">{t("confirm-delete")}</Text>
          <Text>{t("action-cannot-be-undone")}</Text>
        </View>

        <View style={[layout.flex.row, layout.flex.justify.end, gap.all[3]]}>
          <Button
            variant="secondary"
            width="min"
            onPress={() => setDeleteDialog({ isVisible: false, index: null })}
            disabled={saving}
          >
            <Text>{t("cancel")}</Text>
          </Button>
          <Button
            variant="destructive"
            width="min"
            onPress={confirmDelete}
            disabled={saving}
          >
            <Text>{saving ? t("deleting") : t("delete")}</Text>
          </Button>
        </View>
      </ResponsiveDialog>
    </>
  );
}
