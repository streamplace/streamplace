import { UseMediaState } from "@tamagui/web";
import AQLink from "components/aqlink";
import ErrorBox from "components/error/error";
import Loading from "components/loading/loading";
import StreamCardHorizontal, {
  StreamCardSize,
} from "components/ui/cards/horizontal";
import Container from "components/ui/container";
import LiveDot from "components/ui/live-dot";
import Title from "components/ui/title";
import {
  pollSegments,
  Repo,
  selectRecentSegments,
} from "features/streamplace/streamplaceSlice";
import useStreamplaceNode from "hooks/useStreamplaceNode";
import { useEffect, useState } from "react";
import { RefreshControl } from "react-native";
import { useAppDispatch, useAppSelector } from "store/hooks";
import { H6, ScrollView, ScrollViewProps, useMedia, View } from "tamagui";

type Segment = {
  id: string;
  repoDID: string;
  signingKeyDID: string;
  startTime: string;
  repo: Repo;
  viewers: number;
};

// Mock data for segments
const mockSegments: Segment[] = [
  {
    id: "mock-segment-1",
    repoDID: "did:plc:lghfd7elj6cjjwlhecp2utao",
    signingKeyDID: "did:mock:1",
    startTime: new Date().toISOString(),
    repo: {
      handle: "soapy.social",
      did: "did:plc:lghfd7elj6cjjwlhecp2utao",
      pds: "bsky.network",
      rootCid: "invalid",
      version: "0.0",
    },
    viewers: 15,
  },
  {
    id: "mock-segment-2",
    repoDID: "did:plc:oio4hkxaop4ao4wz2pp3f4cr",
    signingKeyDID: "did:mock:2",
    startTime: new Date().toISOString(),
    repo: {
      handle: "mackuba.eu",
      did: "did:plc:oio4hkxaop4ao4wz2pp3f4cr",
      pds: "bsky.network",
      rootCid: "invalid",
      version: "0.0",
    },
    viewers: 30,
  },
  {
    id: "mock-segment-3",
    repoDID: "did:plc:p2cp5gopk7mgjegy6wadk3ep",
    signingKeyDID: "did:mock:3",
    startTime: new Date().toISOString(),
    repo: {
      handle: "samuel.bsky.team",
      did: "did:plc:p2cp5gopk7mgjegy6wadk3ep",
      pds: "bsky.network",
      rootCid: "invalid",
      version: "0.0",
    },
    viewers: 8,
  },
  {
    id: "mock-segment-4",
    repoDID: "did:plc:vc7f4oafdgxsihk4cry2xpze",
    signingKeyDID: "did:mock:3",
    startTime: new Date().toISOString(),
    repo: {
      handle: "jer.ry",
      did: "did:plc:vc7f4oafdgxsihk4cry2xpze",
      pds: "bsky.network",
      rootCid: "invalid",
      version: "0.0",
    },
    viewers: 8,
  },
  {
    id: "mock-segment-5",
    repoDID: "did:plc:by3jhwdqgbtrcc7q4tkkv3cf",
    signingKeyDID: "did:mock:3",
    startTime: new Date().toISOString(),
    repo: {
      handle: "alice.mosphere.at",
      did: "did:plc:by3jhwdqgbtrcc7q4tkkv3cf",
      pds: "bsky.network",
      rootCid: "invalid",
      version: "0.0",
    },
    viewers: 8,
  },
  {
    id: "mock-segment-6",
    repoDID: "did:plc:vlblikmsgpx2i7fvxinrsvzu",
    signingKeyDID: "did:mock:3",
    startTime: new Date().toISOString(),
    repo: {
      handle: "moll.dev",
      did: "did:plc:vlblikmsgpx2i7fvxinrsvzu",
      pds: "bsky.network",
      rootCid: "invalid",
      version: "0.0",
    },
    viewers: 8,
  },
  {
    id: "mock-segment-7",
    repoDID: "did:plc:xwhsmuozq3mlsp56dyd7copv",
    signingKeyDID: "did:mock:3",
    startTime: new Date().toISOString(),
    repo: {
      handle: "paizuri.moe",
      did: "did:plc:xwhsmuozq3mlsp56dyd7copv",
      pds: "bsky.network",
      rootCid: "invalid",
      version: "0.0",
    },
    viewers: 8,
  },
  {
    id: "mock-segment-8",
    repoDID: "did:plc:gq4fo3u6tqzzdkjlwzpb23tj",
    signingKeyDID: "did:mock:3",
    startTime: new Date().toISOString(),
    repo: {
      handle: "da.me",
      did: "did:plc:gq4fo3u6tqzzdkjlwzpb23tj",
      pds: "bsky.network",
      rootCid: "invalid",
      version: "0.0",
    },
    viewers: 8,
  },
  {
    id: "mock-segment-9",
    repoDID: "did:plc:rbvrr34edl5ddpuwcubjiost",
    signingKeyDID: "did:mock:3",
    startTime: new Date().toISOString(),
    repo: {
      handle: "stream.place",
      did: "did:plc:rbvrr34edl5ddpuwcubjiost",
      pds: "bsky.network",
      rootCid: "invalid",
      version: "0.0",
    },
    viewers: 8,
  },
  {
    id: "mock-segment-10",
    repoDID: "did:plc:gotnvwkr56ibs33l4hwgfoet",
    signingKeyDID: "did:mock:3",
    startTime: new Date().toISOString(),
    repo: {
      handle: "zeu.dev",
      did: "did:plc:gotnvwkr56ibs33l4hwgfoet",
      pds: "bsky.network",
      rootCid: "invalid",
      version: "0.0",
    },
    viewers: 8,
  },
  {
    id: "mock-segment-11",
    repoDID: "did:plc:tpg43qhh4lw4ksiffs4nbda3",
    signingKeyDID: "did:mock:3",
    startTime: new Date().toISOString(),
    repo: {
      handle: "jacob.gold",
      did: "did:plc:tpg43qhh4lw4ksiffs4nbda3",
      pds: "bsky.network",
      rootCid: "invalid",
      version: "0.0",
    },
    viewers: 8,
  },
];

const MAGIC_DIVIDE_BY_BOTTOM_ROW = 1.85;

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
  if (media.gtXl) {
    return 4;
  } else if (media.gtLg) {
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

// HACK to provide ratio for correct-looking padding for grid
// TODO: use an actual grid lib for RN?
function getPadPercentage(media: UseMediaState): number {
  if (media.gtXxl) {
    return 2.27;
  } else if (media.xxl) {
    return 2.4;
  } else {
    return 2.4;
  }
}

function HomeScreenItem({
  item,
  media,
  size,
  horizontal = false,
}: {
  item: Segment;
  media: UseMediaState;
  size: StreamCardSize;
  horizontal?: boolean;
}) {
  const user = item.repo?.handle || item.repoDID || item.signingKeyDID;
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
        horizontal={horizontal}
        thumbnailUrl={
          item.signingKeyDID.startsWith("did:mock")
            ? "https://picsum.photos/1600/900?rand=" + item.id
            : `https://stream.place/api/playback/${user}/stream.png`
        }
        avatarUrl="https://cdn.bsky.app/img/avatar/plain/did:plc:4ukwiehjoytl56ysom2pdwko/bafkreieal2i74ynzrvofia6fa3efqnyxmox76ohrfldt5kvls73lbspzdm@jpeg"
        streamerName={user}
        category={[]}
        viewers={item.viewers}
        isLive={true}
      />
    </AQLink>
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
  const [useMockData, setUseMockData] = useState(true); // Set to true to use mock data

  const segments = useMockData ? mockSegments : realSegments;
  const media = useMedia();

  useEffect(() => {
    if (!useMockData) {
      // Only poll if not using mock data
      dispatch(pollSegments());
    }
    // get array of
  }, [useMockData, dispatch]);

  useEffect(() => {
    if (!loading) {
      setManualRefresh(false);
    }
  }, [loading]);

  if (error && !useMockData) {
    // Only show error if not using mock data
    if (loading) {
      return <Loading />;
    }
    return (
      <ErrorBox
        onRetry={() => {
          dispatch(pollSegments());
        }}
      />
    );
  }

  if (firstRequest && !useMockData && !segments.length) {
    // Only show loading if not using mock data and no segments yet
    return <Loading />;
  }

  let cols = getHomeScreenCols(media);
  let size = getHomeScreenItemSize(media);

  const firstRowCols = cols > 2 ? cols - 1 : 0;

  const firstRowItems = segments.slice(0, firstRowCols);
  let cutSegs = segments.slice(firstRowCols, -1);

  // fill in null data to pad out the list for grid display
  let segs: (Segment | null)[] = cutSegs.concat(
    Array((cols - (segments.length % cols)) % cols).fill(null),
  );
  if (cutSegs.length === 0 && segs.every((s) => s === null) && cols > 0) {
    // ensure segs is not just [null] if segments is empty
    segs = [];
  }

  // Create rows for the grid
  const rows: (Segment | null)[][] = [];
  for (let i = 0; i < segs.length; i += cols) {
    rows.push(segs.slice(i, i + cols));
  }

  return (
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
            if (!useMockData) {
              // Only refresh if not using mock data
              dispatch(pollSegments());
              setManualRefresh(true);
            } else {
              // Optionally update mock data here if needed for refresh
              setManualRefresh(false);
            }
          }}
        />
      }
    >
      <Container>
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
            minHeight="80vh"
            paddingHorizontal={0}
          >
            <H6>No one is streaming right now 😭</H6>
          </View>
        )}
        {firstRowItems.length > 0 && (
          <View
            flexDirection="row"
            gap={24} // This is the gap between columns
            marginBottom={24} // This is the gap between rows
            width="full"
          >
            {firstRowItems.map((item, itemIndex) => (
              <View
                key={item.id || `item${itemIndex}`}
                flex={
                  itemIndex == 0 ? getPadPercentage(media) / cols : 1 / cols
                }
                justifyContent="center"
              >
                <HomeScreenItem
                  item={item}
                  media={media}
                  size={size}
                  horizontal={itemIndex == 0}
                />
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
                      key={item.id || `item-${rowIndex}-${itemIndex}`}
                      flex={1}
                    >
                      <HomeScreenItem
                        item={item}
                        media={media}
                        size={size}
                        //horizontal={row[row.length - 1] == null}
                      />
                    </View>
                  ) : (
                    <View
                      key={`item-${rowIndex}-${itemIndex}`}
                      flex={
                        getPadPercentage(media) / MAGIC_DIVIDE_BY_BOTTOM_ROW
                      }
                    ></View>
                  ),
                )}
              </View>
            ))}
          </View>
        )}
      </Container>
    </ScrollView>
  );
}
