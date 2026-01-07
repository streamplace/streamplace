import { Text, useStreamplaceStore, zero } from "@streamplace/components";
import AQLink from "components/aqlink";
import Container from "components/container";
import ErrorBox from "components/error/error";
import StreamCardHorizontal, { StreamCardSize } from "components/home/cards";
import LiveDot from "components/home/live-dot";
import Loading from "components/loading/loading";
import Title from "components/title";
import useAvatars from "hooks/useAvatars";
import { useEffect, useState } from "react";
import {
  Image,
  RefreshControl,
  ScrollView,
  View,
  useWindowDimensions,
} from "react-native";
import { PlaceStreamLivestream } from "streamplace";

// as we're not using a specific grid library these are necessary
// to constrain the cards
const FIRST_ROW_MAGIC_RATIO = 0.95;
const LAST_ROW_MAGIC_RATIO = 1.16;

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
  streams: PlaceStreamLivestream.LivestreamView[];
} {
  const mockSegments: PlaceStreamLivestream.LivestreamView[] = [];
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
      } as PlaceStreamLivestream.Record,
      author: {
        did: did,
        handle: handle,
      },
      indexedAt: new Date().toISOString(),
      viewerCount: { count: Math.floor(Math.random() * 1000) },
    });
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

function getHomeScreenCols(width: number): number {
  if (width >= 1550) return 4;
  if (width >= 1280) return 3;
  if (width >= 1024) return 2;
  if (width >= 768) return 2;
  return 1;
}

// Get the ratio for the first card based on column count
function getPadPercentage(cols: number): number {
  if (cols >= 3) return 2.07;
  return 1;
}

function HomeScreenItem({
  item,
  size,
  avatarUrl,
  horizontal = false,
}: {
  item: PlaceStreamLivestream.LivestreamView;
  size: StreamCardSize;
  avatarUrl?: string;
  horizontal?: boolean;
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
          (item.record as PlaceStreamLivestream.Record).title || "A livestream!"
        }
        horizontal={horizontal}
        thumbnailUrl={`/api/playback/${user}/stream.jpg?ts=${(Date.now() / 120000).toFixed(0)}`}
        avatarUrl={avatarUrl}
        streamerName={user}
        category={[]}
        viewers={item.viewerCount?.count}
        isLive={true}
      />
    </AQLink>
  );
}

function PlaceholderItem() {
  return (
    <View style={[{ flex: 1 }, { opacity: 0, pointerEvents: "none" }]}>
      <StreamCardHorizontal
        size={"sm"}
        title={"you found a secret :)"}
        horizontal={false}
        thumbnailUrl={``}
        avatarUrl={
          "https://cdn.bsky.app/img/avatar/plain/did:plc:4ukwiehjoytl56ysom2pdwko/bafkreieal2i74ynzrvofia6fa3efqnyxmox76ohrfldt5kvls73lbspzdm@jpeg"
        }
        streamerName={
          "hi! im here to pad out the grid so it doesn't look all wacky"
        }
        category={[]}
        viewers={0}
        isLive={false}
      />
    </View>
  );
}

export default function HomeScreen({
  contentContainerStyle = {},
}: {
  contentContainerStyle?: any;
}) {
  const liveUsers = useStreamplaceStore((state) => state.liveUsers);
  const setLiveUsers = useStreamplaceStore((state) => state.setLiveUsers);
  const refreshLiveUsers = () => setLiveUsers({ liveUsersRefresh: Date.now() });
  const liveUsersLoading = useStreamplaceStore(
    (state) => state.liveUsersLoading,
  );
  const liveUsersError = useStreamplaceStore((state) => state.liveUsersError);
  const [manualRefresh, setManualRefresh] = useState(false);
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
      return <Loading />;
    }
    if (!segments) {
      return <ErrorBox onRetry={refreshLiveUsers} />;
    }
  }

  if (segments === null) {
    // Only show loading if not using mock data and no segments yet
    return <Loading />;
  }

  let cols = getHomeScreenCols(width);
  let size = getHomeScreenItemSize(width);

  // Only use horizontal layout for first card when we have enough columns (3+)
  const useHorizontalFirst = cols >= 3;
  const firstRowCols = useHorizontalFirst ? cols - 1 : cols;

  const firstRowItems = segments.slice(0, firstRowCols);
  let cutSegs = segments.slice(firstRowCols);

  // fill in null data to pad out the list for grid display
  let segs: (PlaceStreamLivestream.LivestreamView | null)[] = cutSegs.concat(
    Array((cols - (segments.length % cols)) % cols).fill(null),
  );
  if (cutSegs.length === 0 && segs.every((s) => s === null) && cols > 0) {
    // ensure segs is not just [null] if segments is empty
    segs = [];
  }

  // assemble rows
  const rows: (PlaceStreamLivestream.LivestreamView | null)[][] = [];
  for (let i = 0; i < cutSegs.length; i += cols) {
    let row = cutSegs.slice(i, i + cols);
    // pad the last row with nulls if it's not full
    if (i + cols >= cutSegs.length && row.length < cols) {
      const paddingNeeded = cols - row.length;
      row = [...row, ...Array(paddingNeeded).fill(null)];
    }
    rows.push(row);
  }

  return (
    <>
      {liveUsersError && (
        <View>
          <Container
            style={{
              backgroundColor: "#774316",
              borderRadius: 8,
              borderColor: "#99889988",
              borderWidth: 2,
              height: "auto",
              flexDirection: "row",
              alignItems: "center",
              justifyContent: "flex-start",
              paddingHorizontal: 12,
              paddingVertical: 12,
              gap: 12,
            }}
          >
            <Text style={{ fontSize: 24, minWidth: 24, color: "white" }}>
              ⚠️
            </Text>
            <Text style={{ color: "white" }}>
              There was an error fetching the latest streams. You might be
              offline? code: {liveUsersError || "nocode"}
            </Text>
          </Container>
        </View>
      )}
      <ScrollView
        style={{
          minHeight: "80%",
          width: "100%",
        }}
        contentContainerStyle={contentContainerStyle} // Apply passed contentContainerStyle
        refreshControl={
          <RefreshControl
            refreshing={manualRefresh}
            onRefresh={() => {
              refreshLiveUsers();
              setManualRefresh(true);
            }}
          />
        }
      >
        <Container>
          {segments.length > 0 && (
            <View
              style={[
                { flexDirection: "row" },
                { alignItems: "center" },
                { gap: 12 },
                zero.my[8],
                zero.px[0],
              ]}
            >
              <LiveDot />
              <Title>
                {segments.length} {segments.length === 1 ? "person" : "people"}{" "}
                live now
              </Title>
            </View>
          )}

          {segments.length === 0 && (
            <View
              style={[
                { flex: 1 },
                { justifyContent: "center" },
                { alignItems: "center" },
                { minHeight: "auto", paddingVertical: 42 },
              ]}
            >
              <Image
                source={require("../../assets/images/jelly.png")}
                style={{ height: 64, width: 64 }}
              />
              <Text
                style={[{ fontSize: 20, fontWeight: "bold", marginTop: 12 }]}
              >
                No one is streaming right now
              </Text>
              <Text style={{ marginTop: 8 }}>Check back later?</Text>
            </View>
          )}
          {firstRowItems.length > 0 && (
            <View
              style={[
                { flexDirection: "row" },
                {
                  gap: 24,
                  marginBottom: 24,
                  width: "100%",
                },
              ]}
            >
              {firstRowItems.map((item, itemIndex) => (
                <View
                  key={item.cid || `item${itemIndex}`}
                  style={[
                    {
                      flex:
                        itemIndex == 0 && useHorizontalFirst
                          ? getPadPercentage(cols)
                          : 1,
                    },
                    { justifyContent: "center" },
                  ]}
                >
                  <HomeScreenItem
                    item={item}
                    size={size}
                    avatarUrl={avis[item.author.did]?.avatar}
                    horizontal={itemIndex == 0 && useHorizontalFirst}
                  />
                </View>
              ))}
              {/* Pad the first row to match the column count */}
              {Array(
                useHorizontalFirst
                  ? cols - firstRowItems.length - 1
                  : cols - firstRowItems.length,
              )
                .fill(null)
                .map((_, i) => (
                  <View key={`item-${i}`} style={{ flex: 1 }}>
                    <PlaceholderItem />
                  </View>
                ))}
            </View>
          )}

          {segments.length > 0 && (
            <View>
              {rows.map((row, rowIndex) => (
                <View
                  key={`row-${rowIndex}`}
                  style={[
                    { flexDirection: "row" },
                    { gap: 24, marginBottom: 24 },
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
                        />
                      </View>
                    ) : (
                      <View
                        key={`item-${rowIndex}-${itemIndex}`}
                        style={{ flex: 1 }}
                      >
                        <PlaceholderItem />
                      </View>
                    ),
                  )}
                </View>
              ))}
            </View>
          )}
        </Container>
      </ScrollView>
    </>
  );
}
