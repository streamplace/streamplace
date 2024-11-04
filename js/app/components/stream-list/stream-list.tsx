import { useNavigation } from "@react-navigation/native";
import AQLink from "components/aqlink";
import ErrorBox from "components/error/error";
import Loading from "components/loading/loading";
import { formatAddress } from "hooks/textUtils";
import useAquareumNode from "hooks/useAquareumNode";
import { useEffect, useState } from "react";
import { RefreshControl } from "react-native";
import { H6, Image, ScrollView, ScrollViewProps, View } from "tamagui";

type Segment = {
  id: string;
  user: string;
  startTime: string;
  endTime: string;
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
    setError(false);
    setLoading(true);
    (async () => {
      try {
        const res = await fetch(`${url}/api/segment/recent`);
        if (!res.ok) {
          return;
        }
        const data = await res.json();
        if (!Array.isArray(data)) {
          throw new Error("got non-array back from /api/segment/recent");
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
  if (loading) {
    return <Loading></Loading>;
  }
  if (error) {
    return <ErrorBox onRetry={() => setRetryTime(Date.now())} />;
  }
  return (
    <ScrollView
      contentContainerStyle={{
        alignItems: "stretch",

        ...contentContainerStyle,
      }}
      refreshControl={
        <RefreshControl
          refreshing={loading}
          onRefresh={() => setRetryTime(Date.now())}
        />
      }
    >
      {streams.map((seg) => (
        <View flex={1} key={seg.user}>
          <AQLink to={{ screen: "Stream", params: { user: seg.user } }}>
            <View f={1} alignItems="center" display="flex">
              <Image
                f={1}
                aspectRatio={16 / 9}
                maxWidth={400}
                width="100%"
                src={`${url}/api/playback/${seg.user}/stream.jpg`}
                resizeMode="contain"
                objectFit="contain"
              />
              <H6>{formatAddress(seg.user)}</H6>
            </View>
          </AQLink>
        </View>
      ))}
    </ScrollView>
  );
}
