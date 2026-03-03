import { TriggerRef } from "@rn-primitives/dropdown-menu";
import { Brush } from "lucide-react-native";
import { useCallback, useContext, useEffect, useRef, useState } from "react";
import { Image, Linking, Platform, Pressable, View } from "react-native";
import { EmoteView } from "streamplace/src/lexicons/types/place/stream/richtext/facet";
import { formatHandle, zero } from "../..";
import { useAvatars } from "../../hooks/useAvatars";
import { useTheme } from "../../ui";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuTrigger,
} from "../ui/dropdown";
import { Text } from "../ui/text";
import { OpenCardContext } from "./user-profile-card";

const EMOTE_IMAGE_SIZE = 28;

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

  const emoteCreatorDid = emote.record.creator;
  const emoteCreatorProfile = useAvatars([emoteCreatorDid || ""])[
    emoteCreatorDid || ""
  ];

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
                    display: "inline",
                    verticalAlign: "top",
                    alignItems: "center",
                  }
                : {}),
              padding: 2,
              margin: -2,
              marginTop: -EMOTE_IMAGE_SIZE,
              top: EMOTE_IMAGE_SIZE * 0.3,
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
                height: EMOTE_IMAGE_SIZE,
                width: EMOTE_IMAGE_SIZE,
              } as any
            }
          />
        </Pressable>
      </DropdownMenuTrigger>
      <DropdownMenuContent style={{ minWidth: 200, maxWidth: 280 }}>
        <View style={[zero.py[3], zero.gap.all[3]]}>
          <View
            style={[
              zero.gap.all[1],
              zero.layout.flex.row,
              zero.layout.flex.alignCenter,
            ]}
          >
            <View style={[zero.layout.flex.align.start]}>
              <Image
                source={{ uri: emote.record.imageUrl }}
                style={[zero.w[16], zero.h[16]]}
                accessibilityLabel={emote.record.alt ?? emote.record.name}
              />
            </View>
            <View style={{}}>
              <Text>:{emote.record.name}:</Text>
              {emoteCreatorProfile ? (
                <Pressable
                  style={[
                    zero.layout.flex.row,
                    zero.layout.flex.alignCenter,
                    zero.gap.column[1],
                  ]}
                  onPress={() => {
                    // just link to bsky for now to get around internal linking issues in components
                    Linking.openURL(
                      `https://bsky.app/profile/${emoteCreatorProfile.did}`,
                    );
                  }}
                >
                  <Brush size={5 * 4} color={theme.colors.primary} />
                  {emoteCreatorProfile.avatar && (
                    <Image
                      source={{ uri: emoteCreatorProfile.avatar }}
                      style={[zero.w[5], zero.h[5], { borderRadius: 999 }]}
                    />
                  )}
                  <Text color="primary">
                    {formatHandle(emoteCreatorProfile)}
                  </Text>
                </Pressable>
              ) : null}
            </View>
          </View>
          {ownerProfile ? (
            <Pressable
              style={[
                zero.layout.flex.row,
                zero.layout.flex.alignCenter,
                zero.gap.column[2],
              ]}
              onPress={() => {
                // just link to bsky for now to get around internal linking issues in components
                Linking.openURL(`https://bsky.app/profile/${ownerProfile.did}`);
              }}
            >
              {ownerProfile.avatar ? (
                <Image
                  source={{ uri: ownerProfile.avatar }}
                  style={[zero.w[5], zero.h[5], { borderRadius: 10 }]}
                />
              ) : (
                <View
                  style={[
                    zero.w[10],
                    zero.h[5],
                    {
                      borderRadius: 10,
                      backgroundColor: theme.colors.mutedForeground,
                    },
                  ]}
                />
              )}
              <Text size="xs" color="muted">
                from @{ownerProfile.handle}
              </Text>
            </Pressable>
          ) : null}
        </View>
      </DropdownMenuContent>
    </DropdownMenu>
  );
};
