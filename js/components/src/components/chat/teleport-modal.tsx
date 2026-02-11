import React, { useEffect, useMemo, useState } from "react";
import { Image, Pressable, ScrollView, View } from "react-native";
import { PlaceStreamLivestream } from "streamplace";
import { useAvatars, zero } from "../..";
import { useStreamplaceStore } from "../../streamplace-store";
import {
  Button,
  DialogFooter,
  Input,
  MenuGroup,
  MenuItem,
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
      size="md"
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
            <MenuGroup>
              {filteredStreams.map((stream) => (
                <Pressable
                  key={stream.uri}
                  onPress={() => setSelectedStream(stream)}
                >
                  <MenuItem
                    style={
                      [
                        selectedStream?.uri === stream.uri && {
                          backgroundColor: "rgba(0, 122, 255, 0.1)",
                        },
                        zero.layout.flex.spaceBetween,
                        zero.r.md,
                        zero.flex[1],
                        zero.gap.all[2],
                        { width: "100%" },
                      ] as any
                    }
                  >
                    <Image
                      source={{
                        uri: profiles[stream.author.did]?.avatar,
                        width: 50,
                        height: 50,
                      }}
                      style={[zero.r.full]}
                    />
                    <View
                      style={[
                        zero.layout.flex.row,
                        zero.gap.all[2],
                        zero.layout.flex.alignCenter,
                        { flex: 1, minWidth: 0, width: "100%" },
                      ]}
                    >
                      <View
                        style={[
                          zero.layout.flex.column,
                          { flex: 1, minWidth: 0 },
                        ]}
                      >
                        <Text numberOfLines={1} ellipsizeMode="tail">
                          {stream.author?.handle}
                        </Text>
                        {stream.record.title ? (
                          <Text
                            color="muted"
                            ellipsizeMode="tail"
                            numberOfLines={1}
                          >
                            {(stream.record.title as any) || ""}
                          </Text>
                        ) : null}
                        {stream.viewerCount && (
                          <Text color="muted">
                            {stream.viewerCount.count} viewer
                            {stream.viewerCount.count !== 1 ? "s" : ""}
                          </Text>
                        )}
                      </View>
                    </View>
                  </MenuItem>
                </Pressable>
              ))}
            </MenuGroup>
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
