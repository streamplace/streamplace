import AQLink from "components/aqlink";
import ErrorBox from "components/error/error";
import Loading from "components/loading/loading";
import StreamCardHorizontal from "components/ui/cards/horizontal";
import Container from "components/ui/container";
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
import { H2, H3, H6, ScrollView, ScrollViewProps, View } from "tamagui";

type Segment = {
  id: string;
  repoDID: string;
  signingKeyDID: string;
  startTime: string;
  repo: Repo;
  viewers: number;
};

function HomeScreenItem({ item }: { item: Segment }) {
  const user = item.repo?.handle || item.repoDID || item.signingKeyDID;
  return (
    <View marginHorizontal="$2">
      <AQLink
        to={{
          screen: "Stream",
          params: {
            user: user,
          },
        }}
      >
        <StreamCardHorizontal
          size="md"
          thumbnailUrl={`https://stream.place/api/playback/${user}/stream.png`}
          avatarUrl="https://cdn.bsky.app/img/avatar/plain/did:plc:4ukwiehjoytl56ysom2pdwko/bafkreieal2i74ynzrvofia6fa3efqnyxmox76ohrfldt5kvls73lbspzdm@jpeg"
          streamerName={user}
          category={[]}
          viewers={item.viewers}
          isLive={true}
        />
      </AQLink>
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
  const { segments, error, loading, firstRequest } =
    useAppSelector(selectRecentSegments);
  const dispatch = useAppDispatch();
  const [manualRefresh, setManualRefresh] = useState(false);
  useEffect(() => {
    dispatch(pollSegments());
  }, []);
  useEffect(() => {
    if (!loading) {
      setManualRefresh(false);
    }
  }, [loading]);
  if (error) {
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
  if (firstRequest) {
    return <Loading />;
  }
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
            dispatch(pollSegments());
            setManualRefresh(true);
          }}
        />
      }
    >
      <Container>
        <Title marginVertical={16}>
          {segments.length} {segments.length === 1 ? "person" : "people"} live
          now
        </Title>
        <View f={1} justifyContent="center" alignItems="center">
          <FlatList
            renderItem={HomeScreenItem}
            numColumns={3}
            data={segments}
          ></FlatList>
          {segments.length === 0 && (
            <>
              <H6>No one is streaming right now 😭</H6>
            </>
          )}
        </View>
      </Container>
    </ScrollView>
  );
}
