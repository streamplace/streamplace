import {
  Button,
  ContentMetadataForm,
  Textarea,
  useCreateStreamRecord,
  useLivestream,
  useUpdateStreamRecord,
  zero,
} from "@streamplace/components";
import { useToast } from "@streamplace/components/src/components/ui/toast";
import { ImagePlus, X } from "lucide-react-native";
import { useCallback, useEffect, useMemo, useState } from "react";
import {
  Image,
  Platform,
  ScrollView,
  Text,
  TouchableOpacity,
  View,
} from "react-native";
import { selectUserProfile } from "../../features/bluesky/blueskySlice";
import { useCaptureVideoFrame } from "../../hooks/useCaptureVideoFrame";
import { useLiveUser } from "../../hooks/useLiveUser";
import { useAppSelector } from "../../store/hooks";

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
}: {
  selectedImage?: string | File | Blob;
  onImageSelect?: () => void;
  onImageRemove?: () => void;
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
        <TouchableOpacity onPress={onImageSelect} style={containerStyle}>
          <ImagePlus size={48} color="#6b7280" />
          <Text style={[text.gray[400], { marginTop: 8, fontSize: 14 }]}>
            Add thumbnail image
          </Text>
          <Text style={[text.gray[500], { fontSize: 12, marginTop: 4 }]}>
            Optional • JPG, PNG up to 975KB
          </Text>
        </TouchableOpacity>
      )}
    </View>
  );
};

function LivestreamPanel() {
  const { toastController, ToastComponent } = useToast();
  const userIsLive = useLiveUser();
  const captureFrame = useCaptureVideoFrame();
  const profile = useAppSelector(selectUserProfile);
  const livestream = useLivestream();
  const createStreamRecord = useCreateStreamRecord();
  const updateStreamRecord = useUpdateStreamRecord();

  const [title, setTitle] = useState("");
  const [loading, setLoading] = useState(false);
  const [selectedImage, setSelectedImage] = useState<
    string | File | Blob | undefined
  >();
  const [mode, setMode] = useState<"create" | "edit" | "metadata">(
    livestream ? "edit" : "create",
  );
  const [toastTimeoutId, setToastTimeoutId] = useState<NodeJS.Timeout | null>(
    null,
  );

  const handleModeChange = useCallback(
    (newMode: "create" | "edit" | "metadata") => {
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
          submitPost: true,
        });
      } else {
        await updateStreamRecord(
          title.trim(),
          livestream,
          thumbnailToUse as Blob | undefined,
        );
      }

      // Clear any existing timeout
      if (toastTimeoutId) {
        clearTimeout(toastTimeoutId);
      }

      // Show success message
      const toastTitle =
        mode === "create" ? "Livestream announced" : "Livestream updated";

      toastController.show(toastTitle, title.trim(), { duration: 4 });

      // Add manual timeout as fallback
      const timeoutId = setTimeout(() => {
        toastController.hide();
      }, 4500);
      setToastTimeoutId(timeoutId);

      // Clear form on successful create
      if (mode === "create") {
        setTitle("");
        setSelectedImage(undefined);
      }
    } catch (error) {
      console.error("Error with livestream:", error);

      try {
        // Clear any existing timeout
        if (toastTimeoutId) {
          clearTimeout(toastTimeoutId);
        }

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

        toastController.show(errorTitle, truncatedError, { duration: 5 });

        // Add manual timeout as fallback
        const timeoutId = setTimeout(() => {
          toastController.hide();
        }, 5500);
        setToastTimeoutId(timeoutId);
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
    captureFrame,
    createStreamRecord,
    updateStreamRecord,
    livestream,
    toastController,
  ]);

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

  const noLivestream = mode === "edit" && !livestream;

  const disabled = useMemo(
    () => !userIsLive || loading || title.trim() === "" || noLivestream,
    [userIsLive, loading, title, noLivestream],
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

  // Clean up toast and timeout on unmount
  useEffect(() => {
    return () => {
      if (toastTimeoutId) {
        clearTimeout(toastTimeoutId);
      }
      toastController.hide();
    };
  }, [toastController]);

  return (
    <>
      <ScrollView
        contentContainerStyle={{
          flexGrow: 1,
        }}
        showsVerticalScrollIndicator={false}
      >
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
                { label: "Edit", value: "edit" },
                { label: "Metadata", value: "metadata" },
              ]}
              style={[{ marginVertical: -2 }]}
              selectedValue={mode}
              setSelectedValue={handleModeChange as any}
              disabledValues={livestream ? [] : ["edit"]}
            />
          </View>

          {mode === "edit" && (
            <Text
              style={[p[4], text.white, typography.universal?.xl || text.white]}
            >
              Change your Current Livestream Title
            </Text>
          )}

          {noLivestream ? (
            <View style={[layout.flex.center, p[4]]}>
              <Text style={[text.neutral[400], { fontSize: 16 }]}>
                No active livestream to edit. Start a livestream first!
              </Text>
            </View>
          ) : mode === "metadata" ? (
            // Metadata view
            <View style={[flex.values[1], p[4]]}>
              <ContentMetadataForm
                showUpdateButton={!userIsLive}
                style={{ flex: 1, height: "100%" }}
              />
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
                    @{profile?.handle || "streamer"}
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
                {mode === "edit" && (
                  <View
                    style={[
                      layout.flex.row,
                      layout.flex.alignCenter,
                      w.percent[100],
                    ]}
                  >
                    <View style={[flex.values[1]]}>
                      <Text style={[text.neutral[400], { fontSize: 12 }]}>
                        Updating will not send out notifications to viewers or
                        create a new social media post.
                      </Text>
                    </View>
                  </View>
                )}
              </View>

              {/* Image upload for create mode */}
              {mode === "create" && (
                <ImageUploadComponent
                  selectedImage={selectedImage}
                  onImageSelect={handleImageSelect}
                  onImageRemove={handleImageRemove}
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
            </View>
          )}
        </View>
      </ScrollView>
      {ToastComponent}
    </>
  );
}

export default LivestreamPanel;
