import Chat from "components/chat/chat";
import ChatBox from "components/chat/chat-box";
import Loading from "components/loading/loading";
import { Player } from "components/player/player";
import { PlayerProps } from "components/player/props";
import PlayerProvider from "components/player/provider";
import Popup from "components/popup";
import Viewers from "components/viewers";
import { usePlayer } from "features/player/playerSlice";
import {
  selectTelemetry,
  telemetryOpt,
} from "features/streamplace/streamplaceSlice";
import { useKeyboard } from "hooks/useKeyboard";
import usePlatform from "hooks/usePlatform";
import { useCallback, useEffect, useState } from "react";
import {
  LayoutChangeEvent,
  View as RNView,
  SafeAreaView,
  Linking,
} from "react-native";
import { useAppDispatch, useAppSelector } from "store/hooks";
import {
  Button,
  H2,
  H3,
  isWeb,
  Text,
  useWindowDimensions,
  View,
} from "tamagui";
import { MessageCircleOff, MessageCircleMore } from "@tamagui/lucide-icons";
import FollowButton from "components/follow-button";
import { useToastController } from "@tamagui/toast";
import { selectProfiles, getProfile } from "features/bluesky/blueskySlice";
import storage from "storage";
import { useFullscreen } from "contexts/FullscreenContext";

export default function Livestream(props: Partial<PlayerProps>) {
  return (
    <PlayerProvider {...props}>
      <LivestreamInner {...props} />
    </PlayerProvider>
  );
}

export function LivestreamInner(props: Partial<PlayerProps>) {
  const telemetry = useAppSelector(selectTelemetry);
  const player = useAppSelector(usePlayer());
  const profiles = useAppSelector(selectProfiles);
  const toast = useToastController();

  const { src, ...extraProps } = props;
  const dispatch = useAppDispatch();
  const { width, height } = useWindowDimensions();
  const video = player.segment?.video?.[0];
  const [videoWidth, setVideoWidth] = useState(0);
  const [videoHeight, setVideoHeight] = useState(0);
  const { keyboardHeight } = useKeyboard();
  const { isIOS } = usePlatform();

  const [outerHeight, setOuterHeight] = useState(0);
  const [innerHeight, setInnerHeight] = useState(0);
  const [isChatVisible, setIsChatVisible] = useState(true);
  const [currentUserDID, setCurrentUserDID] = useState<string | null>(null);
  const { fullscreen, setFullscreen } = useFullscreen();

  const streamerDID = player.livestream?.author?.did;
  const streamerProfile = streamerDID ? profiles[streamerDID] : undefined;
  const streamerHandle = streamerProfile?.handle;

  // this would all be really easy if i had library that would give me the
  // safe area view height and width but i don't. so let's measure
  const onInnerLayout = useCallback((event: LayoutChangeEvent) => {
    const { width, height } = event.nativeEvent.layout;
    setInnerHeight(height);
  }, []);

  const onOuterLayout = useCallback((event: LayoutChangeEvent) => {
    const { width, height } = event.nativeEvent.layout;
    setOuterHeight(height);
  }, []);

  useEffect(() => {
    if (video) {
      const ratio = video.width / width;
      setVideoWidth(video.width / ratio);
      setVideoHeight(video.height / ratio);
    }
  }, [video, width, height]);

  useEffect(() => {
    getCurrentUserDID().then((did) => {
      console.log("currentUserDID:", did);
      setCurrentUserDID(did);
    });
  }, []);

  useEffect(() => {
    if (streamerDID && !streamerProfile) {
      dispatch(getProfile(streamerDID));
    }
  }, [streamerDID, streamerProfile, dispatch]);

  let slideKeyboard = 0;
  if (isIOS && keyboardHeight > 0) {
    slideKeyboard = -keyboardHeight + (outerHeight - innerHeight);
  }

  const handleFollowChange = (isFollowing: boolean) => {
    if (!streamerHandle) return;
    if (isFollowing) {
      toast.show(`You are now following @${streamerHandle}`);
    } else {
      toast.show(`You have unfollowed @${streamerHandle}`);
    }
  };

  if (fullscreen) {
    return (
      <RNView style={{ flex: 1 }}>
        <Player
          telemetry={telemetry === true}
          src={src}
          fullscreen={fullscreen}
          setFullscreen={setFullscreen}
          {...extraProps}
        />
      </RNView>
    );
  }

  return (
    <RNView style={{ flex: 1 }}>
      <SafeAreaView style={{ flex: 1 }} onLayout={onOuterLayout}>
        <RNView
          style={{ flex: 1, position: "relative" }}
          onLayout={onInnerLayout}
        >
          {videoWidth === 0 && (
            <View
              f={1}
              position="absolute"
              top={0}
              left={0}
              right={0}
              bottom={0}
            >
              <Loading />
            </View>
          )}
          {telemetry === null && (
            <Popup
              onClose={() => {
                dispatch(telemetryOpt(false));
              }}
              containerProps={{
                bottom: "$8",
                zIndex: 1000,
              }}
              bubbleProps={{
                cursor: "pointer",
                backgroundColor: "$accentBackground",
                gap: "$3",
                maxWidth: 400,
              }}
            >
              <H3 textAlign="center">Player Telemetry</H3>
              <Text>
                Streamplace is beta software and it helps us out to have the
                player report back on how playback is working. Would you like to
                opt in to optional player telemetry?
              </Text>
              <View flexDirection="row" gap="$2" f={1}>
                <Button
                  f={3}
                  backgroundColor="$accentColor"
                  onPress={() => {
                    dispatch(telemetryOpt(true));
                  }}
                >
                  Opt in
                </Button>
                <Button
                  f={3}
                  onPress={() => {
                    dispatch(telemetryOpt(false));
                  }}
                >
                  Opt out
                </Button>
              </View>
            </Popup>
          )}
          <View
            f={1}
            opacity={videoWidth === 0 ? 0 : 1}
            flexDirection="column"
            $gtXs={{ flexDirection: "row" }}
            zIndex={2}
          >
            <View
              width={videoWidth}
              height={videoHeight}
              maxHeight="100%"
              fs={0}
              $gtXs={{ fs: 1 }}
              zIndex={2}
            >
              <Player
                telemetry={telemetry === true}
                src={src}
                fullscreen={fullscreen}
                setFullscreen={setFullscreen}
                {...extraProps}
              />
              <View
                height={100}
                fg={0}
                p="$4"
                display="none"
                flexDirection="column"
                $gtXs={{ display: "flex" }}
              >
                <View
                  flexDirection="row"
                  alignItems="center"
                  justifyContent="space-between"
                  width="100%"
                >
                  <View
                    flexDirection="row"
                    alignItems="center"
                    gap="$2"
                    minWidth={0}
                  >
                    {streamerDID && !streamerHandle ? (
                      // Skeleton loader for handle
                      <Text>&nbsp;</Text>
                    ) : (
                      streamerHandle && (
                        <Text
                          onPress={() =>
                            Linking.openURL(
                              `https://bsky.app/profile/${streamerHandle}`,
                            )
                          }
                          aria-label={`View @${streamerHandle} on Bluesky`}
                          style={{ cursor: "pointer" }}
                        >
                          {`@${streamerHandle}`}
                        </Text>
                      )
                    )}
                    {streamerDID && streamerHandle && currentUserDID && (
                      <FollowButton
                        streamerDID={streamerDID}
                        currentUserDID={currentUserDID}
                        onFollowChange={handleFollowChange}
                      />
                    )}
                  </View>
                  <View flexDirection="row" alignItems="center" gap="$2">
                    <Viewers viewers={player.viewers ?? 0} />
                    <Button
                      backgroundColor="transparent"
                      onPress={() => setIsChatVisible(!isChatVisible)}
                      marginLeft="$2"
                    >
                      {isChatVisible ? (
                        <MessageCircleOff size={22} />
                      ) : (
                        <MessageCircleMore size={22} />
                      )}
                    </Button>
                  </View>
                </View>
                <View width="100%" marginTop={4}>
                  <H2
                    maxWidth="100%"
                    lineHeight={32}
                    numberOfLines={
                      typeof window !== "undefined" && window.innerWidth < 600
                        ? 1
                        : undefined
                    }
                  >
                    {player.livestream?.record.title}
                  </H2>
                </View>
              </View>
            </View>

            <View
              f={1}
              fg={1}
              zIndex={1}
              $gtXs={{
                width: isChatVisible ? 380 : 0,
                fb: isChatVisible ? 380 : 0,
                fs: 0,
                borderLeftColor: "#666",
                borderLeftWidth: isChatVisible ? 1 : 0,
                overflow: "hidden",
              }}
              backgroundColor="$background2"
              animation={"quick"}
              transform={
                isIOS
                  ? [
                      {
                        translateY: slideKeyboard,
                      },
                    ]
                  : undefined
              }
            >
              {/* Native potrait view: first row = handle/follow/viewers, second row = title */}
              <View
                $gtXs={{ display: "none" }}
                flexDirection="column"
                borderBottomColor="#666"
                borderBottomWidth={1}
                borderTopColor="#666"
                borderTopWidth={1}
                zIndex={1}
              >
                <View
                  flexDirection="row"
                  alignItems="center"
                  gap="$2"
                  paddingTop="$1"
                  paddingBottom="$1"
                  paddingLeft="$3"
                  paddingRight="$3"
                  justifyContent="space-between"
                >
                  <View
                    style={{
                      flexDirection: "row",
                      alignItems: "center",
                      flex: 1,
                      gap: 8,
                      minWidth: 0,
                    }}
                  >
                    {streamerDID && !streamerHandle ? (
                      // Skeleton loader for handle
                      <Text>&nbsp;</Text>
                    ) : (
                      streamerHandle && (
                        <Text
                          onPress={() =>
                            Linking.openURL(
                              `https://bsky.app/profile/${streamerHandle}`,
                            )
                          }
                          aria-label={`View @${streamerHandle} on Bluesky`}
                          style={{ cursor: "pointer" }}
                          numberOfLines={1}
                          ellipsizeMode="tail"
                        >
                          {`@${streamerHandle}`}
                        </Text>
                      )
                    )}
                    {streamerDID && streamerHandle && currentUserDID && (
                      <FollowButton
                        streamerDID={streamerDID}
                        currentUserDID={currentUserDID}
                        onFollowChange={handleFollowChange}
                      />
                    )}
                  </View>
                  <View style={{ alignItems: "flex-end" }}>
                    <Viewers viewers={player.viewers ?? 0} />
                  </View>
                </View>
                <View
                  paddingLeft="$3"
                  paddingRight="$3"
                  paddingBottom="$3"
                  marginTop={-15}
                >
                  <Text fontSize={18} numberOfLines={1} ellipsizeMode="tail">
                    {player.livestream?.record.title}
                  </Text>
                </View>
              </View>
              <Chat
                isChatVisible={isChatVisible}
                setIsChatVisible={setIsChatVisible}
              />
              <View>
                <ChatBox
                  isChatVisible={isChatVisible}
                  setIsChatVisible={setIsChatVisible}
                />
              </View>
            </View>
          </View>
        </RNView>
      </SafeAreaView>
    </RNView>
  );
}

async function getCurrentUserDID(): Promise<string | null> {
  try {
    const did = await storage.getItem(
      `${isWeb ? "@@atproto/oauth-client-browser(sub)" : "did"}`,
    );
    if (did) {
      return did;
    }
    console.debug("Could not find user DID");
    return null;
  } catch (err) {
    console.error("[ERROR] Failed to get current user DID:", err);
    return null;
  }
}
