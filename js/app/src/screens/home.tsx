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
import { FlatList } from "react-native-gesture-handler";
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
    repoDID: "did:plc:mock1",
    signingKeyDID: "did:mock:1",
    startTime: new Date().toISOString(),
    repo: {
      handle: "mockuser1.net",
      did: "did:plc:mock1",
      pds: "bsky.network",
      rootCid: "invalid",
      version: "0.0",
    },
    viewers: 15,
  },
  {
    id: "mock-segment-2",
    repoDID: "did:plc:mock2",
    signingKeyDID: "did:mock:2",
    startTime: new Date().toISOString(),
    repo: {
      handle: "mockuser1.coffeeeeeeeeeeee",
      did: "did:plc:mock1",
      pds: "bsky.network",
      rootCid: "invalid",
      version: "0.0",
    },
    viewers: 30,
  },
  {
    id: "mock-segment-3",
    repoDID: "did:plc:mock3",
    signingKeyDID: "did:mock:3",
    startTime: new Date().toISOString(),
    repo: {
      handle: "mockuser1.hof",
      did: "did:plc:mock1",
      pds: "bsky.network",
      rootCid: "invalid",
      version: "0.0",
    },
    viewers: 8,
  },
  {
    id: "mock-segment-4",
    repoDID: "did:plc:mock3",
    signingKeyDID: "did:mock:3",
    startTime: new Date().toISOString(),
    repo: {
      handle: "mockuser1.ee",
      did: "did:plc:mock1",
      pds: "bsky.network",
      rootCid: "invalid",
      version: "0.0",
    },
    viewers: 8,
  },
  {
    id: "mock-segment-5",
    repoDID: "did:plc:mock3",
    signingKeyDID: "did:mock:3",
    startTime: new Date().toISOString(),
    repo: {
      handle: "mockuser1.wang",
      did: "did:plc:mock1",
      pds: "bsky.network",
      rootCid: "invalid",
      version: "0.0",
    },
    viewers: 8,
  },
  {
    id: "mock-segment-6",
    repoDID: "did:plc:mock3",
    signingKeyDID: "did:mock:3",
    startTime: new Date().toISOString(),
    repo: {
      handle: "mockuser1.site",
      did: "did:plc:mock1",
      pds: "bsky.network",
      rootCid: "invalid",
      version: "0.0",
    },
    viewers: 8,
  },
  {
    id: "mock-segment-7",
    repoDID: "did:plc:mock3",
    signingKeyDID: "did:mock:3",
    startTime: new Date().toISOString(),
    repo: {
      handle: "mockuser1.nl",
      did: "did:plc:mock1",
      pds: "bsky.network",
      rootCid: "invalid",
      version: "0.0",
    },
    viewers: 8,
  },
  {
    id: "mock-segment-8",
    repoDID: "did:plc:mock3",
    signingKeyDID: "did:mock:3",
    startTime: new Date().toISOString(),
    repo: {
      handle: "mockuser1.jp",
      did: "did:plc:mock1",
      pds: "bsky.network",
      rootCid: "invalid",
      version: "0.0",
    },
    viewers: 8,
  },
  {
    id: "mock-segment-9",
    repoDID: "did:plc:mock3",
    signingKeyDID: "did:mock:3",
    startTime: new Date().toISOString(),
    repo: {
      handle: "mockuser1.berlin",
      did: "did:plc:mock1",
      pds: "bsky.network",
      rootCid: "invalid",
      version: "0.0",
    },
    viewers: 8,
  },
  {
    id: "mock-segment-10",
    repoDID: "did:plc:mock3",
    signingKeyDID: "did:mock:3",
    startTime: new Date().toISOString(),
    repo: {
      handle: "mockuser1.kyoto",
      did: "did:plc:mock1",
      pds: "bsky.network",
      rootCid: "invalid",
      version: "0.0",
    },
    viewers: 8,
  },
];

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
    return 3;
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
    return 10;
  } else if (media.xxl) {
    return 11.5;
  } else {
    return 10;
  }
}

function HomeScreenItem({
  item,
  media,
  size,
}: {
  item: Segment;
  media: UseMediaState;
  size: StreamCardSize;
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
    >
      <StreamCardHorizontal
        size={size}
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
  }, [useMockData]);

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

  if (firstRequest && !useMockData) {
    // Only show loading if not using mock data
    return <Loading />;
  }

  let cols = getHomeScreenCols(media);
  let size = getHomeScreenItemSize(media);

  // fill in null data to pad out the list
  let segs: (Segment | null)[] = segments.concat(
    Array((cols - (segments.length % cols)) % cols).fill(null),
  );

  console.log(segs);

  return (
    <ScrollView
      contentContainerStyle={{
        alignItems: "stretch",
        minHeight: "100%",
        ...contentContainerStyle,
      }}
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
          <View flexDirection="row" alignItems="center" gap="$3">
            <LiveDot />
            <Title marginVertical={16}>
              {segments.length} {segments.length === 1 ? "person" : "people"}{" "}
              live now
            </Title>
          </View>
        )}
        <FlatList
          key={cols}
          ListEmptyComponent={() => (
            <View
              f={1}
              justifyContent="center"
              alignItems="center"
              minHeight="80vh"
            >
              <H6>No one is streaming right now 😭</H6>
            </View>
          )}
          renderItem={(i) =>
            i.item !== null ? (
              <View f={1} width={getPadPercentage(media) + "%"}>
                <HomeScreenItem
                  key={i.index}
                  item={i.item}
                  media={media}
                  size={size}
                />
              </View>
            ) : (
              <>
                {/* HACK */}
                <View f={1} width={getPadPercentage(media) + "%"} />
              </>
            )
          }
          numColumns={cols}
          data={segs}
          style={{
            minHeight: "80%",
            maxWidth: "100%",
          }}
          contentContainerStyle={{
            gap: 24,
          }}
          columnWrapperStyle={cols > 1 && { gap: 24 }}
        />
      </Container>
    </ScrollView>
  );
}
