import { useLivestream, useToast, zero } from "@streamplace/components";
import ThumbnailSelector from "components/thumbnail-selector";
import ButtonSelector from "components/ui/button-selector";
import {
  createLivestreamRecord,
  selectNewLivestream,
  selectUserProfile,
  updateLivestreamRecord,
} from "features/bluesky/blueskySlice";
import { useCaptureVideoFrame } from "hooks/useCaptureVideoFrame";
import { useLiveUser } from "hooks/useLiveUser";
import { useEffect, useState } from "react";
import {
  Platform,
  Pressable,
  ScrollView,
  Text,
  TextInput,
  useWindowDimensions,
  View,
} from "react-native";
import { useAppDispatch, useAppSelector } from "store/hooks";

const { flex, p, px, py, mt, gap, layout, bg, borders, text, r, w } = zero;
const isWeb = Platform.OS === "web";

interface LivestreamPanelProps {
  initialTitle?: string;
  initialThumbnail?: Blob;
  livestreamId?: string; // for edit mode, if needed
  onSuccess?: (record: any) => void;
  onError?: (error: any) => void;
}

export default function LivestreamPanel({
  initialTitle = "",
  initialThumbnail,
  livestreamId,
  onSuccess,
  onError,
}: LivestreamPanelProps) {
  const dispatch = useAppDispatch();
  const userIsLive = useLiveUser();
  const [title, setTitle] = useState(initialTitle);
  const [loading, setLoading] = useState(false);
  const [customThumbnail, setCustomThumbnail] = useState<Blob | undefined>(
    initialThumbnail,
  );
  const profile = useAppSelector(selectUserProfile);
  const newLivestream = useAppSelector(selectNewLivestream);
  const captureFrame = useCaptureVideoFrame();
  const { width } = useWindowDimensions();
  const livestream = useLivestream();

  // Toast
  const { toastController, ToastComponent } = useToast();

  // ButtonSelector state for manual mode switching
  const [mode, setSelectedMode] = useState<string>(
    livestream ? "edit" : "create",
  );
  // If user selects "edit" but no livestream, show disabled message
  const noLivestream = mode === "edit" && !livestream;

  // Responsive layout logic
  const isWide = width > 1020;
  const useTwoColumns = false;

  useEffect(() => {
    if (newLivestream?.record) {
      toastController.show(
        mode === "create" ? "Livestream announced" : "Livestream title updated",
        newLivestream.record.title,
      );
      setTitle("");
      setCustomThumbnail(undefined);
      if (onSuccess) onSuccess(newLivestream.record);
    }
  }, [newLivestream?.record, mode]);
  useEffect(() => {
    if (newLivestream?.error) {
      toastController.show(
        mode === "create"
          ? "Error creating livestream"
          : "Error updating livestream",
        String(newLivestream.error),
      );
      if (onError) onError(newLivestream.error);
    }
  }, [newLivestream?.error, mode]);
  const disabled =
    !userIsLive ||
    loading ||
    title.trim() === "" ||
    (mode === "create" && !profile) ||
    noLivestream;

  const handleSubmit = async () => {
    setLoading(true);
    try {
      if (mode === "create") {
        let thumbnailToUse = customThumbnail;
        if (!thumbnailToUse && isWeb && captureFrame) {
          const capturedFrame = await captureFrame(1280, 0.85);
          if (capturedFrame) {
            thumbnailToUse = capturedFrame;
          }
        }
        await dispatch(
          createLivestreamRecord({
            title,
            customThumbnail: thumbnailToUse,
          }),
        );
      } else {
        await dispatch(
          updateLivestreamRecord({
            title,
            livestream,
          }),
        );
      }
    } catch (error) {
      console.error(
        mode === "create"
          ? "Error creating livestream:"
          : "Error updating livestream:",
        error,
      );
      toastController.show(
        mode === "create"
          ? "Error creating livestream"
          : "Error updating livestream",
        String(error),
      );
      if (onError) onError(error);
    } finally {
      setLoading(false);
    }
  };

  const buttonText = loading
    ? "Loading..."
    : !userIsLive
      ? mode === "create"
        ? "Waiting for stream to start..."
        : "Waiting for stream to start..."
      : mode === "create"
        ? "Announce Livestream!"
        : "Update Livestream!";

  return (
    <>
      {ToastComponent}
      <ScrollView
        contentContainerStyle={{
          flexGrow: 1,
        }}
        showsVerticalScrollIndicator={false}
      >
        <View
          style={[
            flex.values[1],
            bg.gray[800],
            r[3],
            borders.width.thin,
            borders.color.gray[700],
            layout.flex.column,
          ]}
        >
          <View
            style={[
              layout.flex.row,
              layout.flex.spaceBetween,
              layout.flex.alignCenter,
              p[4],
              borders.bottom.width.thin,
              borders.bottom.color.gray[700],
            ]}
          >
            <Text style={[text.white, { fontSize: 18, fontWeight: "600" }]}>
              Live Chat
            </Text>
            <ButtonSelector
              values={[
                { label: "Create", value: "create" },
                { label: "Edit", value: "edit" },
              ]}
              selectedValue={mode}
              setSelectedValue={setSelectedMode}
              disabledValues={livestream ? [] : ["edit"]}
            />
          </View>
          {mode === "edit" && (
            <Text
              style={[px[4], text.white, { fontSize: 20, fontWeight: "bold" }]}
            >
              Change your Current Livestream Title
            </Text>
          )}
          {mode === "edit" && noLivestream ? (
            <View style={[layout.flex.center, p[4]]}>
              <Text style={[text.gray[400], { fontSize: 16 }]}>
                No active livestream to edit. Start a livestream first!
              </Text>
            </View>
          ) : (
            <View
              style={[
                { flexDirection: useTwoColumns ? "row" : "column" },
                useTwoColumns ? gap.row[12] : gap.column[4],
                w.percent[100],
                { alignSelf: "center" },
                p[4],
                useTwoColumns
                  ? { alignItems: "flex-start" }
                  : { alignItems: "stretch" },
                { justifyContent: "center" },
              ]}
            >
              {/* Left column: labels and fields */}
              <View
                style={[
                  flex.values[2],
                  { minWidth: 0 },
                  gap.column[3],
                  w.percent[100],
                ]}
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
                      text.gray[300],
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
                    @{profile?.handle}
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
                      text.gray[300],
                      { minWidth: 100, textAlign: "left", paddingBottom: 8 },
                    ]}
                  >
                    Title
                  </Text>
                  <View style={[flex.values[1]]}>
                    <TextInput
                      value={title}
                      onChangeText={setTitle}
                      style={[
                        p[2],
                        r[1],
                        bg.gray[900],
                        text.white,
                        w.percent[100],
                        { minHeight: 100, fontSize: 16 },
                      ]}
                      maxLength={140}
                      multiline
                    />
                  </View>
                </View>
                {mode === "edit" && (
                  <View
                    style={[
                      layout.flex.row,
                      layout.flex.alignCenter,
                      w.percent[100],
                      { marginTop: -16 },
                    ]}
                  >
                    <View style={[flex.values[1]]}>
                      <Text style={[text.gray[400], { fontSize: 12 }]}>
                        Updating will not send out notifications to viewers or
                        create a new social media post.
                      </Text>
                    </View>
                  </View>
                )}
              </View>
              {/* Right column: thumbnail (only for create mode) */}
              {mode === "create" && (
                <View
                  style={[
                    flex.values[1],
                    { minWidth: 0 },
                    gap.column[4],
                    { alignItems: "center" },
                    { justifyContent: "flex-start" },
                    { marginTop: 12 },
                  ]}
                >
                  <Text
                    style={[
                      text.white,
                      { fontWeight: "bold", marginBottom: 8 },
                    ]}
                  >
                    Custom Thumbnail (Optional)
                  </Text>
                  <View style={[w.percent[100]]}>
                    <ThumbnailSelector
                      onThumbnailSelected={setCustomThumbnail}
                    />
                  </View>
                </View>
              )}
              <View
                style={[
                  w.percent[100],
                  { alignItems: "center" },
                  mode === "edit" ? { marginTop: -16 } : mt[4],
                ]}
              >
                <Pressable
                  disabled={disabled}
                  style={[
                    bg.primary[500],
                    r[1],
                    px[4],
                    py[2],
                    w.percent[100],
                    { alignItems: "center" },
                    { opacity: disabled ? 0.5 : 1 },
                  ]}
                  onPress={handleSubmit}
                >
                  <Text
                    style={[text.white, { fontSize: 16, fontWeight: "bold" }]}
                  >
                    {buttonText}
                  </Text>
                </Pressable>
              </View>
            </View>
          )}
        </View>
      </ScrollView>
    </>
  );
}
