import {
  formatHandleWithAt,
  Text,
  useLivestream,
  zero,
} from "@streamplace/components";
import { useLiveUser } from "hooks/useLiveUser";
import { useEffect, useState } from "react";
import { Pressable, ScrollView, TextInput, View } from "react-native";
import { useStore } from "store";
import { useNewLivestream, useUserProfile } from "store/hooks";

export default function UpdateLivestream() {
  const updateLivestreamRecord = useStore(
    (state) => state.updateLivestreamRecord,
  );

  // Note: Toast functionality removed, would need simple alert replacement
  const userIsLive = useLiveUser();
  const [title, setTitle] = useState("");
  const [loading, setLoading] = useState(false);
  const profile = useUserProfile();
  const livestream = useLivestream();
  const newLivestream = useNewLivestream();

  useEffect(() => {
    if (newLivestream?.record) {
      // Would show toast: "Livestream title updated" with newLivestream.record.title
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
      await updateLivestreamRecord(title, livestream);
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
              { alignItems: "center" },
              { width: "100%", marginTop: -16 },
            ]}
          >
            <Text style={[{ minWidth: 100, textAlign: "left" }]}></Text>
            <View style={zero.flex.values[1]}>
              <Text style={[{ fontSize: 12, color: "#666" }]}>
                Updating will not send out notifications to viewers or create a
                new social media post.
              </Text>
            </View>
          </View>

          <View
            style={[
              { width: "100%" },
              { alignItems: "center" },
              { marginTop: -16 },
            ]}
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
      </View>
    </ScrollView>
  );
}
