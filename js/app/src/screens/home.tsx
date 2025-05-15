import { UseMediaState } from "@tamagui/web";
import AQLink from "components/aqlink";
import ErrorBox from "components/error/error";
import Loading from "components/loading/loading";
import StreamCardHorizontal, { StreamCardSize } from "components/ui/cards";
import Container from "components/ui/container";
import LiveDot from "components/ui/live-dot";
import Title from "components/ui/title";
import {
  pollSegments,
  Repo,
  selectRecentSegments,
} from "features/streamplace/streamplaceSlice";
import useAvatars from "hooks/useAvatars";
import useStreamplaceNode from "hooks/useStreamplaceNode";
import { useEffect, useState } from "react";
import { RefreshControl } from "react-native";
import { useAppDispatch, useAppSelector } from "store/hooks";
import {
  H6,
  ScrollView,
  ScrollViewProps,
  useMedia,
  View,
  Image,
  Paragraph,
  H4,
  H3,
} from "tamagui";

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

type Segment = {
  id: string;
  repoDID: string;
  signingKeyDID: string;
  startTime: string;
  title?: string;
  repo: Repo;
  viewers: number;
  streamRecord?: StreamRecord;
};

const MAGIC_DIVIDE_BY_BOTTOM_ROW = 5;

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

// HACK to provide ratio for correct-looking padding for grid
// TODO: use an actual grid lib for RN?
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
  item: Segment;
  media: UseMediaState;
  size: StreamCardSize;
  avatarUrl?: string;
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
        title={item.streamRecord?.title || "A livestream!"}
        horizontal={horizontal}
        thumbnailUrl={
          item.signingKeyDID.startsWith("did:mock")
            ? "https://picsum.photos/1600/900?rand=" + item.id
            : `https://stream.place/api/playback/${user}/stream.png`
        }
        avatarUrl={
          avatarUrl ||
          "https://cdn.bsky.app/img/avatar/plain/did:plc:4ukwiehjoytl56ysom2pdwko/bafkreieal2i74ynzrvofia6fa3efqnyxmox76ohrfldt5kvls73lbspzdm@jpeg"
        }
        streamerName={user}
        category={[]}
        viewers={item.viewers}
        isLive={true}
      />
    </AQLink>
  );
}

const fakeSegs = generateSegments(6);

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
  const [useMockData, setUseMockData] = useState(true);

  const segments = useMockData ? fakeSegs : realSegments;
  const media = useMedia();

  const avis = useAvatars(segments.map((s) => s.repoDID));

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

  const firstRowCols = cols > 2 ? cols - 1 : cols;

  const firstRowItems = segments.slice(0, firstRowCols);
  let cutSegs = segments.slice(firstRowCols);

  // fill in null data to pad out the list for grid display
  let segs: (Segment | null)[] = cutSegs.concat(
    Array((cols - (segments.length % cols)) % cols).fill(null),
  );
  if (cutSegs.length === 0 && segs.every((s) => s === null) && cols > 0) {
    // ensure segs is not just [null] if segments is empty
    segs = [];
  }

  // assemble rows
  const rows: (Segment | null)[][] = [];
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
              dispatch(pollSegments());
              setManualRefresh(true);
            } else {
              setManualRefresh(false);
            }
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
            minHeight="90%"
            paddingVertical={42}
          >
            <Image
              source={{ uri: require("assets/images/jelly.png") }}
              width={80}
              height={80}
            />
            <H3>No one is streaming right now</H3>
            <Paragraph>Check back later?</Paragraph>
          </View>
        )}
        {firstRowItems.length > 0 && (
          <View flexDirection="row" gap={24} marginBottom={24} width="full">
            {firstRowItems.map((item, itemIndex) => (
              <View
                key={item.id || `item${itemIndex}`}
                flex={
                  itemIndex == 0
                    ? cols > 2
                      ? firstRowItems.length < 2
                        ? 0.65
                        : getPadPercentage(media) * cols
                      : cols
                    : cols
                }
                justifyContent="center"
              >
                <HomeScreenItem
                  item={item}
                  media={media}
                  size={size}
                  avatarUrl={avis[item.repoDID]?.avatar}
                  horizontal={itemIndex == 0 && cols > 2}
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
                        avatarUrl={avis[item.repoDID]?.avatar}
                      />
                    </View>
                  ) : (
                    <View
                      key={`item-${rowIndex}-${itemIndex}`}
                      flex={cols ** 1.16 / cols}
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

function generateSegments(num: number = 32): Segment[] {
  if (num < 1) return [];
  const segments: Segment[] = Array.from({ length: num }, () =>
    generateSegment(),
  );

  return segments;
}
function generateSegment(overrides: Partial<Segment> = {}): Segment {
  const now = new Date();
  const blueskyDIDs = [
    "did:plc:5mu44cojafmxj6h3yaihy2nl",
    "did:plc:sibej6afldtetfanqhganjwg",
    "did:plc:b5ly66nko7iijwy2lktt3ctq",
    "did:plc:batsswaxvws26rr3gf7wvm7k",
    "did:plc:mpivxdlwzdjsb2kca6u2nwmp",
    "did:plc:lhbjaqqvdd4754apaj2tvrcc",
    "did:plc:sirkh6lr4qzftndgtssugecq",
    "did:plc:yc5i6nuv3ikize7ogzymuxdc",
    "did:plc:zkl3munj3wkryitomialsaeb",
    "did:plc:o776gyjla3op3s6unajlhtlc",
    "did:plc:ek4mtqkxgvqrpoia4bkhioon",
    "did:plc:m5xstnab7bsbor2ywjzdccbm",
    "did:plc:j5sogsw5ejwwo6megyxxdwri",
    "did:plc:4ske3eeybp4wtj4k2xpjhmj2",
    "did:plc:brhv3xvi7gmfv7e6d57j33qd",
    "did:plc:b7tjuc7sh76giutk44jkrtbe",
    "did:plc:qbjqfyhsrb3euldz3f2uze7d",
    "did:plc:to45nnl5mh4zz25hozyitbnw",
    "did:plc:iwmgpfnysyzkewdppodsy7h6",
    "did:plc:esu5gl65pt7p2azu53zzagfg",
    "did:plc:co7y2zamzs5jxpha27lxinsg",
    "did:plc:jvsnpavici3i2hbb23wp7rai",
    "did:plc:bexnuogium744jj6ibk4bhy3",
    "did:plc:npmp7gxfv7ojr4osmsbm6kfy",
    "did:plc:llgfbjvsqkaicezsf7mzjxr3",
    "did:plc:hkqmm7bhqjucm6xeitorj65t",
    "did:plc:xjxuc7gt7s3wdjil7txspyya",
  ];

  const randomRepoDID =
    overrides.repoDID ||
    blueskyDIDs[Math.floor(Math.random() * blueskyDIDs.length)];
  const user =
    overrides.repo?.handle ||
    overrides.repoDID ||
    overrides.signingKeyDID ||
    "user" + Math.floor(Math.random() * 1000);

  const livestreamTitles = [
    "Coding with Friends",
    "Building a React App8q7hiqgf973b9qbilrhqo7obyo83qglfiyi!",
    "Game Dev Stream",
    "Let's Play!8q7hiqgf973b9qbilrhqo7obyo83qglfiyi!",
    "Chill Vibes & Code",
    "React Native Tutorial8q7hiqgf973b9qbilrhqo7obyo83qglfiyi!",
    "Node.js Backend",
    "My First Streamo8q7hiqgf973b9qbilrhqo7obyo83qglfiyi!",
    "Live Coding Session",
    "Web3 Development",
    "Streaming Some Games",
    "A Random Stream",
    "DevOps Practice",
    "Frontend Fun",
    "Backend Bonanza",
    "Debugging Time",
    "Let's Code Together8q7hiqgf973b9qbilrhqo7obyo83qglfiyi!",
    "Building a SaaS8q7hiqgf973b9qbilrhqo7obyo83qglfiyi!",
    "Design and Code",
    "Gaming with the Crew",
    "Just Chatting",
    "Music and Code8q7hiqgf973b9qbilrhqo7obyo83qglfiyi!",
    "Art and Code",
    "Open Source Project",
    "Live Q&A",
    "Working on a Side Project",
    "Making a Mobile Game",
    "Tech Talk",
    "Learning a New Language",
    "Solving Problems Live",
  ];

  const randomTitle =
    overrides.title ||
    livestreamTitles[Math.floor(Math.random() * livestreamTitles.length)];

  return {
    id: overrides.id || Math.random().toString(36).substring(2, 15),
    repoDID: randomRepoDID,
    signingKeyDID:
      overrides.signingKeyDID ||
      "did:mock:example" + Math.random().toString(36).substring(2, 15),
    startTime: overrides.startTime || now.toISOString(),
    title: randomTitle,
    repo: overrides.repo || {
      did: randomRepoDID,
      handle: user, // Replace with actual handle lookup if possible
      pds: "bsky.social", // Replace with actual display name lookup if possible
      rootCid: "invalid", // Replace with actual avatar lookup if possible
      version: "0.1",
    },
    viewers: overrides.viewers || Math.floor(Math.random() * 100),
    streamRecord: overrides.streamRecord || {
      createdAt: now,
      title: randomTitle,
      url: "https://example.com/stream",
    },
    ...overrides,
  };
}
