import {
  Admonition,
  Button,
  Checkbox,
  ContentMetadataForm,
  Dashboard,
  formatHandle,
  formatHandleWithAt,
  getBlob,
  Input,
  resolveDIDDocument,
  Text,
  Textarea,
  Tooltip,
  useCreateStreamRecord,
  useEndLivestream,
  useLivestream,
  useToast,
  useUpdateStreamRecord,
  useUrl,
  zero,
} from "@streamplace/components";
import { error } from "console";
import { ArrowRight, ImagePlus, X } from "lucide-react-native";
import { useCallback, useEffect, useMemo, useState } from "react";
import {
  Image,
  Platform,
  Pressable,
  ScrollView,
  TouchableOpacity,
  View,
} from "react-native";
import { useUserProfile } from "store/hooks";
import { useCaptureVideoFrame } from "../../hooks/useCaptureVideoFrame";
import { useLiveUser } from "../../hooks/useLiveUser";

const { flex, p, px, py, gap, layout, bg, borders, text, r, w, typography } =
  zero;

const isWeb = Platform.OS === "web";

const ButtonSelector = ({
  values,
  selectedValue,
  setSelectedValue,
  disabledValues = [],
  style = [],
}: {
  values: { label: string; value: string }[];
  selectedValue: string;
  setSelectedValue: (value: string) => void;
  disabledValues?: string[];
  style?: any[];
}) => (
  <View style={[layout.flex.row, gap.all[1], ...style]}>
    {values.map(({ label, value }) => (
      <Button
        key={value}
        variant={selectedValue === value ? "primary" : "secondary"}
        size="pill"
        width="min"
        disabled={disabledValues.includes(value)}
        onPress={() => setSelectedValue(value)}
        style={[
          r.md,
          {
            opacity: disabledValues.includes(value) ? 0.5 : 1,
          },
        ]}
      >
        <Text
          style={[
            selectedValue === value ? text.white : text.gray[300],
            { fontSize: 14, fontWeight: "600" },
          ]}
        >
          {label}
        </Text>
      </Button>
    ))}
  </View>
);

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
      borders.color.neutral[600],
      bg.neutral[800],
      r.md,
      layout.flex.center,
      {
        height: 200,
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
        resizeMode: "cover" as const,
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
        backgroundColor: "rgba(0, 0, 0, 0.7)",
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
          <Image source={{ uri: imageUrl }} style={imageStyle} />
          <TouchableOpacity onPress={onImageRemove} style={removeButtonStyle}>
            <X size={16} color="white" />
          </TouchableOpacity>
        </View>
      ) : (
        <>
          <TouchableOpacity onPress={onImageSelect} style={containerStyle}>
            <ImagePlus size={48} color="#6b7280" />
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
      <View style={{ marginTop: 8 }}>
        <Admonition variant="info" size="sm">
          <Text size="sm">
            You are required to disclose if your content is not suitable for
            certain viewers.
          </Text>
          <Pressable onPress={onGoToMetadata}>
            <Text size="sm" color={zero.colors.blue[400]}>
              Go to the metadata page{" "}
              <ArrowRight size="14" style={{ marginVertical: -2 }} />
            </Text>
          </Pressable>
        </Admonition>
      </View>
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

  const [title, setTitle] = useState("");
  const [loading, setLoading] = useState(false);
  const [selectedImage, setSelectedImage] = useState<
    string | File | Blob | undefined
  >();
  const [mode, setMode] = useState<"create" | "metadata" | "moderation">(
    "create",
  );

  const [createPost, setCreatePost] = useState(true);
  const [sendPushNotification, setSendPushNotification] = useState(true);
  const [canonicalUrl, setCanonicalUrl] = useState<string>("");
  const defaultCanonicalUrl = useMemo(() => {
    return `${url}/${profile && formatHandle(profile)}`;
  }, [url, profile?.handle]);

  useEffect(() => {
    if (!livestream) {
      return;
    }

    // Prefill title with previous stream's title
    if (livestream.record.title) {
      setTitle(livestream.record.title);
    }

    // Prefill canonical URL
    if (
      livestream.record.canonicalUrl &&
      livestream.record.canonicalUrl !== defaultCanonicalUrl
    ) {
      setCanonicalUrl(livestream.record.canonicalUrl);
    }

    // Prefill notification settings
    if (
      typeof livestream.record.notificationSettings?.pushNotification ===
      "boolean"
    ) {
      setSendPushNotification(
        livestream.record.notificationSettings.pushNotification,
      );
    }

    setCreatePost(typeof livestream.record.post !== "undefined");
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
        });
      } else {
        await updateStreamRecord(
          title.trim(),
          livestream,
          thumbnailToUse as Blob | undefined,
        );
      }

      // Show success message
      const toastTitle =
        mode === "create" ? "Livestream announced" : "Livestream updated";

      toast.show(toastTitle, title.trim(), { duration: 4 });

      // Clear form on successful create
      if (mode === "create") {
        setTitle("");
        setSelectedImage(undefined);
      }
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
      await endLivestream(livestream);
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
      console.error("Failed to fetch last image:", error);
      toast.show("Error", "Failed to load previous thumbnail", {
        duration: 3,
      });
    };
  }, [livestream, toast]);

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
    return mode === "create" ? "Announce Livestream!" : "Update Livestream!";
  }, [loading, userIsLive, mode]);

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
    <>
      <Wrapper {...wrapperProps}>
        <View
          style={[
            flex.values[1],
            bg.neutral[900],
            r.lg,
            borders.width.thin,
            borders.color.neutral[700],
            layout.flex.column,
          ]}
        >
          <View
            style={[
              layout.flex.row,
              layout.flex.spaceBetween,
              layout.flex.alignCenter,
              px[4],
              py[4],
              borders.bottom.width.thin,
              borders.bottom.color.neutral[700],
            ]}
          >
            <Text style={[text.white, { fontSize: 18, fontWeight: "600" }]}>
              Stream Settings
            </Text>
            <ButtonSelector
              values={[
                { label: "Create", value: "create" },
                { label: "Metadata", value: "metadata" },
                { label: "Moderation", value: "moderation" },
              ]}
              style={[{ marginVertical: -2 }]}
              selectedValue={mode}
              setSelectedValue={handleModeChange as any}
              disabledValues={[]}
            />
          </View>

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
            <View
              style={[
                gap.all[8],
                w.percent[100],
                { alignSelf: "center" },
                p[4],
                { alignItems: "stretch" },
                { justifyContent: "center" },
              ]}
            >
              <View style={[gap.all[3], w.percent[100]]}>
                <View
                  style={[
                    layout.flex.row,
                    layout.flex.alignCenter,
                    w.percent[100],
                  ]}
                >
                  <Text
                    style={[
                      text.neutral[300],
                      { minWidth: 100, textAlign: "left", paddingBottom: 8 },
                    ]}
                  >
                    Streamer
                  </Text>
                  <Text
                    style={[
                      text.white,
                      { fontWeight: "bold", paddingBottom: 8 },
                    ]}
                  >
                    {profile && formatHandleWithAt(profile)}
                  </Text>
                </View>
                <View
                  style={[
                    layout.flex.row,
                    layout.flex.alignCenter,
                    w.percent[100],
                  ]}
                >
                  <Text
                    style={[
                      text.neutral[300],
                      { minWidth: 100, textAlign: "left", paddingBottom: 8 },
                    ]}
                  >
                    Title
                  </Text>
                  <View style={[flex.values[1]]}>
                    <Textarea
                      value={title}
                      onChangeText={setTitle}
                      placeholder="Enter your stream title..."
                      maxLength={140}
                      multiline
                      style={[
                        p[3],
                        r.md,
                        bg.neutral[800],
                        text.white,
                        borders.width.thin,
                        borders.color.neutral[600],
                        w.percent[100],
                        { minHeight: 100, fontSize: 16 },
                      ]}
                    />
                    <View
                      style={[
                        layout.flex.row,
                        layout.flex.spaceBetween,
                        { marginTop: 4 },
                      ]}
                    >
                      <Text style={[text.gray[500], { fontSize: 12 }]}>
                        {title.trim() === "" ? "Title is required" : ""}
                      </Text>
                      <Text
                        style={[
                          text.gray[500],
                          { fontSize: 12 },
                          title.length > 120 && { color: "#f59e0b" },
                          title.length >= 140 && { color: "#ef4444" },
                        ]}
                      >
                        {title.length}/140
                      </Text>
                    </View>
                  </View>
                </View>

                <Tooltip
                  content="Set this to have the livestream announced with a link to this URL instead of the default URL."
                  position="top"
                >
                  <View
                    style={[
                      layout.flex.row,
                      layout.flex.alignCenter,
                      w.percent[100],
                    ]}
                  >
                    <Text
                      style={[
                        text.neutral[300],
                        {
                          minWidth: 100,
                          textAlign: "left",
                          paddingBottom: 8,
                          fontSize: 14,
                        },
                      ]}
                    >
                      Canonical URL
                    </Text>
                    <View style={[flex.values[1]]}>
                      <Input
                        value={canonicalUrl}
                        onChange={(value) => setCanonicalUrl(value)}
                        placeholder={defaultCanonicalUrl}
                        variant="filled"
                        inputStyle={[
                          p[3],
                          r.md,
                          bg.neutral[800],
                          text.white,
                          borders.width.thin,
                          borders.color.neutral[600],
                          w.percent[100],
                        ]}
                      />
                    </View>
                  </View>
                </Tooltip>

                <Tooltip
                  content="Create a Bluesky post announcing you're live with a link to the stream."
                  position="top"
                >
                  <Checkbox
                    checked={createPost}
                    onCheckedChange={(checked) => setCreatePost(checked)}
                    label={"Create Bluesky post"}
                    style={[{ fontSize: 12 }]}
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
                    label={"Send push notification"}
                    style={[{ fontSize: 12 }]}
                  />
                </Tooltip>
              </View>

              {/* Image upload for create mode */}
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

              <Button
                disabled={disabled}
                onPress={handleSubmit}
                style={[
                  bg.primary[500],
                  r.md,
                  py[3],
                  w.percent[100],
                  layout.flex.center,
                  { opacity: disabled ? 0.5 : 1 },
                ]}
              >
                <Text
                  style={[text.white, { fontSize: 16, fontWeight: "bold" }]}
                >
                  {buttonText}
                </Text>
              </Button>
              <Button
                variant={canEndLivestream ? "destructive" : "secondary"}
                onPress={handleEndLivestream}
                style={[
                  r.md,
                  py[3],
                  w.percent[100],
                  layout.flex.center,
                  {
                    opacity: !canEndLivestream ? 0.5 : 1,
                    cursor: canEndLivestream ? "pointer" : "not-allowed",
                  },
                ]}
                disabled={!canEndLivestream || endingLivestream}
              >
                <Text
                  style={[text.white, { fontSize: 16, fontWeight: "bold" }]}
                >
                  {endingLivestream ? "Ending Livestream..." : "End Livestream"}
                </Text>
              </Button>
            </View>
          )}
        </View>
      </Wrapper>
    </>
  );
}

export default LivestreamPanel;
