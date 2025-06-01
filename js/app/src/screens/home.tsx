import { AlertCircle } from "@tamagui/lucide-icons";
import { UseMediaState } from "@tamagui/web";
import AQLink from "components/aqlink";
import Container from "components/container";
import ErrorBox from "components/error/error";
import StreamCardHorizontal, { StreamCardSize } from "components/home/cards";
import LiveDot from "components/home/live-dot";
import Loading from "components/loading/loading";
import Title from "components/title";
import {
  pollSegments,
  selectRecentSegments,
} from "features/streamplace/streamplaceSlice";
import useAvatars from "hooks/useAvatars";
import useStreamplaceNode from "hooks/useStreamplaceNode";
import { useEffect, useState } from "react";
import { RefreshControl } from "react-native";
import { useAppDispatch, useAppSelector } from "store/hooks";
import { PlaceStreamLivestream } from "streamplace";
import {
  H3,
  Image,
  Paragraph,
  ScrollView,
  ScrollViewProps,
  Text,
  useMedia,
  View,
} from "tamagui";

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

function getHomeScreenItemSize(media: UseMediaState): StreamCardSize {
  if (media.gtXxl) {
    return "md";
  } else if (media.gtLg) {
    return "sm";
  } else if (media.md) {
    return "sm";
  } else {
    return "xs";
  }
}

function getHomeScreenCols(media: UseMediaState): number {
  if (media.gtXxl) {
    return 4;
  } else if (media.gtXl) {
    return 3;
  } else if (media.gtMd) {
    return 2;
  } else if (media.gtSm) {
    return 2;
  } else if (media.gtXs) {
    return 2;
  } else {
    return 1;
  }
}
// Get the ratio for the first icon padding
function getPadPercentage(media: UseMediaState): number {
  if (media.gtXl) {
    return 2.28;
  } else {
    return 2.3;
  }
}

function HomeScreenItem({
  item,
  media,
  size,
  avatarUrl,
  horizontal = false,
}: {
  item: PlaceStreamLivestream.LivestreamView;
  media: UseMediaState;
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
        thumbnailUrl={`/api/playback/${user}/stream.png?bweh=${(Date.now() / 120000).toFixed(0)}`}
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
    <View flex={1} opacity={0} pointerEvents="none">
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
  contentContainerStyle?: Exclude<
    ScrollViewProps["contentContainerStyle"],
    string
  >;
}) {
  const { url } = useStreamplaceNode();
  const {
    segments: realSegments,
    error,
    loading,
    firstRequest,
  } = useAppSelector(selectRecentSegments);
  const dispatch = useAppDispatch();
  const [manualRefresh, setManualRefresh] = useState(false);

  // Use mock data for development/testing if needed
  //const segments = generateMockSegments(1).streams; // Uncomment this line to use mock data
  const segments = realSegments; // Comment this line out if using mock data
  const media = useMedia();

  const avis = useAvatars(segments.map((s) => s.author.did));

  useEffect(() => {
    dispatch(pollSegments());
    // get array of
  }, [dispatch]);

  useEffect(() => {
    if (!loading) {
      setManualRefresh(false);
    }
  }, [loading]);

  if (error) {
    if (loading) {
      return <Loading />;
    }
    if (!segments) {
      return (
        <ErrorBox
          onRetry={() => {
            dispatch(pollSegments());
          }}
        />
      );
    }
  }

  if (firstRequest && !segments.length) {
    // Only show loading if not using mock data and no segments yet
    return <Loading />;
  }

  let cols = getHomeScreenCols(media);
  let size = getHomeScreenItemSize(media);

  const firstRowCols = cols > 2 ? cols - 1 : cols;

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
      {error && (
        <View>
          <Container
            backgroundColor="#774316"
            borderRadius="$4"
            borderColor="#99889988"
            borderWidth={2}
            height="unset"
            flexDirection="row"
            alignItems="center"
            justifyContent="flex-start"
            paddingHorizontal="$3"
            paddingVertical="$3"
            gap="$3"
          >
            <AlertCircle size={24} minWidth={24} color="$white" />
            <Text>
              There was an error fetching the latest streams. You might be
              offline? code: {error || "nocode"}
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
              dispatch(pollSegments());
              setManualRefresh(true);
            }}
          />
        }
      >
        <Container width="100%">
          {segments.length > 0 && (
            <View
              flexDirection="row"
              alignItems="center"
              gap="$3"
              marginVertical="$4"
              paddingHorizontal="$0"
            >
              <LiveDot />
              <Title>
                {segments.length} {segments.length === 1 ? "person" : "people"}{" "}
                live now
              </Title>
            </View>
          )}

          {segments.length === 0 && !loading && (
            <View
              f={1}
              justifyContent="center"
              alignItems="center"
              minHeight="auto"
              paddingVertical={42}
            >
              <Image
                source={require("../../assets/images/jelly.png")}
                height="$9"
                width="$9"
              />
              <H3>No one is streaming right now</H3>
              <Paragraph>Check back later?</Paragraph>
            </View>
          )}
          {firstRowItems.length > 0 && (
            <View flexDirection="row" gap={24} marginBottom={24} width="full">
              {firstRowItems.map((item, itemIndex) => (
                <View
                  key={item.cid || `item${itemIndex}`}
                  flex={
                    itemIndex == 0 && cols > 2 ? getPadPercentage(media) : 1
                  }
                  justifyContent="center"
                >
                  <HomeScreenItem
                    item={item}
                    media={media}
                    size={size}
                    avatarUrl={avis[item.author.did]?.avatar}
                    horizontal={itemIndex == 0 && cols > 2}
                  />
                </View>
              ))}
              {/* if cols > 2 (first el is horizontal) then pad the rest, else pad to 2 */}
              {Array(
                cols > 2
                  ? cols - firstRowItems.length - 1
                  : cols - firstRowItems.length,
              )
                .fill(null)
                .map((i) => (
                  <View key={`item-${i}`} flex={1}>
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
                  flexDirection="row"
                  gap={24} // This is the gap between columns
                  marginBottom={24} // This is the gap between rows
                >
                  {row.map((item, itemIndex) =>
                    item !== null ? (
                      <View
                        key={item.cid || `item-${rowIndex}-${itemIndex}`}
                        flex={1}
                      >
                        <HomeScreenItem
                          item={item}
                          media={media}
                          size={size}
                          avatarUrl={avis[item.author.did]?.avatar}
                        />
                      </View>
                    ) : (
                      <View key={`item-${rowIndex}-${itemIndex}`} flex={1}>
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
