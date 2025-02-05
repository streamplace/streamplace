import AQLink from "components/aqlink";
import ErrorBox from "components/error/error";
import Loading from "components/loading/loading";
import {
  pollSegments,
  selectRecentSegments,
} from "features/streamplace/streamplaceSlice";
import useStreamplaceNode from "hooks/useStreamplaceNode";
import { useEffect, useState } from "react";
import { RefreshControl } from "react-native";
import { useAppDispatch, useAppSelector } from "store/hooks";
import { H6, Image, ScrollView, ScrollViewProps, Text, View } from "tamagui";

type Segment = {
  id: string;
  repoDID: string;
  signingKeyDID: string;
  startTime: string;
  repo: Repo;
};

type Repo = {
  did: string;
  handle: string;
  pds: string;
  version: string;
  rootCid: string;
};

export default function StreamList({
  contentContainerStyle = {},
}: {
  contentContainerStyle?: Exclude<
    ScrollViewProps["contentContainerStyle"],
    string
  >;
}) {
  const [retryTime, setRetryTime] = useState<number>(Date.now());
  const { url } = useStreamplaceNode();
  const { segments, error, loading } = useAppSelector(selectRecentSegments);
  const dispatch = useAppDispatch();
  useEffect(() => {
    const interval = setInterval(() => {
      setRetryTime(Date.now());
    }, 10000);
    return () => clearInterval(interval);
  }, []);
  useEffect(() => {
    dispatch(pollSegments());
  }, [url, retryTime]);
  if (error) {
    return (
      <ErrorBox
        onRetry={() => {
          setRetryTime(Date.now());
        }}
      />
    );
  }
  if (loading && segments.length === 0) {
    return <Loading></Loading>;
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
          refreshing={loading}
          onRefresh={() => setRetryTime(Date.now())}
        />
      }
    >
      {segments.map((segment, i) => {
        const user =
          segment.repo?.handle || segment.repoDID || segment.signingKeyDID;
        return (
          <View flex={1} key={i} alignItems="stretch">
            <AQLink
              to={{
                screen: "Stream",
                params: {
                  user: user,
                },
              }}
            >
              <View
                alignItems="center"
                display="flex"
                position="relative"
                maxWidth={400}
                flexBasis="100%"
                marginHorizontal="auto"
              >
                <Image
                  f={1}
                  aspectRatio={16 / 9}
                  width="100%"
                  src={`${url}/api/playback/${user}/stream.jpg`}
                  resizeMode="contain"
                  objectFit="contain"
                />
                <View
                  position="absolute"
                  top={0}
                  right={0}
                  flexDirection="row"
                  justifyContent="center"
                  alignItems="center"
                  overflow="visible"
                >
                  <View position="relative">
                    <Text
                      textShadowColor="black"
                      textShadowOffset={{ width: -1, height: 1 }}
                      textShadowRadius={3}
                    >
                      LIVE
                    </Text>
                    <Text
                      textShadowColor="black"
                      textShadowOffset={{ width: 1, height: -1 }}
                      textShadowRadius={3}
                      position="absolute"
                    >
                      LIVE
                    </Text>
                  </View>
                  <View
                    bg="$red10"
                    w={15}
                    h={15}
                    margin={5}
                    borderRadius="$10"
                  />
                </View>
                <H6>
                  {segment.repo?.handle ? `@${segment.repo.handle}` : user}
                </H6>
              </View>
            </AQLink>
          </View>
        );
      })}
      <View f={1} justifyContent="center" alignItems="center">
        {segments.length === 0 && <H6>No one is streaming right now 😭</H6>}
      </View>
    </ScrollView>
  );
}
