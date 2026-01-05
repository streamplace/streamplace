import { useNavigation } from "@react-navigation/native";
import {
  LivestreamProvider,
  Player,
  PlayerProvider,
  Text,
  Trans,
  useAvatars,
  useLivestreamStore,
  useTranslation,
  View,
  zero,
} from "@streamplace/components";
import { overflow } from "@streamplace/components/src/lib/theme/atoms";
import { ChevronLeft } from "lucide-react-native";
import { memo, useEffect, useMemo, useState } from "react";
import { Image, Platform, Pressable, useWindowDimensions } from "react-native";
import { useStore } from "../../store";
import FollowButton from "../follow-button";
import { DesktopUi } from "./desktop-ui";

const { bg, borders, flex, gap, h, layout, mt, position, px, py, r, text, w } =
  zero;

interface SourceType {
  did: string;
  source: string;
}

export const UserOffline = memo(() => {
  console.log("rendering offline");
  const { t } = useTranslation("common");
  const navigation = useNavigation();
  const profile = useLivestreamStore((x) => x.profile);
  const { width, height } = useWindowDimensions();

  const { isSmallScreen, isLandscape, useCompactLayout } = useMemo(() => {
    const isSmall = width < 1250;
    const isLand = width > height;
    return {
      isSmallScreen: isSmall,
      isLandscape: isLand,
      useCompactLayout: isSmall && !isLand,
    };
  }, [width, height]);

  const [recommendedSource, setRecommendedSource] = useState<SourceType | null>(
    null,
  );
  const [isLoadingRecommendation, setIsLoadingRecommendation] = useState(false);
  const getRecommendations = useStore((state) => state.getRecommendations);

  const pfp = useAvatars(profile ? [profile?.did] : []);

  // use the detailed profile from useAvatars
  const detailedProfile = profile ? pfp[profile?.did] : null;

  useEffect(() => {
    if (!profile?.did) return;

    let mounted = true;

    const fetchRecommendation = async () => {
      setIsLoadingRecommendation(true);
      try {
        console.log("fetching recommendations for", profile.did);
        const result = await getRecommendations(profile.did);
        if (!mounted) return;
        if (result.recommendations && result.recommendations.length > 0) {
          // Get the first livestream recommendation
          const firstLivestream = result.recommendations.find(
            (rec) =>
              rec.$type ===
              "place.stream.live.getRecommendations#livestreamRecommendation",
          );
          if (firstLivestream?.did) {
            setRecommendedSource({
              did: firstLivestream.did,
              source: firstLivestream.source || "default",
            });
          }
        }
      } catch (err) {
        console.error("failed to get recommendations", err);
      } finally {
        if (mounted) setIsLoadingRecommendation(false);
      }
    };

    fetchRecommendation();
    return () => {
      mounted = false;
    };
  }, [profile?.did, getRecommendations]);

  if (!profile) {
    return (
      <View style={[flex.values[1], bg.gray[900], layout.flex.center]}>
        <Text size="2xl" color="muted">
          {t("user-offline")}
        </Text>
      </View>
    );
  }

  if (!isLoadingRecommendation && !recommendedSource) {
    return (
      <View
        style={[
          flex.values[1],
          useCompactLayout ? layout.flex.alignCenter : layout.flex.center,
          useCompactLayout ? mt[12] : mt[4],
        ]}
      >
        {/* Back Button and Profile */}
        {Platform.OS !== "web" && (
          <View
            style={[
              {
                padding: 3,
                paddingRight: 8,
                backgroundColor: "rgba(90,90,90, 0.25)",
                borderRadius: 12,
                alignSelf: "flex-start",
                zIndex: 100,
              },
              r.lg,
              layout.position.absolute,
              position.left[4],
              useCompactLayout ? position.top[4] : position.top[0],
            ]}
          >
            <View style={[layout.flex.row, layout.flex.center, gap.all[2]]}>
              <Pressable
                onPress={() => {
                  if (navigation.canGoBack()) {
                    navigation.goBack();
                  } else {
                    navigation.reset({
                      index: 0,
                      routes: [
                        { name: "Home", params: { screen: "StreamList" } },
                      ],
                    });
                  }
                }}
              >
                <ChevronLeft color="white" />
              </Pressable>
              <Image
                source={
                  profile?.did
                    ? { uri: detailedProfile?.avatar }
                    : require("assets/images/goose.png")
                }
                style={[
                  {
                    width: 36,
                    height: 36,
                    backgroundColor: "green",
                  },
                  { borderRadius: 999 },
                  borders.width.thin,
                  borders.color.gray[700],
                ]}
              />
              <Text>{profile?.handle}</Text>
            </View>
          </View>
        )}
        {/* Banner Background */}
        {detailedProfile?.banner && (
          <Image
            blurRadius={10}
            source={{ uri: detailedProfile.banner }}
            style={[
              {
                position: "absolute",
                top: -50,
                left: 0,
                right: 0,
                bottom: 0,
                width: "100%",
                height: "110%",
                opacity: 0.15,
              },
            ]}
          />
        )}

        <View
          style={[
            useCompactLayout ? mt[20] : layout.flex.row,
            gap.all[isLandscape && isSmallScreen ? 3 : 6],
            layout.flex.center,
            px[4],
          ]}
        >
          <View
            style={[
              isLandscape && isSmallScreen ? { width: 280 } : w.percent[100],
              useCompactLayout ? h.auto : h.percent[100],
              useCompactLayout
                ? { maxWidth: "100%" }
                : isLandscape && isSmallScreen
                  ? { maxWidth: 300 }
                  : { maxWidth: 450 },
              isLandscape && isSmallScreen ? px[4] : px[8],
              isLandscape && isSmallScreen ? py[3] : py[6],
              bg.neutral[900],
              r.lg,
              borders.color.neutral[800],
              borders.width.thin,
              gap.row[isLandscape && isSmallScreen ? 1 : 2],
              layout.flex.justify.center,
            ]}
          >
            <Text size={isLandscape && isSmallScreen ? "base" : "xl"}>
              <Trans
                i18nKey="user-offline-no-recommendations"
                ns="common"
                values={{ handle: profile.handle }}
                components={{
                  1: (
                    <Text
                      size={isLandscape && isSmallScreen ? "base" : "xl"}
                      style={[text.gray[400]]}
                    />
                  ),
                  br: <Text />,
                }}
              />
            </Text>
          </View>
        </View>
      </View>
    );
  }

  return (
    <View
      style={[
        flex.values[1],
        useCompactLayout ? layout.flex.alignCenter : layout.flex.center,
        useCompactLayout ? mt[12] : mt[4],
      ]}
    >
      {/* Back Button and Profile */}
      {Platform.OS !== "web" && (
        <View
          style={[
            {
              padding: 3,
              paddingRight: 8,
              backgroundColor: "rgba(90,90,90, 0.25)",
              borderRadius: 12,
              alignSelf: "flex-start",
              zIndex: 100,
            },
            r.lg,
            layout.position.absolute,
            position.left[4],
            useCompactLayout ? position.top[4] : position.top[0],
          ]}
        >
          <View style={[layout.flex.row, layout.flex.center, gap.all[2]]}>
            <Pressable
              onPress={() => {
                if (navigation.canGoBack()) {
                  navigation.goBack();
                } else {
                  navigation.reset({
                    index: 0,
                    routes: [
                      { name: "Home", params: { screen: "StreamList" } },
                    ],
                  });
                }
              }}
            >
              <ChevronLeft color="white" />
            </Pressable>
            <Image
              source={
                profile?.did
                  ? { uri: detailedProfile?.avatar }
                  : require("assets/images/goose.png")
              }
              style={[
                {
                  width: 36,
                  height: 36,
                  backgroundColor: "green",
                },
                { borderRadius: 999 },
                borders.width.thin,
                borders.color.gray[700],
              ]}
            />
            <Text>{profile?.handle}</Text>
          </View>
        </View>
      )}
      {/* Banner Background */}
      {detailedProfile?.banner && (
        <Image
          blurRadius={10}
          source={{ uri: detailedProfile.banner }}
          style={[
            {
              position: "absolute",
              top: -50,
              left: 0,
              right: 0,
              bottom: 0,
              width: "100%",
              height: "110%",
              opacity: 0.15,
            },
          ]}
        />
      )}

      {recommendedSource && (
        <LivestreamProvider src={recommendedSource.did} ignoreOuterContext>
          <View
            style={[
              useCompactLayout ? mt[20] : layout.flex.row,
              gap.all[isLandscape && isSmallScreen ? 3 : 6],
              layout.flex.center,
              px[4],
            ]}
          >
            <View
              style={[
                isLandscape && isSmallScreen ? { width: 280 } : w.percent[100],
                useCompactLayout ? h.auto : h.percent[100],
                useCompactLayout
                  ? { maxWidth: "100%" }
                  : isLandscape && isSmallScreen
                    ? { maxWidth: 280 }
                    : { maxWidth: 400 },
                isLandscape && isSmallScreen ? px[4] : px[8],
                isLandscape && isSmallScreen ? py[3] : py[6],
                bg.neutral[900],
                r.lg,
                borders.color.neutral[800],
                borders.width.thin,
                gap.row[isLandscape && isSmallScreen ? 1 : 2],
                layout.flex.justify.center,
              ]}
            >
              <Text size={isLandscape && isSmallScreen ? "base" : "xl"}>
                <Trans
                  i18nKey="user-offline-message"
                  ns="common"
                  values={{
                    handle: profile.handle,
                    source: recommendedSource.source,
                  }}
                  components={{
                    1: (
                      <Text
                        size={isLandscape && isSmallScreen ? "base" : "xl"}
                        style={[text.gray[400]]}
                      />
                    ),
                  }}
                />
              </Text>
              <View style={[gap.all[1]]}>
                {isLoadingRecommendation ? (
                  <Text style={[text.gray[300]]}>{t("loading")}</Text>
                ) : (
                  <RecommendedSourceInfo />
                )}
              </View>
            </View>
            <View
              style={[
                useCompactLayout
                  ? [w.percent[100], { maxWidth: "100%", aspectRatio: 16 / 9 }]
                  : [
                      flex.values[1],
                      {
                        aspectRatio: 16 / 9,
                        ...(!(isLandscape && isSmallScreen) && {
                          maxWidth: 650,
                          minWidth: 650,
                        }),
                      },
                    ],
                overflow.hidden,
                r.lg,
                overflow.hidden,
                borders.color.neutral[800],
                borders.width.thin,
                bg.black,
                gap.row[2],
              ]}
            >
              {!isLoadingRecommendation && (
                <PlayerProvider>
                  <Player src={recommendedSource.did} embedded={true}>
                    <DesktopUi setIsChatOpen={undefined} />
                  </Player>
                </PlayerProvider>
              )}
            </View>
          </View>
        </LivestreamProvider>
      )}
    </View>
  );
});

const RecommendedSourceInfo = memo(() => {
  const { t } = useTranslation("common");
  const profile = useLivestreamStore((x) => x.profile);
  const viewers = useLivestreamStore((x) => x.viewers);
  const lsInfo = useLivestreamStore((x) => x.livestream);
  const currentUserDID = useStore((state) => state.oauthSession?.did);

  const pfp = useAvatars(profile?.did ? [profile.did] : []);
  const detailedProfile = profile?.did ? pfp[profile.did] : null;

  return (
    <View
      style={[
        layout.flex.column,
        layout.flex.justifyCenter,
        gap.all[4],
        w.percent[100],
      ]}
    >
      <View style={[layout.flex.column, gap.all[4]]}>
        <View style={[layout.flex.row, gap.all[4], layout.flex.alignCenter]}>
          <Image
            source={{ uri: detailedProfile?.avatar || profile?.avatar }}
            style={[
              { width: 48, height: 48, borderRadius: 999 },
              borders.width.thin,
              borders.color.gray[700],
            ]}
          />
          <View style={[flex.values[1]]}>
            <Text weight="bold">
              @{detailedProfile?.handle || profile?.handle}
            </Text>
            <Text style={[text.gray[300]]} size="base">
              {t("viewer-count", { count: viewers || 0 })}
            </Text>
          </View>
        </View>
      </View>
      {profile?.did && (
        <FollowButton
          streamerDID={profile.did}
          currentUserDID={currentUserDID}
        />
      )}
    </View>
  );
});
