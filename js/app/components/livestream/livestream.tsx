import {
  LivestreamProvider,
  useLivestream,
  useProfile,
  useSegment,
  useViewers,
} from "@streamplace/components";
import { MessageCircleMore, MessageCircleOff } from "@tamagui/lucide-icons";
import { useToastController } from "@tamagui/toast";
import Chat from "components/chat/chat";
import ChatBox from "components/chat/chat-box";
import FollowButton from "components/follow-button";
import Avatar from "components/home/avatar";
import Loading from "components/loading/loading";
import { Player } from "components/player/player";
import { PlayerProps } from "components/player/props";
import Timer from "components/timer";
import Viewers from "components/viewers";
import { useFullscreen } from "contexts/FullscreenContext";
import {
  setSidebarHidden,
  setSidebarUnhidden,
} from "features/base/sidebarSlice";
import { getProfile } from "features/bluesky/blueskySlice";
import useAvatars from "hooks/useAvatars";
import { useKeyboard } from "hooks/useKeyboard";
import usePlatform from "hooks/usePlatform";
import { useCallback, useEffect, useState } from "react";
import {
  LayoutChangeEvent,
  Linking,
  View as RNView,
  SafeAreaView,
} from "react-native";
import storage from "storage";
import { useAppDispatch } from "store/hooks";
import {
  Button,
  isWeb,
  ScrollView,
  Text,
  useWindowDimensions,
  View,
} from "tamagui";

export default function Livestream(props: Partial<PlayerProps>) {
  if (props.src === undefined) {
    console.error("Livestream: src prop is required");
    return <Text>Source is undefined</Text>;
  }
  return (
    <LivestreamProvider src={props.src} {...props}>
      <LivestreamInner {...props} />
    </LivestreamProvider>
  );
}

export function LivestreamInner(props: Partial<PlayerProps>) {
  const toast = useToastController();
  const viewers = useViewers();

  const { src, ...extraProps } = props;
  const dispatch = useAppDispatch();
  const { width, height } = useWindowDimensions();
  const segment = useSegment();
  const video = segment?.video?.[0];
  const [videoWidth, setVideoWidth] = useState(0);
  const [videoHeight, setVideoHeight] = useState(0);
  const { keyboardHeight } = useKeyboard();
  const { isIOS } = usePlatform();

  const [outerHeight, setOuterHeight] = useState(0);
  const [innerHeight, setInnerHeight] = useState(0);
  const [isChatVisible, setIsChatVisible] = useState(true);
  const [offline, setOffline] = useState(true);
  const [currentUserDID, setCurrentUserDID] = useState<string | null>(null);
  const { fullscreen, setFullscreen } = useFullscreen();

  useEffect(() => {
    if (fullscreen) {
      dispatch(setSidebarHidden());
    } else {
      dispatch(setSidebarUnhidden());
    }
  }, [setFullscreen]);

  const livestream = useLivestream();
  const streamerProfile = useProfile();

  const streamerDID = livestream?.author?.did;
  const streamerHandle = streamerProfile?.handle;
  const startTime = livestream?.record?.createdAt
    ? new Date(livestream?.record?.createdAt)
    : undefined;

  const didArr = livestream?.author?.did ? [livestream?.author?.did] : [];

  const avi = useAvatars(didArr)[didArr[0]];

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

  useEffect(() => {
    // 10 second cut off for segements
    const cuttOffDate = new Date(Date.now() - 10 * 1000);
    // 15 second cut off if segment start time not found
    const startTime = segment?.startTime
      ? new Date(segment?.startTime)
      : new Date(Date.now() - 15 * 1000);

    if (startTime > cuttOffDate) {
      setOffline(false);
    }
  }, [segment]);

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

  const MainView = width < 660 ? View : ScrollView;
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
          <View
            f={1}
            opacity={videoWidth === 0 ? 0 : 1}
            flexDirection="column"
            $gtXs={{ flexDirection: "row" }}
            zIndex={2}
          >
            <MainView
              width={videoWidth}
              height="100%"
              maxHeight={videoHeight}
              fs={0}
              $gtXs={{ fs: 1, maxHeight: "100%" }}
              zIndex={2}
            >
              <View
                maxHeight={height}
                $gtLg={{ maxHeight: height }}
                $gtXxl={{ maxHeight: height }}
                $platform-ios={{
                  height: videoHeight,
                }}
              >
                <Player
                  src={src}
                  fullscreen={fullscreen}
                  setFullscreen={setFullscreen}
                  {...extraProps}
                />
              </View>
              <View
                fg={0}
                px="$4"
                py="$2"
                flexDirection="row"
                justifyContent="space-between"
                maxWidth="100%"
                bg="$black3"
                borderBottomWidth="$0.5"
                borderTopWidth="$0.5"
                borderColor="$black5"
                $gtXs={{
                  bg: "$colorTransparent",
                  py: "$4",
                  borderBottomWidth: 0,
                  borderTopWidth: 0,
                }}
              >
                <View
                  flexDirection="row"
                  alignItems="flex-start"
                  justifyContent="space-between"
                >
                  <View
                    flexDirection="row"
                    alignItems="center"
                    gap="$3"
                    minWidth={0}
                    flexShrink={1}
                    overflow="hidden"
                  >
                    <Avatar src={avi?.avatar} />
                    <View
                      flexDirection="column"
                      alignItems="flex-start"
                      gap="$2"
                      minWidth={0}
                      overflow="hidden"
                    >
                      <View
                        flexDirection="row"
                        alignItems="center"
                        flexShrink={1}
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
                              hoverStyle={{
                                color: "$blue11",
                              }}
                              aria-label={`View @${streamerHandle} on Bluesky`}
                              style={{ cursor: "pointer" }}
                              ellipse={true}
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
                      <Text
                        fontSize="$6"
                        numberOfLines={1}
                        ellipse={true}
                        maxWidth="100%"
                        minWidth={0}
                        flexShrink={1}
                      >
                        {livestream?.record.title}
                      </Text>
                    </View>
                  </View>
                </View>
                <View
                  flexDirection="row"
                  alignItems="center"
                  gap="$2"
                  display="none"
                  $gtXs={{ display: "flex" }}
                >
                  {startTime instanceof Date && !offline && (
                    <Timer start={startTime} />
                  )}
                  <Viewers viewers={viewers ?? 0} />
                  <Button
                    backgroundColor="transparent"
                    onPress={() => setIsChatVisible(!isChatVisible)}
                    marginLeft="$2"
                    display="none"
                    $gtXs={{ display: "flex" }}
                  >
                    {isChatVisible ? (
                      <MessageCircleOff size={22} />
                    ) : (
                      <MessageCircleMore size={22} />
                    )}
                  </Button>
                </View>
              </View>
            </MainView>

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
