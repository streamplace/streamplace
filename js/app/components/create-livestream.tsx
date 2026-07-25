import { formatHandleWithAt, Text, zero } from "@streamplace/components";
import ActivityPicker from "components/activity-picker";
import ThumbnailSelector from "components/thumbnail-selector";
import { useCaptureVideoFrame } from "hooks/useCaptureVideoFrame";
import { useLiveUser } from "hooks/useLiveUser";
import { useEffect, useState } from "react";
import {
  Platform,
  Pressable,
  ScrollView,
  TextInput,
  useWindowDimensions,
  View,
} from "react-native";
import { useStore } from "store";
import { useNewLivestream, useUserProfile } from "store/hooks";
import type { place } from "streamplace";

const isWeb = Platform.OS === "web";

export default function CreateLivestream() {
  const createLivestreamRecord = useStore(
    (state) => state.createLivestreamRecord,
  );
  const streamplaceUrl = useStore((state) => state.url);
  // Note: Toast functionality removed, would need simple alert replacement
  const userIsLive = useLiveUser();
  const [title, setTitle] = useState("");
  const [activity, setActivity] = useState<
    place.stream.livestream.Main["activity"] | undefined
  >(undefined);
  const [tags, setTags] = useState<string[]>([]);
  const [tagInput, setTagInput] = useState("");
  const [loading, setLoading] = useState(false);
  const [customThumbnail, setCustomThumbnail] = useState<Blob | undefined>(
    undefined,
  );
  const profile = useUserProfile();
  const newLivestream = useNewLivestream();
  const captureFrame = useCaptureVideoFrame();
  const { width } = useWindowDimensions();

  // Responsive layout logic
  const isWide = width > 1020;
  const useTwoColumns = isWide;

  useEffect(() => {
    if (newLivestream?.record) {
      setTitle("");
      setActivity(undefined);
      setTags([]);
      setTagInput("");
      setCustomThumbnail(undefined);
    }
  }, [newLivestream?.record]);

  useEffect(() => {
    if (newLivestream?.error) {
      // Would show toast: "Error creating livestream" with error message
    }
  }, [newLivestream?.error]);

  const disabled = !userIsLive || loading || title === "";

  const handleSubmit = async () => {
    setLoading(true);
    try {
      let thumbnailToUse = customThumbnail;
      if (!thumbnailToUse && isWeb && captureFrame) {
        const capturedFrame = await captureFrame(1280, 0.85);
        if (capturedFrame) {
          thumbnailToUse = capturedFrame;
        }
      }

      await createLivestreamRecord(title, thumbnailToUse, activity, tags);
    } catch (error) {
      console.error("Error creating livestream:", error);
      // Would show toast: "Error creating livestream"
    } finally {
      setLoading(false);
    }
  };

  const buttonText = loading
    ? "Loading..."
    : !userIsLive
      ? "Waiting for stream to start..."
      : "Announce Livestream!";

  return (
    <ScrollView
      style={{ width: "60%" }}
      contentContainerStyle={{
        flexGrow: 1,
        justifyContent: "flex-start",
        paddingVertical: 40,
      }}
      showsVerticalScrollIndicator={false}
    >
      <View
        style={[
          { flexDirection: useTwoColumns ? "row" : "column" },
          { gap: useTwoColumns ? 48 : 16 },
          { width: "100%" },
          { maxWidth: useTwoColumns ? 900 : undefined },
          { alignSelf: "center" },
          zero.p[4],
          { alignItems: useTwoColumns ? "flex-start" : "stretch" },
          { justifyContent: "center" },
        ]}
      >
        {/* Left column: labels and fields */}
        <View
          style={[
            { flex: 2, minWidth: 0 },
            { gap: 12 },
            { width: useTwoColumns ? 500 : "100%" },
          ]}
        >
          <View
            style={[
              { flexDirection: "row" },
              { alignItems: "center" },
              { width: "100%" },
            ]}
          >
            <Text
              style={[{ paddingBottom: 8, minWidth: 100, textAlign: "left" }]}
            >
              Streamer
            </Text>
            <Text style={[{ paddingBottom: 8, fontWeight: "bold" }]}>
              {profile && formatHandleWithAt(profile)}
            </Text>
          </View>

          <View
            style={[
              { flexDirection: "row" },
              { alignItems: "center" },
              { width: "100%" },
            ]}
          >
            <Text
              style={[{ paddingBottom: 8, minWidth: 100, textAlign: "left" }]}
            >
              Title
            </Text>
            <View style={zero.flex.values[1]}>
              <TextInput
                value={title}
                onChangeText={setTitle}
                maxLength={140}
                style={[
                  {
                    minHeight: 100,
                    width: "100%",
                    borderWidth: 1,
                    borderColor: "#ccc",
                    borderRadius: 8,
                    padding: 12,
                    textAlignVertical: "top",
                  },
                ]}
                multiline
              />
            </View>
          </View>

          <View
            style={[
              { flexDirection: "row" },
              { alignItems: "flex-start" },
              { width: "100%" },
            ]}
          >
            <Text style={[{ paddingTop: 8, minWidth: 100, textAlign: "left" }]}>
              Activity
            </Text>
            <View style={zero.flex.values[1]}>
              <ActivityPicker value={activity} onChange={setActivity} />
            </View>
          </View>

          <View
            style={[
              { flexDirection: "row" },
              { alignItems: "flex-start" },
              { width: "100%" },
            ]}
          >
            <Text style={[{ paddingTop: 8, minWidth: 100, textAlign: "left" }]}>
              Tags
            </Text>
            <View style={[zero.flex.values[1], { gap: 8 }]}>
              <View style={{ flexDirection: "row", flexWrap: "wrap", gap: 6 }}>
                {tags.map((tag) => (
                  <Pressable
                    key={tag}
                    onPress={() => setTags(tags.filter((t) => t !== tag))}
                    style={{
                      flexDirection: "row",
                      alignItems: "center",
                      backgroundColor: "#e8f0fe",
                      borderRadius: 16,
                      paddingHorizontal: 10,
                      paddingVertical: 4,
                      gap: 4,
                    }}
                  >
                    <Text style={{ color: "#0066cc", fontSize: 13 }}>
                      {tag}
                    </Text>
                    <Text
                      style={{ color: "#0066cc", fontSize: 14, lineHeight: 16 }}
                    >
                      ×
                    </Text>
                  </Pressable>
                ))}
              </View>
              {tags.length < 10 && (
                <TextInput
                  value={tagInput}
                  onChangeText={(v) =>
                    setTagInput(v.replace(/[^a-zA-Z0-9]/g, ""))
                  }
                  placeholder="Add a tag, press Enter"
                  onSubmitEditing={() => {
                    const trimmed = tagInput.trim();
                    if (trimmed && !tags.includes(trimmed)) {
                      setTags([...tags, trimmed]);
                    }
                    setTagInput("");
                  }}
                  style={{
                    borderWidth: 1,
                    borderColor: "#ccc",
                    borderRadius: 8,
                    padding: 10,
                  }}
                />
              )}
            </View>
          </View>

          <View
            style={[{ width: "100%" }, { alignItems: "center" }, zero.mt[4]]}
          >
            <Pressable
              disabled={disabled}
              style={[
                {
                  opacity: disabled ? 0.5 : 1,
                  width: "100%",
                  backgroundColor: "#0066cc",
                  padding: 16,
                  borderRadius: 8,
                  alignItems: "center",
                },
              ]}
              onPress={handleSubmit}
            >
              <Text
                style={{ color: "white", fontSize: 16, fontWeight: "bold" }}
              >
                {buttonText}
              </Text>
            </Pressable>
          </View>
        </View>

        {/* Right column: thumbnail */}
        <View
          style={[
            { flex: 1, minWidth: 0 },
            { gap: 16 },
            { alignItems: "center" },
            { justifyContent: "flex-start" },
            { width: useTwoColumns ? 400 : "100%" },
            {
              marginTop: 12,
              ...(useTwoColumns ? {} : { marginLeft: 40 }),
            },
          ]}
        >
          <View
            style={[
              { flexDirection: "column" },
              { alignItems: "center" },
              { width: "100%" },
            ]}
          >
            <Text
              style={[
                {
                  paddingBottom: 0,
                  lineHeight: 18,
                  fontWeight: "bold",
                  marginBottom: 8,
                },
              ]}
            >
              Custom Thumbnail (Optional)
            </Text>
            <View style={[{ maxWidth: 400, width: "100%" }]}>
              <ThumbnailSelector onThumbnailSelected={setCustomThumbnail} />
            </View>
          </View>
        </View>
      </View>
    </ScrollView>
  );
}
