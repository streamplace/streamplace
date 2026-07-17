import {
  Admonition,
  Button,
  Checkbox,
  ContentMetadataForm,
  Dashboard,
  DropdownMenu,
  DropdownMenuItem,
  DropdownMenuTrigger,
  formatHandle,
  getBlob,
  hexToRgba,
  Input,
  resolveDIDDocument,
  ResponsiveDropdownMenuContent,
  SegmentedTabs,
  Text,
  Textarea,
  Tooltip,
  useCreateStreamRecord,
  useEndLivestream,
  useLivestream,
  useTheme,
  useToast,
  useUpdateStreamRecord,
  useUrl,
  zero,
} from "@streamplace/components";
import {
  scrims,
  shadows,
  statusColors,
  textAlphas,
  surfaces,
  borderAlphas,
} from "@streamplace/components/src/lib/theme/tokens";
import { Image } from "expo-image";
import { ChevronsUpDown, Globe, ImagePlus, X } from "lucide-react-native";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import {
  Platform,
  Pressable,
  ScrollView,
  TouchableOpacity,
  View,
} from "react-native";
import { useUserProfile } from "store/hooks";
import type { PlaceStreamLivestream } from "streamplace";
import { useCaptureVideoFrame } from "../../hooks/useCaptureVideoFrame";
import { useLiveUser } from "../../hooks/useLiveUser";
import ActivityPicker from "../activity-picker";

const { flex, p, px, py, gap, layout, bg, borders, text, r, w, typography } =
  zero;

const isWeb = Platform.OS === "web";

export const LANG_TAG_PREFIX = "lang:";

export const LANGUAGES = [
  { code: "en", label: "English", native: "English" },
  { code: "es", label: "Spanish", native: "Español" },
  { code: "fr", label: "French", native: "Français" },
  { code: "de", label: "German", native: "Deutsch" },
  { code: "pt", label: "Portuguese", native: "Português" },
  { code: "ja", label: "Japanese", native: "日本語" },
  { code: "ko", label: "Korean", native: "한국어" },
  { code: "zh", label: "Chinese", native: "中文" },
  { code: "ar", label: "Arabic", native: "العربية" },
  { code: "ru", label: "Russian", native: "Русский" },
  { code: "hi", label: "Hindi", native: "हिन्दी" },
  { code: "tr", label: "Turkish", native: "Türkçe" },
  { code: "it", label: "Italian", native: "Italiano" },
  { code: "pl", label: "Polish", native: "Polski" },
  { code: "nl", label: "Dutch", native: "Nederlands" },
  { code: "sv", label: "Swedish", native: "Svenska" },
  { code: "id", label: "Indonesian", native: "Bahasa Indonesia" },
  { code: "th", label: "Thai", native: "ภาษาไทย" },
  { code: "vi", label: "Vietnamese", native: "Tiếng Việt" },
  { code: "uk", label: "Ukrainian", native: "Українська" },
];

function LanguagePicker({
  tags,
  onTagsChange,
}: {
  tags: string[];
  onTagsChange: (tags: string[]) => void;
}) {
  const current = tags.find((t) => t.startsWith(LANG_TAG_PREFIX));
  const currentCode = current ? current.slice(LANG_TAG_PREFIX.length) : null;
  const currentLabel =
    LANGUAGES.find((l) => l.code === currentCode)?.native ?? null;

  const select = (code: string | null) => {
    const withoutLang = tags.filter((t) => !t.startsWith(LANG_TAG_PREFIX));
    onTagsChange(
      code ? [...withoutLang, `${LANG_TAG_PREFIX}${code}`] : withoutLang,
    );
  };

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button
          variant="secondary"
          size="sm"
          width="min"
          style={[
            { backgroundColor: surfaces.dark[2] },
            { borderColor: borderAlphas.dark.strong },
          ]}
          leftIcon={<Globe size={14} color={textAlphas.dark[2]} />}
          rightIcon={<ChevronsUpDown size={14} color={textAlphas.dark[3]} />}
        >
          <Text style={[text.neutral[300], { fontSize: 14 }]}>
            {currentLabel ?? "Language"}
          </Text>
        </Button>
      </DropdownMenuTrigger>
      <ResponsiveDropdownMenuContent>
        {currentLabel && (
          <DropdownMenuItem onPress={() => select(null)}>
            <Text style={[text.neutral[400], { fontSize: 14 }]}>None</Text>
          </DropdownMenuItem>
        )}
        {LANGUAGES.map((lang) => (
          <DropdownMenuItem key={lang.code} onPress={() => select(lang.code)}>
            <Text
              style={[
                lang.code === currentCode ? text.white : text.neutral[300],
                { fontSize: 14 },
              ]}
            >
              {lang.native}
            </Text>
          </DropdownMenuItem>
        ))}
      </ResponsiveDropdownMenuContent>
    </DropdownMenu>
  );
}

const ImageUploadComponent = ({
  selectedImage,
  onImageSelect,
  onImageRemove,
  onUseLastImage,
  hasLastImage,
  onGoToMetadata,
}: {
  selectedImage?: string | File | Blob;
  onImageSelect?: () => void;
  onImageRemove?: () => void;
  onUseLastImage?: () => void;
  hasLastImage?: boolean;
  onGoToMetadata?: () => void;
}) => {
  const imageUrl = useMemo(() => {
    if (!selectedImage) return undefined;
    if (selectedImage instanceof File || selectedImage instanceof Blob) {
      return URL.createObjectURL(selectedImage);
    }
    return selectedImage;
  }, [selectedImage]);

  const containerStyle = useMemo(
    () => [
      borders.width.thin,
      { borderColor: borderAlphas.dark.strong },
      { backgroundColor: surfaces.dark[2] },
      r.md,
      layout.flex.center,
      {
        height: 100,
        borderStyle: "dashed",
      },
    ],
    [],
  );

  const imageStyle = useMemo(
    () => [
      r.md,
      {
        width: "100%",
        height: 200,
      },
    ],
    [],
  );

  const removeButtonStyle = useMemo(
    () => [
      {
        position: "absolute" as const,
        top: 8,
        right: 8,
        backgroundColor: scrims.dark,
        borderRadius: 12,
        width: 24,
        height: 24,
      },
      layout.flex.center,
    ],
    [],
  );

  return (
    <View style={[gap.all[2]]}>
      <Text style={[text.white, { fontWeight: "bold", marginBottom: 8 }]}>
        Thumbnail (Optional)
      </Text>

      {selectedImage ? (
        <View style={[{ position: "relative" }]}>
          <Image
            source={{ uri: imageUrl }}
            style={imageStyle}
            contentFit="cover"
          />
          <TouchableOpacity onPress={onImageRemove} style={removeButtonStyle}>
            <X size={16} color="white" />
          </TouchableOpacity>
        </View>
      ) : (
        <>
          <TouchableOpacity onPress={onImageSelect} style={containerStyle}>
            <ImagePlus size={48} color={textAlphas.dark[3]} />
            <Text style={[text.gray[400], { marginTop: 8, fontSize: 14 }]}>
              Add thumbnail image
            </Text>
            <Text style={[text.gray[500], { fontSize: 12, marginTop: 4 }]}>
              Optional • JPG, PNG up to 975KB
            </Text>
          </TouchableOpacity>
          {hasLastImage && (
            <Button
              variant="secondary"
              size="sm"
              onPress={onUseLastImage}
              style={[{ marginTop: 8 }]}
            >
              <Text style={[text.gray[300], { fontSize: 14 }]}>
                Use Last Image
              </Text>
            </Button>
          )}
        </>
      )}
      {selectedImage &&
        selectedImage instanceof Blob &&
        selectedImage.size > 975000 && (
          <View style={{ marginTop: 8 }}>
            <Admonition variant="warning" size="sm">
              <Text size="sm">
                Heads up: this image is larger than 975KB (it's
                {" " + selectedImage.size} bytes). Bluesky post creation might
                fail.
              </Text>
            </Admonition>
          </View>
        )}
    </View>
  );
};

function LivestreamPanel({ scrollable = true }: { scrollable?: boolean }) {
  const toast = useToast();
  const userIsLive = useLiveUser();
  const captureFrame = useCaptureVideoFrame();
  const profile = useUserProfile();
  const livestream = useLivestream(true);
  const createStreamRecord = useCreateStreamRecord();
  const updateStreamRecord = useUpdateStreamRecord();
  const endLivestream = useEndLivestream();
  const url = useUrl();
  const [endingLivestream, setEndingLivestream] = useState(false);

  const initializedRef = useRef(false);
  const [title, setTitle] = useState("");
  const [loading, setLoading] = useState(false);
  const [selectedImage, setSelectedImage] = useState<
    string | File | Blob | undefined
  >();
  const [mode, setMode] = useState<"create" | "metadata" | "moderation">(
    "create",
  );

  const [activity, setActivity] = useState<
    PlaceStreamLivestream.Record["activity"] | undefined
  >(undefined);
  const [tags, setTags] = useState<string[]>([]);
  const [tagInput, setTagInput] = useState("");

  const [createPost, setCreatePost] = useState(true);
  const [idleTimeout, setIdleTimeout] = useState(true);
  const [sendPushNotification, setSendPushNotification] = useState(true);
  const [canonicalUrl, setCanonicalUrl] = useState<string>("");
  const defaultCanonicalUrl = useMemo(() => {
    return `${url}/${profile && formatHandle(profile)}`;
  }, [url, profile?.handle]);

  useEffect(() => {
    if (!livestream) {
      return;
    }

    if (!initializedRef.current) {
      initializedRef.current = true;

      if (livestream.record.title) {
        setTitle(livestream.record.title);
      }

      if (livestream.record.activity) {
        setActivity(
          livestream.record
            .activity as PlaceStreamLivestream.Record["activity"],
        );
      }

      if (livestream.record.tags) {
        setTags(livestream.record.tags as string[]);
      }

      if (
        livestream.record.canonicalUrl &&
        livestream.record.canonicalUrl !== defaultCanonicalUrl
      ) {
        setCanonicalUrl(livestream.record.canonicalUrl);
      }

      if (
        typeof livestream.record.notificationSettings?.pushNotification ===
        "boolean"
      ) {
        setSendPushNotification(
          livestream.record.notificationSettings.pushNotification,
        );
      }

      setCreatePost(typeof livestream.record.post !== "undefined");
    }
  }, [livestream, defaultCanonicalUrl]);

  const handleModeChange = useCallback(
    (newMode: "create" | "metadata" | "moderation") => {
      setMode(newMode);
    },
    [],
  );

  const handleSubmit = useCallback(async () => {
    if (!title.trim()) return;

    setLoading(true);

    try {
      let thumbnailToUse = selectedImage;

      // Auto-capture frame if no custom thumbnail and we have capture capability
      if (!thumbnailToUse && mode === "create" && captureFrame && isWeb) {
        try {
          const capturedFrame = await captureFrame(1280, 0.85);
          if (capturedFrame) {
            thumbnailToUse = capturedFrame;
          }
        } catch (captureError) {
          console.warn("Failed to capture video frame:", captureError);
        }
      }

      if (mode === "create") {
        await createStreamRecord({
          title: title.trim(),
          customThumbnail: thumbnailToUse as Blob | undefined,
          submitPost: createPost,
          notificationSettings: {
            pushNotification: sendPushNotification,
          },
          canonicalUrl: canonicalUrl || undefined,
          idleTimeoutSeconds: idleTimeout ? 300 : 0,
          activity,
          tags,
        });
      } else {
        await updateStreamRecord(
          title.trim(),
          livestream,
          thumbnailToUse as Blob | undefined,
          activity,
          tags,
        );
      }

      // Show success message
      const toastTitle =
        mode === "create" ? "Livestream announced" : "Livestream updated";

      toast.show(toastTitle, title.trim(), { duration: 4 });
    } catch (error) {
      console.error("Error with livestream:", error);

      try {
        // Truncate very long error messages
        const errorMessage = String(error);
        const truncatedError =
          errorMessage.length > 200
            ? errorMessage.substring(0, 200) + "..."
            : errorMessage;

        const errorTitle =
          mode === "create"
            ? "Error creating livestream"
            : "Error updating livestream";

        toast.show(errorTitle, truncatedError, { duration: 5 });
      } catch (toastError) {
        console.error("Error showing toast:", toastError);
      }
    } finally {
      setLoading(false);
    }
  }, [
    title,
    selectedImage,
    mode,
    createStreamRecord,
    updateStreamRecord,
    livestream,
  ]);

  const handleEndLivestream = useCallback(async () => {
    if (!livestream) return;
    setEndingLivestream(true);
    try {
      await endLivestream();
    } catch (error) {
      console.error("Error ending livestream:", error);
      toast.show("Error", "Failed to end livestream", {
        duration: 3,
      });
    }
  }, [livestream, endLivestream]);

  useEffect(() => {
    if (livestream && livestream.record.endedAt !== undefined) {
      setEndingLivestream(false);
    }
  }, [livestream]);

  const handleImageSelect = useCallback(() => {
    // Default web file picker behavior
    const input = document.createElement("input");
    input.type = "file";
    input.accept = "image/*";
    input.onchange = (e) => {
      const file = (e.target as HTMLInputElement).files?.[0];
      if (file) {
        setSelectedImage(file);
      }
    };
    input.click();
  }, []);

  const handleImageRemove = useCallback(() => {
    setSelectedImage(undefined);
  }, []);

  const handleUseLastImage = useCallback(async () => {
    if (!livestream?.record.thumb) return;

    try {
      const did = livestream.uri.split("/")[2];
      const cid = (livestream.record.thumb.ref as any).$link;

      const didDoc = await resolveDIDDocument(did);
      const blob = await getBlob(did, cid, didDoc);
      setSelectedImage(blob);
    } catch (error) {
      console.error("Failed to fetch last image:", error);
      toast.show("Error", "Failed to load previous thumbnail", {
        duration: 3,
      });
    }
  }, [livestream, toast]);

  const { theme } = useTheme();
  const disabled = useMemo(
    () => !userIsLive || loading || title.trim() === "",
    [userIsLive, loading, title],
  );

  const buttonText = useMemo(() => {
    if (loading) return "Loading...";
    if (!userIsLive) {
      return mode === "create"
        ? "Waiting for stream to start..."
        : "Waiting for stream to start...";
    }
    if (!livestream || livestream.record.endedAt !== undefined) {
      return "Start Livestream!";
    }
    return "Update Livestream!";
  }, [loading, userIsLive, mode, livestream]);

  const Wrapper = scrollable ? ScrollView : View;
  const wrapperProps = scrollable
    ? {
        contentContainerStyle: {
          flexGrow: 1,
        },
        showsVerticalScrollIndicator: false,
      }
    : {};

  const canEndLivestream =
    livestream && livestream.record.endedAt === undefined;

  return (
    <View
      style={[
        flex.values[1],
        { backgroundColor: surfaces.dark[1] },
        r.lg,
        borders.width.thin,
        { borderColor: borderAlphas.dark.strong },
        layout.flex.column,
        // Clip the scroll body to the rounded corners — only in the desktop
        // sticky-footer layout; the mobile inline layout must not be clipped.
        scrollable && { overflow: "hidden" },
      ]}
    >
      {/* Fixed header */}
      <View
        style={[
          px[4],
          py[4],
          borders.bottom.width.thin,
          { borderBottomColor: borderAlphas.dark.strong, gap: 12 },
        ]}
      >
        <Text style={[text.white, { fontSize: 15, fontWeight: "600" }]}>
          Stream Settings
        </Text>
        <SegmentedTabs
          fullWidth
          size="sm"
          options={[
            { label: "Create", value: "create" },
            { label: "Metadata", value: "metadata" },
            { label: "Moderation", value: "moderation" },
          ]}
          value={mode}
          onChange={handleModeChange as any}
        />
      </View>

      {/* Scrollable body */}
      <Wrapper
        style={
          scrollable
            ? { flexGrow: 1, flexShrink: 1, flexBasis: 0, minHeight: 0 }
            : undefined
        }
        {...wrapperProps}
      >
        {mode === "metadata" ? (
            // Metadata view
            <View style={[flex.values[1], p[4]]}>
              <ContentMetadataForm
                showUpdateButton={!userIsLive}
                style={{ flex: 1, height: "100%" }}
              />
            </View>
          ) : mode === "moderation" ? (
            // Moderation view
            <View style={[flex.values[1], { minHeight: 400 }]}>
              <Dashboard.ModeratorPanel isLive={userIsLive} embedded={true} />
            </View>
          ) : (
            // Create/Edit view
            <View style={[gap.all[5], w.percent[100], p[4]]}>
              {/* Fields */}
              <View style={[gap.all[4], w.percent[100]]}>
                {/* Title */}
                <View style={[gap.all[1]]}>
                  <Text size="sm" color="muted">
                    Title
                  </Text>
                  <Textarea
                    value={title}
                    onChangeText={setTitle}
                    placeholder="Enter your stream title..."
                    maxLength={140}
                    multiline
                    style={[
                      p[3],
                      r.md,
                      { backgroundColor: surfaces.dark[2] },
                      text.white,
                      borders.width.thin,
                      { borderColor: borderAlphas.dark.strong },
                      w.percent[100],
                      { fontSize: 15, lineHeight: 22 },
                    ]}
                  />
                  <View style={[layout.flex.row, layout.flex.spaceBetween]}>
                    <Text style={[text.neutral[600], { fontSize: 11 }]}>
                      {title.trim() === "" ? "Required" : ""}
                    </Text>
                    <Text
                      style={[
                        { fontSize: 11 },
                        title.length <= 120
                          ? text.neutral[600]
                          : title.length < 140
                            ? { color: statusColors.dark.warning }
                            : { color: statusColors.dark.danger },
                      ]}
                    >
                      {title.length}/140
                    </Text>
                  </View>
                </View>

                {/* Activity */}
                <View style={[gap.all[1]]}>
                  <Text size="sm" color="muted">
                    Activity
                  </Text>
                  <ActivityPicker value={activity} onChange={setActivity} />
                </View>

                {/* Tags + language */}
                <View style={[gap.all[2]]}>
                  <View
                    style={[
                      layout.flex.row,
                      layout.flex.spaceBetween,
                      layout.flex.alignCenter,
                    ]}
                  >
                    <Text size="sm" color="muted">
                      Tags
                    </Text>
                    <LanguagePicker tags={tags} onTagsChange={setTags} />
                  </View>
                  {tags.length < 10 && (
                    <Input
                      value={tagInput}
                      onChange={(v) =>
                        setTagInput(v.replace(/[^a-zA-Z0-9]/g, ""))
                      }
                      placeholder="Add a tag, press Enter"
                      variant="filled"
                      onSubmitEditing={(e) => {
                        const trimmed = tagInput.trim();
                        if (trimmed && !tags.includes(trimmed)) {
                          setTags([...tags, trimmed]);
                        }
                        setTagInput("");
                        // re-focus the input after submitting
                        setTimeout(() => {
                          const input = e.target;
                          input.focus();
                        }, 10);
                      }}
                      inputStyle={[
                        p[3],
                        r.md,
                        { backgroundColor: surfaces.dark[2] },
                        text.white,
                        borders.width.thin,
                        { borderColor: borderAlphas.dark.strong },
                        w.percent[100],
                        { fontSize: 15 },
                      ]}
                    />
                  )}
                  {tags.filter((t) => !t.startsWith(LANG_TAG_PREFIX)).length >
                    0 && (
                    <View
                      style={{
                        flexDirection: "row",
                        flexWrap: "wrap",
                        gap: 6,
                        width: "100%",
                      }}
                    >
                      {tags
                        .filter((t) => !t.startsWith(LANG_TAG_PREFIX))
                        .map((tag) => (
                          <Pressable
                            key={tag}
                            onPress={() =>
                              setTags(tags.filter((t) => t !== tag))
                            }
                            style={{
                              flexDirection: "row",
                              alignItems: "center",
                              backgroundColor: hexToRgba(
                                theme.colors.primary,
                                0.12,
                              ),
                              borderWidth: 1,
                              borderColor: hexToRgba(theme.colors.primary, 0.3),
                              borderRadius: 16,
                              paddingHorizontal: 10,
                              paddingVertical: 3,
                              gap: 5,
                            }}
                          >
                            <Text
                              style={{
                                color: theme.colors.primary,
                                fontSize: 13,
                              }}
                            >
                              {tag}
                            </Text>
                            <Text
                              style={{
                                color: hexToRgba(theme.colors.primary, 0.6),
                                fontSize: 14,
                                lineHeight: 16,
                              }}
                            >
                              ×
                            </Text>
                          </Pressable>
                        ))}
                    </View>
                  )}
                </View>

                {/* Canonical URL */}
                <Tooltip
                  content="Set this to have the livestream announced with a link to this URL instead of the default URL."
                  position="top"
                >
                  <View style={[gap.all[1]]}>
                    <Text size="sm" color="muted">
                      Canonical URL
                    </Text>
                    <Input
                      value={canonicalUrl}
                      onChange={(value) => setCanonicalUrl(value)}
                      placeholder={defaultCanonicalUrl}
                      variant="filled"
                      inputStyle={[
                        p[3],
                        r.md,
                        { backgroundColor: surfaces.dark[2] },
                        text.white,
                        borders.width.thin,
                        { borderColor: borderAlphas.dark.strong },
                        w.percent[100],
                        { fontSize: 15 },
                      ]}
                    />
                  </View>
                </Tooltip>

                {/* Options */}
                <View
                  style={[
                    gap.all[2],
                    p[3],
                    r.md,
                    borders.width.thin,
                    { borderColor: borderAlphas.dark.strong },
                  ]}
                >
                  <Tooltip
                    content="Create a Bluesky post announcing you're live with a link to the stream."
                    position="top"
                  >
                    <Checkbox
                      checked={createPost}
                      onCheckedChange={(checked) => setCreatePost(checked)}
                      label="Create Bluesky post"
                    />
                  </Tooltip>
                  <Tooltip
                    content="Send a push notification to your followers on the Streamplace iOS/Android app."
                    position="top"
                  >
                    <Checkbox
                      checked={sendPushNotification}
                      onCheckedChange={(checked) =>
                        setSendPushNotification(checked)
                      }
                      label="Send push notification"
                    />
                  </Tooltip>
                  <Tooltip
                    content="Enabling this setting will turn your livestream off after 5 minutes of inactivity, and you'll need to press the 'Start Livestream' button again to start it again next time you stream."
                    position="top"
                  >
                    <Checkbox
                      checked={idleTimeout}
                      onCheckedChange={(checked) => setIdleTimeout(checked)}
                      label="End livestream automatically"
                    />
                  </Tooltip>
                </View>
              </View>

              {/* Thumbnail */}
              {mode === "create" && (
                <ImageUploadComponent
                  selectedImage={selectedImage}
                  onImageSelect={handleImageSelect}
                  onImageRemove={handleImageRemove}
                  onUseLastImage={handleUseLastImage}
                  hasLastImage={!!livestream?.record.thumb}
                  onGoToMetadata={() => handleModeChange("metadata")}
                />
              )}

            </View>
          )}
      </Wrapper>

      {/* Sticky footer — the primary go-live actions stay pinned above the fold
          no matter how long the settings form gets. The destructive End action
          only appears once there's a published stream to end. */}
      {mode === "create" && (
        <View
          style={[
            px[4],
            py[4],
            {
              borderTopWidth: 1,
              borderTopColor: borderAlphas.dark.strong,
              backgroundColor: surfaces.dark[1],
              gap: 8,
              // Cast the footer's shadow *up* over the scroll body so content
              // reads as passing beneath it — the cue that there's more to
              // scroll. (A gradient scrim would be web-only; shadows are
              // cross-platform.)
              ...shadows.lg,
              shadowOffset: { width: 0, height: -6 },
              shadowOpacity: 0.4,
            },
          ]}
        >
          {/* Hero action → Paper (system primary), not indigo. */}
          <Button
            disabled={disabled}
            onPress={handleSubmit}
            style={[py[3], { opacity: disabled ? 0.5 : 1 }]}
          >
            {buttonText}
          </Button>
          {canEndLivestream && (
            <Button
              variant="danger"
              onPress={handleEndLivestream}
              style={[py[3]]}
              disabled={endingLivestream}
            >
              {endingLivestream ? "Ending..." : "End Livestream"}
            </Button>
          )}
        </View>
      )}
    </View>
  );
}

export default LivestreamPanel;
