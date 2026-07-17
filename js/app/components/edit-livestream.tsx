import {
  Button,
  formatHandleWithAt,
  Text,
  useLivestream,
  useTheme,
  zero,
} from "@streamplace/components";
import ActivityPicker from "components/activity-picker";
import { useLiveUser } from "hooks/useLiveUser";
import { useEffect, useState } from "react";
import { Pressable, ScrollView, TextInput, View } from "react-native";
import { useStore } from "store";
import { useNewLivestream, useUserProfile } from "store/hooks";
import type { PlaceStreamLivestream } from "streamplace";

export default function UpdateLivestream() {
  const updateLivestreamRecord = useStore(
    (state) => state.updateLivestreamRecord,
  );

  // Note: Toast functionality removed, would need simple alert replacement
  const userIsLive = useLiveUser();
  const [title, setTitle] = useState("");
  const [activity, setActivity] = useState<
    PlaceStreamLivestream.Record["activity"] | undefined
  >(undefined);
  const [tags, setTags] = useState<string[]>([]);
  const [tagInput, setTagInput] = useState("");
  const [loading, setLoading] = useState(false);
  const profile = useUserProfile();
  const livestream = useLivestream();
  const newLivestream = useNewLivestream();
  const { theme } = useTheme();

  useEffect(() => {
    if (livestream?.record) {
      const rec = livestream.record as PlaceStreamLivestream.Record;
      setActivity(rec.activity as PlaceStreamLivestream.Record["activity"]);
      setTags(rec.tags ?? []);
    }
  }, [livestream?.uri]);

  useEffect(() => {
    if (newLivestream?.record) {
      setTitle("");
    }
  }, [newLivestream?.record]);

  useEffect(() => {
    if (newLivestream?.error) {
      // Would show toast: "Error updating livestream" with error message
    }
  }, [newLivestream?.error]);

  const disabled = !userIsLive || loading || title === "";

  const handleSubmit = async () => {
    setLoading(true);
    try {
      await updateLivestreamRecord(title, livestream, activity, tags);
    } catch (error) {
      console.error("Error updating livestream:", error);
      // Would show toast: "Error updating livestream"
    } finally {
      setLoading(false);
    }
  };

  const buttonText = loading
    ? "Loading..."
    : !userIsLive
      ? "Waiting for stream to start..."
      : "Update Livestream!";

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
      <Text style={[{ fontSize: 20, fontWeight: "bold" }, zero.pl[4]]}>
        Change your Current Livestream Title
      </Text>
      <View
        style={[
          { width: "100%" },
          { alignSelf: "center" },
          zero.p[4],
          { justifyContent: "center" },
        ]}
      >
        <View style={[{ flex: 2, minWidth: 0 }, { gap: 12 }]}>
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
                    borderColor: theme.colors.border,
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
                      backgroundColor: theme.colors.surface2,
                      borderRadius: 16,
                      paddingHorizontal: 10,
                      paddingVertical: 4,
                      gap: 4,
                    }}
                  >
                    <Text style={{ color: theme.colors.primary, fontSize: 13 }}>
                      {tag}
                    </Text>
                    <Text
                      style={{
                        color: theme.colors.primary,
                        fontSize: 14,
                        lineHeight: 16,
                      }}
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
                    borderColor: theme.colors.border,
                    borderRadius: 8,
                    padding: 10,
                  }}
                />
              )}
            </View>
          </View>

          <View
            style={[
              { flexDirection: "row" },
              { alignItems: "center" },
              { width: "100%", marginTop: -16 },
            ]}
          >
            <Text style={[{ minWidth: 100, textAlign: "left" }]}></Text>
            <View style={zero.flex.values[1]}>
              <Text style={[{ fontSize: 12, color: theme.colors.text3 }]}>
                Updating will not send out notifications to viewers or create a
                new social media post.
              </Text>
            </View>
          </View>

          <View style={[{ width: "100%" }, { marginTop: -16 }]}>
            <Button size="lg" disabled={disabled} onPress={handleSubmit}>
              {buttonText}
            </Button>
          </View>
        </View>
      </View>
    </ScrollView>
  );
}
