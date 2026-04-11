import {
  Button,
  Dialog,
  DialogFooter,
  Input,
  MenuContainer,
  MenuGroup,
  MenuSeparator,
  ResponsiveDialog,
  Text,
  zero,
} from "@streamplace/components";
import { Select } from "@streamplace/components/src/components/ui/select";
import { usePDSAgent } from "@streamplace/components/src/streamplace-store/xrpc";
import Loading from "components/loading/loading";
import { Pencil, Plus, Search, Share2, Trash2, X } from "lucide-react-native";
import { useCallback, useEffect, useRef, useState } from "react";
import {
  Alert,
  Image,
  Platform,
  Pressable,
  ScrollView,
  Switch,
  View,
} from "react-native";
import { useStore } from "store";
import { useOAuthSession } from "store/hooks";
import { PlaceStreamEmoteItem, PlaceStreamEmotePack } from "streamplace";
import { SettingsRowItem } from "./components/settings-navigation-item";

const { text, mb, mt, gap, layout, w } = zero;

interface PackRecord {
  uri: string;
  cid: string;
  value: PlaceStreamEmotePack.Record;
}

interface EmoteRecord {
  uri: string;
  cid: string;
  value: PlaceStreamEmoteItem.Record;
}

interface ActorSearchResult {
  did: string;
  handle: string;
}

function emoteImageUrl(did: string, item: PlaceStreamEmoteItem.Record): string {
  const cid = item.image.toJSON().ref.$link ?? "";
  return `https://cdn.bsky.app/img/feed_fullsize/plain/${did}/${cid}@png`;
}

function CreatePackDialog({
  isVisible,
  onClose,
  onSubmit,
  isLoading,
}: {
  isVisible: boolean;
  onClose: () => void;
  onSubmit: (name: string, description: string, openInMyChat: boolean) => void;
  isLoading: boolean;
}) {
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [openInMyChat, setOpenInMyChat] = useState(false);

  const handleSubmit = () => {
    if (!name.trim()) return;
    onSubmit(name.trim(), description.trim(), openInMyChat);
  };

  const handleClose = () => {
    setName("");
    setDescription("");
    setOpenInMyChat(false);
    onClose();
  };

  return (
    <ResponsiveDialog
      open={isVisible}
      onOpenChange={(open) => !open && handleClose()}
      title="Create Emote Pack"
      dismissible={false}
    >
      <View style={[w.percent[100]]}>
        <View style={[mb[4]]}>
          <Text
            style={[text.gray[300], mb[2], { fontSize: 14, fontWeight: "500" }]}
          >
            Name *
          </Text>
          <Input
            value={name}
            onChangeText={setName}
            placeholder="My Emote Pack"
          />
        </View>
        <View style={[mb[4]]}>
          <Text
            style={[text.gray[300], mb[2], { fontSize: 14, fontWeight: "500" }]}
          >
            Description (optional)
          </Text>
          <Input
            value={description}
            onChangeText={setDescription}
            placeholder="A collection of custom emotes"
          />
        </View>
        <View
          style={[
            mb[4],
            {
              flexDirection: "row",
              alignItems: "center",
              justifyContent: "space-between",
            },
          ]}
        >
          <View style={{ flex: 1, marginRight: 12 }}>
            <Text style={[{ fontSize: 14, fontWeight: "500" }]}>
              Open in my chat
            </Text>
            <Text size="sm" muted>
              Allow followers to use this pack in your stream chat
            </Text>
          </View>
          <Switch value={openInMyChat} onValueChange={setOpenInMyChat} />
        </View>
      </View>
      <DialogFooter>
        <Button
          width="min"
          variant="secondary"
          onPress={handleClose}
          disabled={isLoading}
        >
          <Text>Cancel</Text>
        </Button>
        <Button
          width="min"
          onPress={handleSubmit}
          disabled={isLoading || !name.trim()}
        >
          <Text>{isLoading ? "Creating..." : "Create"}</Text>
        </Button>
      </DialogFooter>
    </ResponsiveDialog>
  );
}

function EditPackDialog({
  isVisible,
  onClose,
  onSubmit,
  isLoading,
  pack,
}: {
  isVisible: boolean;
  onClose: () => void;
  onSubmit: (name: string, description: string, openInMyChat: boolean) => void;
  isLoading: boolean;
  pack: PackRecord | null;
}) {
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [openInMyChat, setOpenInMyChat] = useState(false);

  useEffect(() => {
    if (!pack) return;
    setName(pack.value.name);
    setDescription(pack.value.description ?? "");
    setOpenInMyChat(pack.value.openInMyChat ?? false);
  }, [pack]);

  const handleSubmit = () => {
    if (!name.trim()) return;
    onSubmit(name.trim(), description.trim(), openInMyChat);
  };

  return (
    <ResponsiveDialog
      open={isVisible}
      onOpenChange={(open) => !open && onClose()}
      title="Edit Emote Pack"
      dismissible={false}
    >
      <View style={[w.percent[100]]}>
        <View style={[mb[4]]}>
          <Text
            style={[text.gray[300], mb[2], { fontSize: 14, fontWeight: "500" }]}
          >
            Name *
          </Text>
          <Input
            value={name}
            onChangeText={setName}
            placeholder="My Emote Pack"
          />
        </View>
        <View style={[mb[4]]}>
          <Text
            style={[text.gray[300], mb[2], { fontSize: 14, fontWeight: "500" }]}
          >
            Description (optional)
          </Text>
          <Input
            value={description}
            onChangeText={setDescription}
            placeholder="A collection of custom emotes"
          />
        </View>
        <View
          style={[
            mb[4],
            {
              flexDirection: "row",
              alignItems: "center",
              justifyContent: "space-between",
            },
          ]}
        >
          <View style={{ flex: 1, marginRight: 12 }}>
            <Text style={[{ fontSize: 14, fontWeight: "500" }]}>
              Open in my chat
            </Text>
            <Text size="sm" muted>
              Allow followers to use this pack in your stream chat
            </Text>
          </View>
          <Switch value={openInMyChat} onValueChange={setOpenInMyChat} />
        </View>
      </View>
      <DialogFooter>
        <Button
          width="min"
          variant="secondary"
          onPress={onClose}
          disabled={isLoading}
        >
          <Text>Cancel</Text>
        </Button>
        <Button
          width="min"
          onPress={handleSubmit}
          disabled={isLoading || !name.trim()}
        >
          <Text>{isLoading ? "Saving..." : "Save"}</Text>
        </Button>
      </DialogFooter>
    </ResponsiveDialog>
  );
}

function CreateEmoteDialog({
  isVisible,
  onClose,
  onSubmit,
  isLoading,
}: {
  isVisible: boolean;
  onClose: () => void;
  onSubmit: (
    name: string,
    imageBlob: Blob,
    alt: string,
    creator?: string,
  ) => void;
  isLoading: boolean;
}) {
  const agent = usePDSAgent();
  const { theme } = zero.useTheme();
  const [name, setName] = useState("");
  const [alt, setAlt] = useState("");
  const [imageBlob, setImageBlob] = useState<Blob | null>(null);
  const [imagePreview, setImagePreview] = useState<string | null>(null);
  const fileInputRef = useRef<HTMLInputElement | null>(null);
  const [creator, setCreator] = useState<string | null>(null);
  const [creatorHandle, setCreatorHandle] = useState<string | null>(null);
  const [creatorSearch, setCreatorSearch] = useState("");
  const [creatorResults, setCreatorResults] = useState<ActorSearchResult[]>([]);
  const [creatorSearching, setCreatorSearching] = useState(false);
  const creatorDebounceRef = useRef<NodeJS.Timeout | null>(null);

  const handleFileChange = useCallback(
    (event: React.ChangeEvent<HTMLInputElement>) => {
      const file = event.target.files?.[0];
      if (!file) return;
      const blob = new Blob([file], { type: file.type });
      setImageBlob(blob);
      setImagePreview(URL.createObjectURL(blob));
      event.target.value = "";
    },
    [],
  );

  const handleCreatorSearchChange = (query: string) => {
    setCreatorSearch(query);
    if (creatorDebounceRef.current) clearTimeout(creatorDebounceRef.current);
    if (!query.trim()) {
      setCreatorResults([]);
      return;
    }
    creatorDebounceRef.current = setTimeout(async () => {
      if (!agent) return;
      try {
        setCreatorSearching(true);
        const response = await agent.place.stream.live.searchActorsTypeahead({
          q: query,
          limit: 5,
        });
        setCreatorResults(
          response.data.actors.map((a: any) => ({
            did: a.did,
            handle: a.handle,
          })),
        );
      } catch {
        setCreatorResults([]);
      } finally {
        setCreatorSearching(false);
      }
    }, 300);
  };

  const selectCreator = (actor: ActorSearchResult) => {
    setCreator(actor.did);
    setCreatorHandle(actor.handle);
    setCreatorSearch("");
    setCreatorResults([]);
  };

  const clearCreator = () => {
    setCreator(null);
    setCreatorHandle(null);
    setCreatorSearch("");
    setCreatorResults([]);
  };

  const handleSubmit = () => {
    if (!name.trim() || !imageBlob) return;
    onSubmit(name.trim(), imageBlob, alt.trim(), creator ?? undefined);
  };

  const handleClose = () => {
    setName("");
    setAlt("");
    if (imagePreview) URL.revokeObjectURL(imagePreview);
    setImageBlob(null);
    setImagePreview(null);
    setCreator(null);
    setCreatorHandle(null);
    setCreatorSearch("");
    setCreatorResults([]);
    onClose();
  };

  const isWeb = Platform.OS === "web";

  return (
    <Dialog
      open={isVisible}
      onOpenChange={(open) => !open && handleClose()}
      title="Add Emote"
      dismissible={false}
    >
      <View style={[w.percent[100]]}>
        <View style={[mb[4]]}>
          <Text
            style={[text.gray[300], mb[2], { fontSize: 14, fontWeight: "500" }]}
          >
            Name *
          </Text>
          <Input
            value={name}
            onChangeText={setName}
            placeholder="my_emote"
            autoCapitalize="none"
          />
          <Text size="sm" muted style={[mt[1]]}>
            Alphanumeric and underscores only
          </Text>
        </View>

        <View style={[mb[4]]}>
          <Text
            style={[text.gray[300], mb[2], { fontSize: 14, fontWeight: "500" }]}
          >
            Image *
          </Text>
          {isWeb ? (
            <View>
              {imagePreview ? (
                <View style={[mb[2]]}>
                  <Image
                    source={{ uri: imagePreview }}
                    style={{ width: 64, height: 64, borderRadius: 6 }}
                    resizeMode="contain"
                  />
                </View>
              ) : null}
              <Button
                width="min"
                variant="secondary"
                onPress={() => fileInputRef.current?.click()}
              >
                <Text>{imagePreview ? "Change Image" : "Choose Image"}</Text>
              </Button>
              <input
                type="file"
                accept="image/png,image/gif,image/webp,image/avif"
                ref={fileInputRef}
                onChange={handleFileChange}
                style={{ display: "none" }}
              />
            </View>
          ) : (
            <Text size="sm" muted>
              Image upload is only available on web.
            </Text>
          )}
        </View>

        <View style={[mb[4]]}>
          <Text
            style={[text.gray[300], mb[2], { fontSize: 14, fontWeight: "500" }]}
          >
            Alt text (optional)
          </Text>
          <Input
            value={alt}
            onChangeText={setAlt}
            placeholder="Description of the emote"
          />
        </View>

        <View style={[mb[4]]}>
          <Text
            style={[text.gray[300], mb[2], { fontSize: 14, fontWeight: "500" }]}
          >
            Creator (optional)
          </Text>
          {creator ? (
            <View
              style={[layout.flex.row, layout.flex.alignCenter, gap.all[2]]}
            >
              <View style={{ flex: 1 }}>
                <Text>@{creatorHandle ?? creator}</Text>
              </View>
              <Button
                width="min"
                variant="secondary"
                size="pill"
                onPress={clearCreator}
                leftIcon={<X size={14} color={theme.colors.text} />}
              >
                <Text>Clear</Text>
              </Button>
            </View>
          ) : (
            <View>
              <View
                style={[layout.flex.row, layout.flex.alignCenter, gap.all[2]]}
              >
                <Search size={16} color={theme.colors.textMuted} />
                <Input
                  value={creatorSearch}
                  onChangeText={handleCreatorSearchChange}
                  placeholder="Search by handle..."
                />
              </View>
              {creatorSearching && (
                <Text size="sm" muted style={[mt[1]]}>
                  Searching...
                </Text>
              )}
              {!creatorSearching && creatorResults.length > 0 && (
                <View style={[mt[1], { borderRadius: 6, overflow: "hidden" }]}>
                  {creatorResults.map((actor) => (
                    <Pressable
                      key={actor.did}
                      onPress={() => selectCreator(actor)}
                    >
                      {({ pressed }) => (
                        <View
                          style={[
                            zero.px[3],
                            zero.py[2],
                            {
                              backgroundColor: pressed
                                ? "#ffffff10"
                                : "#ffffff08",
                            },
                          ]}
                        >
                          <Text>@{actor.handle}</Text>
                        </View>
                      )}
                    </Pressable>
                  ))}
                </View>
              )}
              {!creatorSearching &&
                creatorSearch.trim() &&
                creatorResults.length === 0 && (
                  <Text size="sm" muted style={[mt[1]]}>
                    No results found
                  </Text>
                )}
            </View>
          )}
        </View>
      </View>

      <DialogFooter>
        <Button
          width="min"
          variant="secondary"
          onPress={handleClose}
          disabled={isLoading}
        >
          <Text>Cancel</Text>
        </Button>
        <Button
          width="min"
          onPress={handleSubmit}
          disabled={isLoading || !name.trim() || !imageBlob}
        >
          <Text>{isLoading ? "Adding..." : "Add Emote"}</Text>
        </Button>
      </DialogFooter>
    </Dialog>
  );
}

function EditEmoteDialog({
  isVisible,
  onClose,
  onSubmit,
  isLoading,
  emote,
}: {
  isVisible: boolean;
  onClose: () => void;
  onSubmit: (name: string, alt: string, creator?: string) => void;
  isLoading: boolean;
  emote: EmoteRecord | null;
}) {
  const agent = usePDSAgent();
  const { theme } = zero.useTheme();
  const [name, setName] = useState("");
  const [alt, setAlt] = useState("");
  const [creator, setCreator] = useState<string | null>(null);
  const [creatorHandle, setCreatorHandle] = useState<string | null>(null);
  const [creatorSearch, setCreatorSearch] = useState("");
  const [creatorResults, setCreatorResults] = useState<ActorSearchResult[]>([]);
  const [creatorSearching, setCreatorSearching] = useState(false);
  const creatorDebounceRef = useRef<NodeJS.Timeout | null>(null);

  useEffect(() => {
    if (!emote) return;
    setName(emote.value.name);
    setAlt(emote.value.alt ?? "");
    setCreatorSearch("");
    setCreatorResults([]);
    const creatorDid = emote.value.creator ?? null;
    setCreator(creatorDid);
    setCreatorHandle(null);
    if (creatorDid && agent) {
      agent.app.bsky.actor
        .getProfile({ actor: creatorDid })
        .then((res) => setCreatorHandle(res.data.handle ?? null))
        .catch(() => {});
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [emote]);

  const handleCreatorSearchChange = (query: string) => {
    setCreatorSearch(query);
    if (creatorDebounceRef.current) clearTimeout(creatorDebounceRef.current);
    if (!query.trim()) {
      setCreatorResults([]);
      return;
    }
    creatorDebounceRef.current = setTimeout(async () => {
      if (!agent) return;
      try {
        setCreatorSearching(true);
        const response = await agent.place.stream.live.searchActorsTypeahead({
          q: query,
          limit: 5,
        });
        setCreatorResults(
          response.data.actors.map((a: any) => ({
            did: a.did,
            handle: a.handle,
          })),
        );
      } catch {
        setCreatorResults([]);
      } finally {
        setCreatorSearching(false);
      }
    }, 300);
  };

  const selectCreator = (actor: ActorSearchResult) => {
    setCreator(actor.did);
    setCreatorHandle(actor.handle);
    setCreatorSearch("");
    setCreatorResults([]);
  };

  const clearCreator = () => {
    setCreator(null);
    setCreatorHandle(null);
    setCreatorSearch("");
    setCreatorResults([]);
  };

  const handleSubmit = () => {
    if (!name.trim()) return;
    onSubmit(name.trim(), alt.trim(), creator ?? undefined);
  };

  return (
    <Dialog
      open={isVisible}
      onOpenChange={(open) => !open && onClose()}
      title="Edit Emote"
      dismissible={false}
    >
      <View style={[w.percent[100]]}>
        <View style={[mb[4]]}>
          <Text
            style={[text.gray[300], mb[2], { fontSize: 14, fontWeight: "500" }]}
          >
            Name *
          </Text>
          <Input
            value={name}
            onChangeText={setName}
            placeholder="my_emote"
            autoCapitalize="none"
          />
          <Text size="sm" muted style={[mt[1]]}>
            Alphanumeric and underscores only
          </Text>
        </View>

        <View style={[mb[4]]}>
          <Text
            style={[text.gray[300], mb[2], { fontSize: 14, fontWeight: "500" }]}
          >
            Alt text (optional)
          </Text>
          <Input
            value={alt}
            onChangeText={setAlt}
            placeholder="Description of the emote"
          />
        </View>

        <View style={[mb[4]]}>
          <Text
            style={[text.gray[300], mb[2], { fontSize: 14, fontWeight: "500" }]}
          >
            Creator (optional)
          </Text>
          {creator ? (
            <View
              style={[layout.flex.row, layout.flex.alignCenter, gap.all[2]]}
            >
              <View style={{ flex: 1 }}>
                <Text numberOfLines={1} ellipsizeMode="middle">
                  @{creatorHandle ?? creator}
                </Text>
              </View>
              <Button
                width="min"
                variant="secondary"
                size="pill"
                onPress={clearCreator}
                leftIcon={<X size={14} color={theme.colors.text} />}
              >
                <Text>Clear</Text>
              </Button>
            </View>
          ) : (
            <View>
              <View
                style={[layout.flex.row, layout.flex.alignCenter, gap.all[2]]}
              >
                <Search size={16} color={theme.colors.textMuted} />
                <Input
                  value={creatorSearch}
                  onChangeText={handleCreatorSearchChange}
                  placeholder="Search by handle..."
                />
              </View>
              {creatorSearching && (
                <Text size="sm" muted style={[mt[1]]}>
                  Searching...
                </Text>
              )}
              {!creatorSearching && creatorResults.length > 0 && (
                <View style={[mt[1], { borderRadius: 6, overflow: "hidden" }]}>
                  {creatorResults.map((actor) => (
                    <Pressable
                      key={actor.did}
                      onPress={() => selectCreator(actor)}
                    >
                      {({ pressed }) => (
                        <View
                          style={[
                            zero.px[3],
                            zero.py[2],
                            {
                              backgroundColor: pressed
                                ? "#ffffff10"
                                : "#ffffff08",
                            },
                          ]}
                        >
                          <Text>@{actor.handle}</Text>
                        </View>
                      )}
                    </Pressable>
                  ))}
                </View>
              )}
              {!creatorSearching &&
                creatorSearch.trim() &&
                creatorResults.length === 0 && (
                  <Text size="sm" muted style={[mt[1]]}>
                    No results found
                  </Text>
                )}
            </View>
          )}
        </View>
      </View>

      <DialogFooter>
        <Button
          width="min"
          variant="secondary"
          onPress={onClose}
          disabled={isLoading}
        >
          <Text>Cancel</Text>
        </Button>
        <Button
          width="min"
          onPress={handleSubmit}
          disabled={isLoading || !name.trim()}
        >
          <Text>{isLoading ? "Saving..." : "Save"}</Text>
        </Button>
      </DialogFooter>
    </Dialog>
  );
}

function DelegatePackDialog({
  isVisible,
  onClose,
  onSubmit,
  isLoading,
  pack,
}: {
  isVisible: boolean;
  onClose: () => void;
  onSubmit: (recipientDID: string) => void;
  isLoading: boolean;
  pack: PackRecord | null;
}) {
  const agent = usePDSAgent();
  const { theme } = zero.useTheme();
  const [recipientDID, setRecipientDID] = useState<string | null>(null);
  const [recipientHandle, setRecipientHandle] = useState<string | null>(null);
  const [search, setSearch] = useState("");
  const [results, setResults] = useState<ActorSearchResult[]>([]);
  const [searching, setSearching] = useState(false);
  const debounceRef = useRef<NodeJS.Timeout | null>(null);

  const handleSearchChange = (query: string) => {
    setSearch(query);
    if (debounceRef.current) clearTimeout(debounceRef.current);
    if (!query.trim()) {
      setResults([]);
      return;
    }
    debounceRef.current = setTimeout(async () => {
      if (!agent) return;
      try {
        setSearching(true);
        const response = await agent.place.stream.live.searchActorsTypeahead({
          q: query,
          limit: 5,
        });
        setResults(
          response.data.actors.map((a: any) => ({
            did: a.did,
            handle: a.handle,
          })),
        );
      } catch {
        setResults([]);
      } finally {
        setSearching(false);
      }
    }, 300);
  };

  const selectRecipient = (actor: ActorSearchResult) => {
    setRecipientDID(actor.did);
    setRecipientHandle(actor.handle);
    setSearch("");
    setResults([]);
  };

  const clearRecipient = () => {
    setRecipientDID(null);
    setRecipientHandle(null);
    setSearch("");
    setResults([]);
  };

  const handleClose = () => {
    clearRecipient();
    onClose();
  };

  const handleSubmit = () => {
    if (!recipientDID) return;
    onSubmit(recipientDID);
  };

  return (
    <ResponsiveDialog
      open={isVisible}
      onOpenChange={(open) => !open && handleClose()}
      title="Delegate Pack"
      dismissible={false}
    >
      <View style={[w.percent[100]]}>
        {pack && (
          <Text style={[text.gray[400], mb[4], { fontSize: 14 }]}>
            Grant a user global access to use emotes from "{pack.value.name}".
          </Text>
        )}
        <View style={[mb[4]]}>
          <Text
            style={[text.gray[300], mb[2], { fontSize: 14, fontWeight: "500" }]}
          >
            User *
          </Text>
          {recipientDID ? (
            <View
              style={[layout.flex.row, layout.flex.alignCenter, gap.all[2]]}
            >
              <View style={{ flex: 1 }}>
                <Text numberOfLines={1} ellipsizeMode="middle">
                  @{recipientHandle ?? recipientDID}
                </Text>
              </View>
              <Button
                width="min"
                variant="secondary"
                size="pill"
                onPress={clearRecipient}
                leftIcon={<X size={14} color={theme.colors.text} />}
              >
                <Text>Clear</Text>
              </Button>
            </View>
          ) : (
            <View>
              <Input
                value={search}
                onChangeText={handleSearchChange}
                placeholder="Search by handle..."
                autoCapitalize="none"
              />
              {searching && (
                <View style={[mt[2]]}>
                  <Loading />
                </View>
              )}
              {results.length > 0 && (
                <View style={[mt[1], { borderRadius: 6, overflow: "hidden" }]}>
                  <MenuGroup>
                    {results.map((actor, i) => (
                      <View key={actor.did}>
                        {i > 0 && <MenuSeparator />}
                        <Pressable
                          onPress={() => selectRecipient(actor)}
                          style={({ pressed }) => ({
                            padding: 10,
                            backgroundColor: pressed
                              ? "#ffffff08"
                              : "transparent",
                          })}
                        >
                          <Text>@{actor.handle}</Text>
                        </Pressable>
                      </View>
                    ))}
                  </MenuGroup>
                </View>
              )}
              {!searching && search.trim() && results.length === 0 && (
                <Text size="sm" muted style={[mt[1]]}>
                  No results found
                </Text>
              )}
            </View>
          )}
        </View>
      </View>
      <DialogFooter>
        <Button
          width="min"
          variant="secondary"
          onPress={handleClose}
          disabled={isLoading}
        >
          <Text>Cancel</Text>
        </Button>
        <Button
          width="min"
          onPress={handleSubmit}
          disabled={isLoading || !recipientDID}
        >
          <Text>{isLoading ? "Delegating..." : "Delegate"}</Text>
        </Button>
      </DialogFooter>
    </ResponsiveDialog>
  );
}

export default function EmotePackManager() {
  const pdsAgent = useStore((state) => state.pdsAgent);
  const session = useOAuthSession();
  const { theme } = zero.useTheme();

  const [packs, setPacks] = useState<PackRecord[] | null>(null);
  const [selectedPackUri, setSelectedPackUri] = useState<string | null>(null);
  const [emotes, setEmotes] = useState<EmoteRecord[] | null>(null);
  const [loadingPacks, setLoadingPacks] = useState(true);
  const [loadingEmotes, setLoadingEmotes] = useState(false);
  const [showCreatePack, setShowCreatePack] = useState(false);
  const [creatingPack, setCreatingPack] = useState(false);
  const [showCreateEmote, setShowCreateEmote] = useState(false);
  const [creatingEmote, setCreatingEmote] = useState(false);
  const [editingPack, setEditingPack] = useState(false);
  const [editPackDialog, setEditPackDialog] = useState<{
    isVisible: boolean;
    pack: PackRecord | null;
  }>({ isVisible: false, pack: null });
  const [editingEmote, setEditingEmote] = useState(false);
  const [editEmoteDialog, setEditEmoteDialog] = useState<{
    isVisible: boolean;
    emote: EmoteRecord | null;
  }>({ isVisible: false, emote: null });
  const [deletingEmotes, setDeletingEmotes] = useState<Set<string>>(new Set());
  const [deletePackDialog, setDeletePackDialog] = useState<{
    isVisible: boolean;
    pack: PackRecord | null;
  }>({ isVisible: false, pack: null });
  const [deleteEmoteDialog, setDeleteEmoteDialog] = useState<{
    isVisible: boolean;
    emote: EmoteRecord | null;
  }>({ isVisible: false, emote: null });
  const [delegating, setDelegating] = useState(false);
  const [delegatePackDialog, setDelegatePackDialog] = useState<{
    isVisible: boolean;
    pack: PackRecord | null;
  }>({ isVisible: false, pack: null });

  const loadPacks = async () => {
    if (!pdsAgent || !session?.did) return;
    try {
      setLoadingPacks(true);
      const result = await pdsAgent.com.atproto.repo.listRecords({
        repo: session.did,
        collection: "place.stream.emote.pack",
        limit: 100,
      });
      const loaded = (result.data.records as PackRecord[]).map((r) => ({
        uri: r.uri,
        cid: r.cid,
        value: r.value as PlaceStreamEmotePack.Record,
      }));
      setPacks(loaded);
      if (loaded.length > 0 && !selectedPackUri) {
        setSelectedPackUri(loaded[0].uri);
      }
    } catch (err) {
      console.error("Failed to load emote packs", err);
      Alert.alert("Error", "Failed to load emote packs.");
    } finally {
      setLoadingPacks(false);
    }
  };

  const loadEmotes = async (packUri: string) => {
    if (!pdsAgent || !session?.did) return;
    try {
      setLoadingEmotes(true);
      const result = await pdsAgent.com.atproto.repo.listRecords({
        repo: session.did,
        collection: "place.stream.emote.item",
        limit: 100,
      });
      const all = (result.data.records as EmoteRecord[]).map((r) => ({
        uri: r.uri,
        cid: r.cid,
        value: r.value as PlaceStreamEmoteItem.Record,
      }));
      setEmotes(all.filter((e) => e.value.pack === packUri));
    } catch (err) {
      console.error("Failed to load emotes", err);
      Alert.alert("Error", "Failed to load emotes.");
    } finally {
      setLoadingEmotes(false);
    }
  };

  const createPack = async (
    name: string,
    description: string,
    openInMyChat: boolean,
  ) => {
    if (!pdsAgent || !session?.did) return;
    try {
      setCreatingPack(true);
      const result = await pdsAgent.com.atproto.repo.createRecord({
        repo: session.did,
        collection: "place.stream.emote.pack",
        record: {
          $type: "place.stream.emote.pack",
          name,
          ...(description ? { description } : {}),
          ...(openInMyChat ? { openInMyChat: true } : {}),
          createdAt: new Date().toISOString(),
        },
      });
      setShowCreatePack(false);
      await loadPacks();
      setSelectedPackUri(result.data.uri);
    } catch (err: any) {
      console.error("Failed to create emote pack", err);
      Alert.alert("Error", err.message ?? "Failed to create emote pack.");
    } finally {
      setCreatingPack(false);
    }
  };

  const editPack = async (
    name: string,
    description: string,
    openInMyChat: boolean,
  ) => {
    if (!pdsAgent || !session?.did || !editPackDialog.pack) return;
    const pack = editPackDialog.pack;
    const rkey = pack.uri.split("/").pop() ?? "";
    try {
      setEditingPack(true);
      await pdsAgent.com.atproto.repo.putRecord({
        repo: session.did,
        collection: "place.stream.emote.pack",
        rkey,
        record: {
          $type: "place.stream.emote.pack",
          name,
          ...(description ? { description } : {}),
          ...(openInMyChat ? { openInMyChat: true } : {}),
          createdAt: pack.value.createdAt,
        },
      });
      setEditPackDialog({ isVisible: false, pack: null });
      await loadPacks();
    } catch (err: any) {
      console.error("Failed to edit emote pack", err);
      Alert.alert("Error", err.message ?? "Failed to edit emote pack.");
    } finally {
      setEditingPack(false);
    }
  };

  const delegatePack = async (recipientDID: string) => {
    if (!pdsAgent || !session?.did || !delegatePackDialog.pack) return;
    const pack = delegatePackDialog.pack;
    try {
      setDelegating(true);
      await pdsAgent.com.atproto.repo.createRecord({
        repo: session.did,
        collection: "place.stream.emote.packDelegation",
        record: {
          $type: "place.stream.emote.packDelegation",
          did: recipientDID,
          pack: { uri: pack.uri, cid: pack.cid },
          createdAt: new Date().toISOString(),
        },
      });
      setDelegatePackDialog({ isVisible: false, pack: null });
    } catch (err: any) {
      console.error("Failed to delegate pack", err);
      Alert.alert("Error", err.message ?? "Failed to delegate pack.");
    } finally {
      setDelegating(false);
    }
  };

  const createEmote = async (
    name: string,
    imageBlob: Blob,
    alt: string,
    creator?: string,
  ) => {
    if (!pdsAgent || !session?.did || !selectedPackUri) return;
    try {
      setCreatingEmote(true);
      const uploadResult = await (pdsAgent as any).uploadBlob(imageBlob, {
        headers: { "Content-Type": imageBlob.type },
      });
      await pdsAgent.com.atproto.repo.createRecord({
        repo: session.did,
        collection: "place.stream.emote.item",
        record: {
          $type: "place.stream.emote.item",
          name,
          image: uploadResult.data.blob,
          pack: selectedPackUri,
          ...(alt ? { alt } : {}),
          ...(creator ? { creator } : {}),
          createdAt: new Date().toISOString(),
        },
      });
      setShowCreateEmote(false);
      await loadEmotes(selectedPackUri);
    } catch (err: any) {
      console.error("Failed to create emote", err);
      Alert.alert("Error", err.message ?? "Failed to create emote.");
    } finally {
      setCreatingEmote(false);
    }
  };

  const editEmote = async (name: string, alt: string, creator?: string) => {
    if (!pdsAgent || !session?.did || !editEmoteDialog.emote) return;
    const emote = editEmoteDialog.emote;
    const rkey = emote.uri.split("/").pop() ?? "";
    try {
      setEditingEmote(true);
      await pdsAgent.com.atproto.repo.putRecord({
        repo: session.did,
        collection: "place.stream.emote.item",
        rkey,
        record: {
          $type: "place.stream.emote.item",
          name,
          image: emote.value.image,
          pack: emote.value.pack,
          ...(alt ? { alt } : {}),
          ...(creator ? { creator } : {}),
          createdAt: emote.value.createdAt,
        },
      });
      setEditEmoteDialog({ isVisible: false, emote: null });
      if (selectedPackUri) await loadEmotes(selectedPackUri);
    } catch (err: any) {
      console.error("Failed to edit emote", err);
      Alert.alert("Error", err.message ?? "Failed to edit emote.");
    } finally {
      setEditingEmote(false);
    }
  };

  const confirmDeletePack = async () => {
    if (!pdsAgent || !session?.did || !deletePackDialog.pack) return;
    const rkey = deletePackDialog.pack.uri.split("/").pop() ?? "";
    try {
      await pdsAgent.com.atproto.repo.deleteRecord({
        repo: session.did,
        collection: "place.stream.emote.pack",
        rkey,
      });
      setDeletePackDialog({ isVisible: false, pack: null });
      setSelectedPackUri(null);
      setEmotes(null);
      await loadPacks();
    } catch (err: any) {
      console.error("Failed to delete emote pack", err);
      Alert.alert("Error", err.message ?? "Failed to delete emote pack.");
    }
  };

  const requestDeleteEmote = (rkey: string) => {
    const emote = emotes?.find((e) => e.uri.endsWith(`/${rkey}`));
    if (emote) setDeleteEmoteDialog({ isVisible: true, emote });
  };

  const confirmDeleteEmote = async () => {
    if (!pdsAgent || !session?.did || !deleteEmoteDialog.emote) return;
    const rkey = deleteEmoteDialog.emote.uri.split("/").pop() ?? "";
    try {
      setDeletingEmotes((prev) => new Set(prev).add(rkey));
      await pdsAgent.com.atproto.repo.deleteRecord({
        repo: session.did,
        collection: "place.stream.emote.item",
        rkey,
      });
      setDeleteEmoteDialog({ isVisible: false, emote: null });
      if (selectedPackUri) await loadEmotes(selectedPackUri);
    } catch (err: any) {
      console.error("Failed to delete emote", err);
      Alert.alert("Error", err.message ?? "Failed to delete emote.");
    } finally {
      setDeletingEmotes((prev) => {
        const next = new Set(prev);
        next.delete(rkey);
        return next;
      });
    }
  };

  useEffect(() => {
    loadPacks();
  }, [pdsAgent, session?.did]);

  useEffect(() => {
    if (selectedPackUri) {
      loadEmotes(selectedPackUri);
    } else {
      setEmotes(null);
    }
  }, [selectedPackUri]);

  if (!pdsAgent || !session) {
    return <Loading />;
  }

  const selectedPack = packs?.find((p) => p.uri === selectedPackUri) ?? null;

  return (
    <>
      <ScrollView>
        <View style={[zero.layout.flex.align.center, zero.px[2], zero.py[2]]}>
          <View style={{ maxWidth: 800, width: "100%" }}>
            <MenuContainer>
              <View>
                <Text size="xl">Emote Packs</Text>
                <Text size="lg" style={[text.gray[400], { marginTop: 4 }]}>
                  Manage custom emote packs for yourself or your chat.
                </Text>

                {loadingPacks ? (
                  <View style={[mt[4]]}>
                    <Loading />
                  </View>
                ) : packs && packs.length > 0 ? (
                  <View
                    style={[
                      layout.flex.row,
                      gap.all[3],
                      mt[4],
                      { alignItems: "center" },
                    ]}
                  >
                    <View style={[zero.flex.values[1]]}>
                      <Select
                        value={selectedPackUri ?? undefined}
                        onValueChange={setSelectedPackUri}
                        items={packs.map((p) => ({
                          label: p.value.name,
                          value: p.uri,
                        }))}
                        placeholder="Select a pack..."
                      />
                    </View>
                    <Button
                      variant="secondary"
                      width="min"
                      size="pill"
                      onPress={() =>
                        selectedPack &&
                        setEditPackDialog({
                          isVisible: true,
                          pack: selectedPack,
                        })
                      }
                      disabled={!selectedPack}
                      leftIcon={<Pencil size={14} color={theme.colors.text} />}
                    >
                      <Text>Edit</Text>
                    </Button>
                    <Button
                      variant="secondary"
                      width="min"
                      size="pill"
                      onPress={() =>
                        selectedPack &&
                        setDelegatePackDialog({
                          isVisible: true,
                          pack: selectedPack,
                        })
                      }
                      disabled={!selectedPack}
                      leftIcon={<Share2 size={14} color={theme.colors.text} />}
                    >
                      <Text>Delegate</Text>
                    </Button>
                    <Button
                      variant="destructive"
                      width="min"
                      size="pill"
                      onPress={() =>
                        selectedPack &&
                        setDeletePackDialog({
                          isVisible: true,
                          pack: selectedPack,
                        })
                      }
                      disabled={!selectedPack}
                    >
                      <Text>Delete Pack</Text>
                    </Button>
                  </View>
                ) : (
                  <View style={[layout.flex.row, gap.all[3], mt[2]]}>
                    <Button
                      onPress={() => setShowCreatePack(true)}
                      size="pill"
                      width="min"
                      leftIcon={<Plus color={theme.colors.text} />}
                    >
                      <Text>New Pack</Text>
                    </Button>
                  </View>
                )}
              </View>
            </MenuContainer>

            {selectedPack && (
              <MenuContainer>
                <View>
                  <View
                    style={[
                      layout.flex.row,
                      {
                        justifyContent: "space-between",
                        alignItems: "center",
                      },
                    ]}
                  >
                    <View>
                      <Text size="lg">Emotes</Text>
                      {selectedPack.value.description && (
                        <Text size="sm" muted style={[mt[1]]}>
                          {selectedPack.value.description}
                        </Text>
                      )}
                    </View>
                    <Button
                      onPress={() => setShowCreateEmote(true)}
                      size="pill"
                      width="min"
                      leftIcon={<Plus color={theme.colors.text} />}
                    >
                      <Text>Add Emote</Text>
                    </Button>
                  </View>
                </View>

                {loadingEmotes ? (
                  <View>
                    <Loading />
                  </View>
                ) : !emotes || emotes.length === 0 ? (
                  <View style={[layout.flex.center, mt[4], mb[2]]}>
                    <Text style={[text.gray[600], { fontSize: 16 }]}>
                      No emotes yet.
                    </Text>
                  </View>
                ) : (
                  <View style={[mb[4]]}>
                    <MenuGroup>
                      {emotes.map((emote, i) => {
                        const rkey = emote.uri.split("/").pop() ?? "";
                        const imageUrl = emoteImageUrl(
                          session.did,
                          emote.value,
                        );
                        return (
                          <View key={emote.uri}>
                            {i > 0 && <MenuSeparator />}
                            <SettingsRowItem>
                              <Image
                                source={{ uri: imageUrl }}
                                style={{
                                  width: 36,
                                  height: 36,
                                  borderRadius: 4,
                                }}
                                resizeMode="contain"
                              />
                              <View
                                style={[
                                  zero.flex.values[1],
                                  { marginLeft: 12 },
                                ]}
                              >
                                <Text size="lg">:{emote.value.name}:</Text>
                                {emote.value.alt && (
                                  <Text size="sm" muted>
                                    {emote.value.alt}
                                  </Text>
                                )}
                              </View>
                              <Pressable
                                onPress={() =>
                                  setEditEmoteDialog({
                                    isVisible: true,
                                    emote,
                                  })
                                }
                                style={({ pressed }) => ({
                                  padding: 8,
                                  borderRadius: 6,
                                  backgroundColor: pressed
                                    ? "#ffffff08"
                                    : "transparent",
                                })}
                              >
                                <Pencil
                                  size={18}
                                  color={theme.colors.textMuted}
                                />
                              </Pressable>
                              <Pressable
                                onPress={() => requestDeleteEmote(rkey)}
                                disabled={deletingEmotes.has(rkey)}
                                style={({ pressed }) => ({
                                  padding: 8,
                                  borderRadius: 6,
                                  backgroundColor: pressed
                                    ? "#ffffff08"
                                    : "transparent",
                                  opacity: deletingEmotes.has(rkey) ? 0.5 : 1,
                                })}
                              >
                                <Trash2
                                  size={18}
                                  color={theme.colors.destructive}
                                />
                              </Pressable>
                            </SettingsRowItem>
                          </View>
                        );
                      })}
                    </MenuGroup>
                  </View>
                )}
              </MenuContainer>
            )}
          </View>
        </View>
      </ScrollView>

      <CreatePackDialog
        isVisible={showCreatePack}
        onClose={() => setShowCreatePack(false)}
        onSubmit={createPack}
        isLoading={creatingPack}
      />

      <CreateEmoteDialog
        isVisible={showCreateEmote}
        onClose={() => setShowCreateEmote(false)}
        onSubmit={createEmote}
        isLoading={creatingEmote}
      />

      <EditPackDialog
        isVisible={editPackDialog.isVisible}
        onClose={() => setEditPackDialog({ isVisible: false, pack: null })}
        onSubmit={editPack}
        isLoading={editingPack}
        pack={editPackDialog.pack}
      />

      <DelegatePackDialog
        isVisible={delegatePackDialog.isVisible}
        onClose={() => setDelegatePackDialog({ isVisible: false, pack: null })}
        onSubmit={delegatePack}
        isLoading={delegating}
        pack={delegatePackDialog.pack}
      />

      <EditEmoteDialog
        isVisible={editEmoteDialog.isVisible}
        onClose={() => setEditEmoteDialog({ isVisible: false, emote: null })}
        onSubmit={editEmote}
        isLoading={editingEmote}
        emote={editEmoteDialog.emote}
      />

      <Dialog
        open={deletePackDialog.isVisible}
        onOpenChange={(open) =>
          !open && setDeletePackDialog({ isVisible: false, pack: null })
        }
        title="Delete Emote Pack"
        dismissible={false}
      >
        <View style={[w.percent[100], mb[8], mt[2]]}>
          <Text style={[{ fontSize: 24 }]}>
            Delete "{deletePackDialog.pack?.value.name}"?
          </Text>
          <Text
            style={[text.gray[400], mt[4], { fontSize: 18, fontWeight: "700" }]}
          >
            This cannot be undone.
          </Text>
        </View>
        <View style={[layout.flex.row, layout.flex.justify.end, gap.all[3]]}>
          <Button
            variant="secondary"
            width="full"
            onPress={() =>
              setDeletePackDialog({ isVisible: false, pack: null })
            }
          >
            <Text>Cancel</Text>
          </Button>
          <Button
            variant="destructive"
            width="full"
            onPress={confirmDeletePack}
          >
            <Text style={[text.white]}>Delete</Text>
          </Button>
        </View>
      </Dialog>

      <Dialog
        open={deleteEmoteDialog.isVisible}
        onOpenChange={(open) =>
          !open && setDeleteEmoteDialog({ isVisible: false, emote: null })
        }
        title="Delete Emote"
        dismissible={false}
      >
        <View style={[w.percent[100], mb[8], mt[2]]}>
          <Text style={[{ fontSize: 24 }]}>
            Delete ":{deleteEmoteDialog.emote?.value.name}:"?
          </Text>
          <Text
            style={[text.gray[400], mt[4], { fontSize: 18, fontWeight: "700" }]}
          >
            This cannot be undone.
          </Text>
        </View>
        <View style={[layout.flex.row, layout.flex.justify.end, gap.all[3]]}>
          <Button
            variant="secondary"
            width="full"
            onPress={() =>
              setDeleteEmoteDialog({ isVisible: false, emote: null })
            }
          >
            <Text>Cancel</Text>
          </Button>
          <Button
            variant="destructive"
            width="full"
            onPress={confirmDeleteEmote}
            disabled={
              deleteEmoteDialog.emote
                ? deletingEmotes.has(
                    deleteEmoteDialog.emote.uri.split("/").pop() ?? "",
                  )
                : false
            }
          >
            <Text style={[text.white]}>Delete</Text>
          </Button>
        </View>
      </Dialog>
    </>
  );
}
