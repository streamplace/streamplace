import { useNavigation } from "@react-navigation/native";
import { Button, useTheme, zero } from "@streamplace/components";
import { EmptyState, EmptyStateTile } from "components/empty-state";
import Title from "components/title";
import VideoCard from "components/video/video-card";
import useAvatars from "hooks/useAvatars";
import { useVideoList, VideoView } from "hooks/useVideoList";
import { AlertCircle, Clapperboard, Upload } from "lucide-react-native";
import { useMemo } from "react";
import {
  ActivityIndicator,
  FlatList,
  Platform,
  useWindowDimensions,
  View,
} from "react-native";
import { useSafeAreaInsets } from "react-native-safe-area-context";

const GAP = 16;
const MAX_WIDTH = 1600;

function getColumns(width: number): number {
  if (width >= 1500) return 4;
  if (width >= 1100) return 3;
  if (width >= 680) return 2;
  return 1;
}

export default function VideoListScreen({
  route,
}: {
  route?: { params?: { did?: string } };
}) {
  const repo = route?.params?.did;
  const { theme } = useTheme();
  const navigation = useNavigation();
  const { width } = useWindowDimensions();
  const safeAreaInsets = useSafeAreaInsets();
  const { videos, loading, refreshing, error, hasMore, loadMore, refresh } =
    useVideoList(repo);

  const columns = getColumns(Math.min(width, MAX_WIDTH));

  // Bulk-resolve author avatars so individual cards don't each fire a fetch.
  const dids = useMemo(
    () => Array.from(new Set(videos.map((v) => v.author.did))),
    [videos],
  );
  const avatars = useAvatars(dids);

  // The per-user page borrows the uploader's handle from the first row for an
  // in-list heading. The global page omits it — the nav bar already says
  // "Videos", so a second "Videos" heading would just be redundant.
  const heading =
    repo && videos[0]?.author
      ? `@${videos[0].author.handle || repo}`
      : undefined;

  const renderItem = ({ item }: { item: VideoView }) => (
    <View style={{ flex: 1 / columns }}>
      <VideoCard video={item} avatarUrl={avatars[item.author.did]?.avatar} />
    </View>
  );

  return (
    <FlatList
      // numColumns can't change without a fresh list instance
      key={`vlist-${columns}`}
      data={videos}
      keyExtractor={(item) => item.uri}
      renderItem={renderItem}
      numColumns={columns}
      columnWrapperStyle={columns > 1 ? { gap: GAP } : undefined}
      contentContainerStyle={{
        // flexGrow lets the empty state fill and center vertically; harmless
        // once the list has items.
        flexGrow: 1,
        gap: GAP,
        paddingHorizontal: GAP,
        paddingBottom:
          (Platform.OS !== "web" ? 64 : 0) + safeAreaInsets.bottom + GAP,
        paddingTop: Platform.OS === "ios" ? 96 : GAP,
        maxWidth: MAX_WIDTH,
        width: "100%",
        alignSelf: "center",
      }}
      {...(Platform.OS !== "web" ? { refreshing, onRefresh: refresh } : {})}
      onEndReached={loadMore}
      onEndReachedThreshold={0.6}
      ListHeaderComponent={
        heading ? (
          <View style={[zero.my[4]]}>
            <Title>{heading}</Title>
          </View>
        ) : null
      }
      ListEmptyComponent={
        loading ? (
          <View
            style={{
              flex: 1,
              minHeight: 320,
              alignItems: "center",
              justifyContent: "center",
            }}
          >
            <ActivityIndicator size="large" color={theme.colors.text3} />
          </View>
        ) : (
          <EmptyState
            illustration={
              error ? (
                <EmptyStateTile icon={AlertCircle} tone={theme.colors.danger} />
              ) : (
                <EmptyStateTile icon={Clapperboard} />
              )
            }
            title={error ? "Couldn't load videos" : "No videos yet"}
            subtitle={
              error
                ? error
                : repo
                  ? "This channel hasn't published any videos."
                  : "Upload a video and it'll show up here — processed, titled, and ready to share."
            }
            action={
              !repo && !error ? (
                <Button
                  width="min"
                  leftIcon={<Upload size={16} />}
                  onPress={() =>
                    (navigation as any).navigate("HomeTab", {
                      screen: "Upload",
                    })
                  }
                >
                  Upload video
                </Button>
              ) : undefined
            }
          />
        )
      }
      ListFooterComponent={
        hasMore && videos.length > 0 ? (
          <View style={{ paddingVertical: 24, alignItems: "center" }}>
            <ActivityIndicator color={theme.colors.textMuted} />
          </View>
        ) : null
      }
    />
  );
}
