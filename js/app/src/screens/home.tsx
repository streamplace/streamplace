import {
  ACTIVITY_LABEL_DISPLAY,
  Skeleton,
  Text,
  useStreamplaceStore,
  useTheme,
  zero,
} from "@streamplace/components";
import { spacing } from "@streamplace/components/src/lib/theme/tokens";
import AQLink from "components/aqlink";
import Container from "components/container";
import { EmptyState } from "components/empty-state";
import ErrorBox from "components/error/error";
import StreamCardHorizontal, { StreamCardSize } from "components/home/cards";
import LiveDot from "components/home/live-dot";
import PullToRefreshScrollView from "components/pull-to-refresh";
import { Image } from "expo-image";
import useAvatars from "hooks/useAvatars";
import { useEffect, useState } from "react";
import { Platform, View, useWindowDimensions } from "react-native";
import { useSafeAreaInsets } from "react-native-safe-area-context";
import { place } from "streamplace";

function getStreamActivity(
  record: place.stream.livestream.Main,
): string | undefined {
  if (!record.activity) return undefined;
  if (record.activity.$type === "place.stream.defs#activityGame") {
    const game = record.activity as place.stream.defs.ActivityGame;
    return game.name ?? undefined;
  }
  if (record.activity.$type === "place.stream.defs#activityLabel") {
    const label = record.activity as place.stream.defs.ActivityLabel;
    return ACTIVITY_LABEL_DISPLAY[label.label] ?? label.label;
  }
  return undefined;
}

type StreamRecord = {
  createdAt: Date;
  title?: string;
  // A post announcing the stream record
  post?: {
    cid: string;
    uri: string;
  };
  // The base URL of the streamed server
  url: string;
};

// Function to generate mock data for testing purposes
function generateMockSegments(count: number): {
  streams: place.stream.livestream.LivestreamView[];
} {
  const mockSegments: place.stream.livestream.LivestreamView[] = [];
  const baseDid = "did:plc:mockmockmockmockmockmockmockmockmock";

  for (let i = 0; i < count; i++) {
    const did = `${baseDid}${i}`;
    const handle = `mockuser${i}`;
    mockSegments.push({
      uri: `at://did:plc:mockmockmockmockmockmockmockmockmock${i}/place.stream.livestream/mock${i}`,
      cid: `bafycidmockcidmockcidmockcidmockcidmockcidmockcidm${i}`,
      record: {
        $type: "place.stream.livestream",
        createdAt: new Date().toISOString(),
        title: `Mock Stream ${i + 1}`,
      } as place.stream.livestream.Main,
      author: {
        did: did,
        handle: handle,
      },
      indexedAt: new Date().toISOString(),
      viewerCount: { count: Math.floor(Math.random() * 1000) },
    } as any);
  }
  return { streams: mockSegments };
}

function getHomeScreenItemSize(width: number): StreamCardSize {
  if (width >= 1536) return "md"; // xxl
  if (width >= 1280) return "sm"; // xl
  if (width >= 1024) return "sm"; // lg
  if (width >= 768) return "sm"; // md
  return "xs"; // sm and below
}

// Uniform responsive grid, YouTube-style. Cards stay large — YouTube
// shows 3 columns on a laptop, 4 on a big display.
function getHomeScreenCols(width: number): number {
  if (width >= 1900) return 4;
  if (width >= 1400) return 3;
  if (width >= 900) return 2;
  return 1;
}

function HomeScreenItem({
  item,
  size,
  avatarUrl,
  horizontal = false,
  showAvatar = true,
}: {
  item: place.stream.livestream.LivestreamView;
  size: StreamCardSize;
  avatarUrl?: string;
  horizontal?: boolean;
  showAvatar?: boolean;
}) {
  const user = item.author.handle || item.author.did;
  return (
    <AQLink
      to={{
        screen: "Stream",
        params: {
          user: user,
        },
      }}
      style={{
        flex: 1,
      }}
    >
      <StreamCardHorizontal
        size={size}
        title={
          (item.record as place.stream.livestream.Main).title || "A livestream!"
        }
        horizontal={horizontal}
        showAvatar={showAvatar}
        thumbnailUrl={`/api/playback/${user}/stream.jpg?ts=${(Date.now() / 120000).toFixed(0)}`}
        avatarUrl={avatarUrl}
        streamerName={user}
        category={[]}
        activity={getStreamActivity(
          item.record as place.stream.livestream.Main,
        )}
        tags={(item.record as place.stream.livestream.Main).tags ?? []}
        viewers={item.viewerCount?.count}
        isLive={true}
      />
    </AQLink>
  );
}

function StreamCardSkeleton() {
  return (
    <View style={{ flex: 1, gap: spacing[3] }}>
      <Skeleton
        radius="lg"
        height={undefined}
        style={{ aspectRatio: 16 / 9 }}
      />
      <View style={{ flexDirection: "row", gap: spacing[3] }}>
        <Skeleton shape="circle" width={40} />
        <View style={{ flex: 1, gap: spacing[2] }}>
          <Skeleton shape="text" width="85%" />
          <Skeleton shape="text" width="45%" />
        </View>
      </View>
    </View>
  );
}

// Full-page skeleton grid matching the real column layout so nothing
// shifts when data arrives.
function HomeSkeletonGrid() {
  const { width } = useWindowDimensions();
  const cols = getHomeScreenCols(width);
  const rows = 3;
  return (
    <Container>
      <View
        style={{
          flexDirection: "row",
          alignItems: "center",
          gap: spacing[3],
          marginVertical: spacing[8],
        }}
      >
        <Skeleton shape="circle" width={12} />
        <Skeleton shape="text" width={180} height={20} />
      </View>
      {Array(rows)
        .fill(null)
        .map((_, r) => (
          <View
            key={r}
            style={{
              flexDirection: "row",
              gap: spacing[6],
              marginBottom: spacing[6],
            }}
          >
            {Array(cols)
              .fill(null)
              .map((_, c) => (
                <StreamCardSkeleton key={c} />
              ))}
          </View>
        ))}
    </Container>
  );
}

export default function HomeScreen({
  contentContainerStyle = {},
}: {
  contentContainerStyle?: any;
}) {
  const safeAreaInsets = useSafeAreaInsets();
  const liveUsers = useStreamplaceStore((state) => state.liveUsers);
  const setLiveUsers = useStreamplaceStore((state) => state.setLiveUsers);
  const refreshLiveUsers = () => setLiveUsers({ liveUsersRefresh: Date.now() });
  const liveUsersLoading = useStreamplaceStore(
    (state) => state.liveUsersLoading,
  );
  const liveUsersError = useStreamplaceStore((state) => state.liveUsersError);
  const [manualRefresh, setManualRefresh] = useState(false);
  const { theme } = useTheme();
  const { width } = useWindowDimensions();

  // Use mock data for development/testing if needed
  //const segments = generateMockSegments(1).streams; // Uncomment this line to use mock data
  const segments = useStreamplaceStore((state) => state.liveUsers);
  // const segments = realSegments; // Comment this line out if using mock data

  const avis = useAvatars((segments || []).map((s) => s.author.did));

  useEffect(() => {
    if (!liveUsersLoading) {
      setManualRefresh(false);
    }
  }, [liveUsersLoading]);

  if (liveUsersError) {
    if (liveUsersLoading) {
      return <HomeSkeletonGrid />;
    }
    if (!segments) {
      return <ErrorBox onRetry={refreshLiveUsers} />;
    }
  }

  if (segments === null) {
    // Only show loading if not using mock data and no segments yet
    return <HomeSkeletonGrid />;
  }

  let cols = getHomeScreenCols(width);
  let size = getHomeScreenItemSize(width);

  // Use horizontal (SBS) layout for all items on single-column breakpoint
  const useHorizontalAll = cols === 1;

  let rows: (place.stream.livestream.LivestreamView | null)[][] = [];

  if (!useHorizontalAll) {
    for (let i = 0; i < segments.length; i += cols) {
      let row = segments.slice(i, i + cols);
      if (row.length < cols) {
        row = [...row, ...Array(cols - row.length).fill(null)];
      }
      rows.push(row);
    }
  }

  const indicatorTop = safeAreaInsets.top;

  return (
    <>
      {liveUsersError && (
        <View>
          <Container
            style={{
              backgroundColor: theme.colors.surface2,
              borderRadius: zero.borderRadius.md,
              borderColor: theme.colors.warning,
              borderWidth: 1,
              height: "auto",
              flexDirection: "row",
              alignItems: "center",
              justifyContent: "flex-start",
              paddingHorizontal: spacing[3],
              paddingVertical: spacing[3],
              gap: spacing[3],
            }}
          >
            <Text size="xl" style={{ minWidth: 24 }}>
              ⚠️
            </Text>
            <Text size="sm">
              There was an error fetching the latest streams. You might be
              offline? code: {liveUsersError || "nocode"}
            </Text>
          </Container>
        </View>
      )}
      <PullToRefreshScrollView
        style={[
          {
            minHeight: "100%",
            width: "100%",
          },
          Platform.OS === "ios" ? zero.pt[24] : zero.pt[0],
        ]}
        contentContainerStyle={[
          // When empty, fill the viewport so the Container centers the empty
          // state vertically (matching Videos / My Videos). Populated feeds
          // keep their natural top-aligned scroll.
          segments.length === 0 && { flexGrow: 1 },
          contentContainerStyle,
        ]}
        refreshing={manualRefresh}
        onRefresh={() => {
          refreshLiveUsers();
          setManualRefresh(true);
        }}
        indicatorTop={indicatorTop}
      >
        <Container>
          {segments.length > 0 && (
            <View
              style={[
                { flexDirection: "row" },
                { alignItems: "center" },
                { gap: spacing[3] },
                { marginTop: spacing[8], marginBottom: spacing[6] },
              ]}
            >
              <LiveDot />
              <Text size="xl" weight="semibold">
                Live now
              </Text>
              <Text size="sm" tabular style={{ color: theme.colors.text3 }}>
                {segments.length}{" "}
                {segments.length === 1 ? "streamer" : "streamers"}
              </Text>
            </View>
          )}

          {segments.length === 0 && (
            <EmptyState
              illustration={
                <Image
                  source={require("../../assets/images/jelly.png")}
                  style={{ height: 64, width: 64 }}
                />
              }
              title="No one is streaming right now"
              subtitle="Check back later?"
            />
          )}
          {useHorizontalAll
            ? segments.map((item) => (
                <View
                  key={item.cid}
                  style={{ width: "100%", marginBottom: spacing[4] }}
                >
                  <HomeScreenItem
                    item={item}
                    size={size}
                    avatarUrl={avis[item.author.did]?.avatar}
                    horizontal={true}
                    showAvatar={false}
                  />
                </View>
              ))
            : null}

          {!useHorizontalAll && segments.length > 0 && (
            <View>
              {rows.map((row, rowIndex) => (
                <View
                  key={`row-${rowIndex}`}
                  style={[
                    { flexDirection: "row" },
                    { gap: spacing[4], marginBottom: spacing[8] },
                  ]}
                >
                  {row.map((item, itemIndex) =>
                    item !== null ? (
                      <View
                        key={item.cid || `item-${rowIndex}-${itemIndex}`}
                        style={{ flex: 1 }}
                      >
                        <HomeScreenItem
                          item={item}
                          size={size}
                          avatarUrl={avis[item.author.did]?.avatar}
                          horizontal={false}
                          showAvatar={true}
                        />
                      </View>
                    ) : (
                      <View
                        key={`spacer-${rowIndex}-${itemIndex}`}
                        style={{ flex: 1 }}
                      />
                    ),
                  )}
                </View>
              ))}
            </View>
          )}
        </Container>
        <View
          style={{
            height:
              Platform.OS !== "web"
                ? 64 + safeAreaInsets.bottom
                : safeAreaInsets.bottom,
          }}
        />
      </PullToRefreshScrollView>
    </>
  );
}
