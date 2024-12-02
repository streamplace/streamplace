import { useNavigation } from "@react-navigation/native";
import AQLink from "components/aqlink";
import ErrorBox from "components/error/error";
import Loading from "components/loading/loading";
import useAquareumNode from "hooks/useAquareumNode";
import { useEffect, useState } from "react";
import { RefreshControl } from "react-native";
import { H6, Image, ScrollView, ScrollViewProps, Text, View } from "tamagui";

type Segment = {
  id: string;
  user: string;
  startTime: string;
  endTime: string;
  repo: Repo;
};

type Repo = {
  did: string;
  handle: string;
  pds: string;
  version: string;
  aquareumKey: string;
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
  const [streams, setStreams] = useState<Segment[]>([]);
  const [error, setError] = useState<boolean>(false);
  const [loading, setLoading] = useState<boolean>(false);
  const [retryTime, setRetryTime] = useState<number>(Date.now());
  const { url } = useAquareumNode();
  const navigation = useNavigation();
  useEffect(() => {
    const interval = setInterval(() => {
      setRetryTime(Date.now());
    }, 10000);
    return () => clearInterval(interval);
  }, []);
  useEffect(() => {
    setError(false);
    setLoading(true);
    (async () => {
      try {
        const res = await fetch(`${url}/api/live-users`);
        if (!res.ok) {
          return;
        }
        const data = await res.json();
        if (!Array.isArray(data)) {
          throw new Error("got non-array back from /api/live-users");
        }
        setStreams(data);
      } catch (e) {
        console.error(e);
        setError(true);
      } finally {
        setLoading(false);
      }
    })();
  }, [url, retryTime]);
  if (error) {
    return <ErrorBox onRetry={() => setRetryTime(Date.now())} />;
  }
  if (loading && streams.length === 0) {
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
      {streams.map((segment, i) => (
        <View flex={1} key={i} alignItems="stretch">
          <AQLink
            to={{ screen: "Stream", params: { user: segment.repo.handle } }}
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
                src={`${url}/api/playback/${segment.repo.aquareumKey}/stream.jpg`}
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
                <View bg="$red10" w={15} h={15} margin={5} borderRadius="$10" />
              </View>
              <H6>@{segment.repo.handle}</H6>
            </View>
          </AQLink>
        </View>
      ))}
      <View f={1} justifyContent="center" alignItems="center">
        {streams.length === 0 && <H6>No one is streaming right now 😭</H6>}
      </View>
    </ScrollView>
  );
}
