import { TriggerRef } from "@rn-primitives/dropdown-menu";
import { useCallback, useContext, useEffect, useRef, useState } from "react";
import { Image, Platform, Pressable, View } from "react-native";
import { EmoteView } from "streamplace/src/lexicons/types/place/stream/richtext/facet";
import { zero } from "../..";
import { useAvatars } from "../../hooks/useAvatars";
import { useTheme } from "../../ui";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuTrigger,
} from "../ui/dropdown";
import { Text } from "../ui/text";
import { OpenCardContext } from "./user-profile-card";

export const EmojiCard = ({
  emote,
  cardKey,
}: {
  emote: EmoteView;
  cardKey: string;
}) => {
  const { theme } = useTheme();
  const { openUri, setOpenUri } = useContext(OpenCardContext);

  const uri = emote.record.uri;
  const isOpen = openUri === cardKey;
  const thisRef = useRef<TriggerRef>(null);
  const [hovered, setHovered] = useState(false);

  const ownerDid = uri.split("/")[2];
  const ownerProfiles = useAvatars([ownerDid]);
  const ownerProfile = ownerProfiles[ownerDid];

  useEffect(() => {
    isOpen ? thisRef.current?.open() : thisRef.current?.close();
  }, [isOpen]);

  const onOpenChange = useCallback(
    (open: boolean) => {
      setOpenUri(open ? cardKey : null);
    },
    [cardKey, setOpenUri],
  );

  return (
    <DropdownMenu onOpenChange={onOpenChange}>
      <DropdownMenuTrigger ref={thisRef} asChild>
        <Pressable
          onPress={() => {}}
          {...(Platform.OS === "web"
            ? {
                onHoverIn: () => setHovered(true),
                onHoverOut: () => setHovered(false),
              }
            : {})}
          // awful awful awful alignment
          style={
            {
              ...(Platform.OS === "web"
                ? {
                    display: "inline-flex",
                    verticalAlign: "middle",
                    alignItems: "center",
                  }
                : {}),
              padding: 2,
              margin: -2,
              marginTop: -28,
              top: 28 * 0.8,
              ...(Platform.OS === "web" && hovered
                ? { backgroundColor: "rgba(255,255,255,0.15)", borderRadius: 6 }
                : {}),
            } as any
          }
        >
          <Image
            source={{ uri: emote.record.imageUrl }}
            accessibilityLabel={emote.name}
            style={
              {
                display: "inline-flex",
                verticalAlign: "middle",
                alignItems: "center",
                height: 28,
                width: 28,
              } as any
            }
          />
        </Pressable>
      </DropdownMenuTrigger>
      <DropdownMenuContent style={{ minWidth: 200, maxWidth: 280 }}>
        <View style={{ padding: 12, gap: 10 }}>
          <View style={{ alignItems: "center" }}>
            <Image
              source={{ uri: emote.record.imageUrl }}
              style={{ width: 64, height: 64 }}
              accessibilityLabel={emote.record.alt ?? emote.record.name}
            />
          </View>
          <View style={{ gap: 2 }}>
            <Text>:{emote.record.name}:</Text>
            {emote.record.alt ? (
              <Text size="sm" color="muted">
                {emote.record.alt}
              </Text>
            ) : null}
          </View>
          {ownerProfile ? (
            <View
              style={[
                zero.layout.flex.row,
                zero.layout.flex.alignCenter,
                zero.gap.column[2],
              ]}
            >
              {ownerProfile.avatar ? (
                <Image
                  source={{ uri: ownerProfile.avatar }}
                  style={{ width: 20, height: 20, borderRadius: 10 }}
                />
              ) : (
                <View
                  style={{
                    width: 20,
                    height: 20,
                    borderRadius: 10,
                    backgroundColor: theme.colors.mutedForeground,
                  }}
                />
              )}
              <Text size="xs" color="muted">
                from @{ownerProfile.handle}
              </Text>
            </View>
          ) : null}
        </View>
      </DropdownMenuContent>
    </DropdownMenu>
  );
};
