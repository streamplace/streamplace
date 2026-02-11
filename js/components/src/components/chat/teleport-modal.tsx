import { Check } from "lucide-react-native";
import React, { useEffect, useMemo, useState } from "react";
import { Image, Pressable, ScrollView, View } from "react-native";
import { PlaceStreamLivestream } from "streamplace";
import { useAvatars, zero } from "../..";
import { useStreamplaceStore } from "../../streamplace-store";
import {
  Button,
  DialogFooter,
  Input,
  ResponsiveDialog,
  Text,
  useTheme,
} from "../ui";

interface TeleportModalProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onSubmit: (targetHandle: string, countdownSeconds: number) => void;
}

export const TeleportModal: React.FC<TeleportModalProps> = ({
  open,
  onOpenChange,
  onSubmit,
}) => {
  const [searchQuery, setSearchQuery] = useState("");
  const [selectedStream, setSelectedStream] =
    useState<PlaceStreamLivestream.LivestreamView | null>(null);
  const [countdownSeconds, setCountdownSeconds] = useState("10");

  const { theme } = useTheme();

  const liveUsersCache = useStreamplaceStore((state) => state.liveUsers);
  const liveUsersLoading = useStreamplaceStore(
    (state) => state.liveUsersLoading,
  );

  const [liveUsers, setLiveUsers] = useState(liveUsersCache);

  useEffect(() => {
    setLiveUsers(liveUsersCache);
  }, [liveUsersCache]);

  const profiles = useAvatars(liveUsers?.map((u) => u.author?.did || "") || []);

  const filteredStreams = useMemo(() => {
    if (!liveUsers) return [];
    if (!searchQuery.trim()) return liveUsers;

    const query = searchQuery.toLowerCase();
    return liveUsers.filter(
      (stream) =>
        stream.author?.handle?.toLowerCase().includes(query) ||
        stream.author?.displayName?.toLowerCase().includes(query),
    );
  }, [liveUsers, searchQuery]);

  const handleCancel = () => {
    setSearchQuery("");
    setSelectedStream(null);
    setCountdownSeconds("10");
    onOpenChange(false);
  };

  const handleSubmit = () => {
    if (!selectedStream?.author?.handle) return;

    const countdown = parseInt(countdownSeconds, 10);
    if (isNaN(countdown) || countdown < 5 || countdown > 300) {
      return;
    }

    onSubmit(selectedStream.author.handle, countdown);
    handleCancel();
  };

  return (
    <ResponsiveDialog
      open={open}
      onOpenChange={onOpenChange}
      title="Teleport to Streamer"
      showCloseButton
      variant="default"
      size="xl"
      dismissible={false}
    >
      <View style={[zero.py[2]]}>
        <View style={[zero.mb[4]]}>
          <Input
            value={searchQuery}
            onChangeText={setSearchQuery}
            placeholder="Search by handle..."
            autoCapitalize="none"
            autoCorrect={false}
          />
        </View>

        {liveUsersLoading && !liveUsers ? (
          <View style={[zero.py[8], { alignItems: "center" }]}>
            <Text style={[{ color: theme.colors.textMuted }]}>
              Loading live users...
            </Text>
          </View>
        ) : filteredStreams.length === 0 ? (
          <View style={[zero.py[8], { alignItems: "center" }]}>
            <Text style={[{ color: theme.colors.textMuted }]}>
              {searchQuery
                ? "No matching live users found"
                : "No live users found"}
            </Text>
          </View>
        ) : (
          <ScrollView style={[{ maxHeight: 400 }]}>
            <View
              style={[
                {
                  flexDirection: "row",
                  flexWrap: "wrap",
                  gap: 12,
                },
              ]}
            >
              {filteredStreams.map((stream) => {
                const isSelected = selectedStream?.uri === stream.uri;
                const profile = profiles[stream.author?.did];

                return (
                  <Pressable
                    key={stream.uri}
                    onPress={() => setSelectedStream(stream)}
                    style={[
                      {
                        width: "48%",
                        minWidth: 200,
                      },
                    ]}
                  >
                    <View
                      style={[
                        {
                          backgroundColor: theme.colors.muted,
                          borderRadius: 12,
                          overflow: "hidden",
                          borderWidth: 2,
                          borderColor: isSelected
                            ? theme.colors.primary
                            : "transparent",
                        },
                      ]}
                    >
                      <View
                        style={[
                          {
                            width: "100%",
                            aspectRatio: 16 / 9,
                            backgroundColor: theme.colors.card,
                            position: "relative",
                          },
                        ]}
                      >
                        <Image
                          source={{
                            uri:
                              "/api/playback/" +
                              stream.author.did +
                              "/stream.jpg",
                          }}
                          style={{
                            width: "100%",
                            height: "100%",
                          }}
                          resizeMode="cover"
                        />
                        {isSelected && (
                          <View
                            style={[
                              {
                                position: "absolute",
                                top: 8,
                                right: 8,
                                backgroundColor: theme.colors.primary,
                                borderRadius: 999,
                                width: 24,
                                height: 24,
                                alignItems: "center",
                                justifyContent: "center",
                                boxShadow: "0 2px 4px rgba(0, 0, 0, 0.6)",
                              },
                            ]}
                          >
                            <Check size={16} color="white" />
                          </View>
                        )}
                        {stream.viewerCount && (
                          <View
                            style={[
                              {
                                position: "absolute",
                                top: 8,
                                left: 8,
                                backgroundColor: "rgba(0, 0, 0, 0.75)",
                                borderRadius: 999,
                                paddingHorizontal: 8,
                                paddingVertical: 4,
                              },
                            ]}
                          >
                            <Text style={[{ fontSize: 12, color: "white" }]}>
                              {stream.viewerCount.count} viewer
                              {stream.viewerCount.count !== 1 ? "s" : ""}
                            </Text>
                          </View>
                        )}
                      </View>
                      <View
                        style={[
                          {
                            padding: 12,
                            flexDirection: "row",
                            gap: 8,
                            alignItems: "center",
                          },
                        ]}
                      >
                        <View
                          style={[
                            {
                              width: 40,
                              height: 40,
                              borderRadius: 20,
                              overflow: "hidden",
                              backgroundColor: theme.colors.card,
                              flexShrink: 0,
                            },
                          ]}
                        >
                          {profile?.avatar ? (
                            <Image
                              source={{
                                uri: profile.avatar,
                              }}
                              style={{ width: "100%", height: "100%" }}
                              resizeMode="cover"
                            />
                          ) : (
                            <View
                              style={{
                                width: "100%",
                                height: "100%",
                                backgroundColor: theme.colors.muted,
                              }}
                            />
                          )}
                        </View>

                        {/* Text */}
                        <View style={[{ flex: 1, minWidth: 0 }]}>
                          <Text
                            numberOfLines={1}
                            ellipsizeMode="tail"
                            style={[{ fontWeight: "600" }]}
                          >
                            {stream.author?.handle}
                          </Text>
                          {stream.record.title ? (
                            <Text
                              numberOfLines={1}
                              ellipsizeMode="tail"
                              style={[
                                {
                                  fontSize: 12,
                                  color: theme.colors.textMuted,
                                },
                              ]}
                            >
                              {stream.record.title as any}
                            </Text>
                          ) : null}
                        </View>
                      </View>
                    </View>
                  </Pressable>
                );
              })}
            </View>
          </ScrollView>
        )}
      </View>
      <DialogFooter>
        <Button width="min" variant="secondary" onPress={handleCancel}>
          <Text>Cancel</Text>
        </Button>
        <Button
          width="min"
          variant="primary"
          onPress={handleSubmit}
          disabled={!selectedStream}
        >
          <Text>Teleport</Text>
        </Button>
      </DialogFooter>
    </ResponsiveDialog>
  );
};
