import ErrorBox from "components/error/error";
import Loading from "components/loading/loading";
import { Link } from "expo-router";
import useAquareumNode from "hooks/useAquareumNode";
import { useEffect, useState } from "react";
import { Pressable } from "react-native";
import { ScrollView, Text, Image, View, H2, H6 } from "tamagui";

type Segment = {
  id: string;
  user: string;
  startTime: string;
  endTime: string;
};

export default function StreamList() {
  const [streams, setStreams] = useState<Segment[]>([]);
  const [error, setError] = useState<boolean>(false);
  const [loading, setLoading] = useState<boolean>(false);
  const [retryTime, setRetryTime] = useState<number>(Date.now());
  const { url } = useAquareumNode();
  useEffect(() => {
    setError(false);
    setLoading(true);
    (async () => {
      try {
        const res = await fetch(`${url}/api/segment/recent`);
        const data = (await res.json()) as Segment[];
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
    <ScrollView contentContainerStyle={{ alignItems: "center" }}>
      {streams.map((seg) => (
        <Link asChild key={seg.user} href={`/stream/${seg.user}`}>
          <Pressable>
            <View key={seg.user}>
              <Image
                height={200}
                src={`${url}/api/playback/${seg.user}/stream.jpg`}
                resizeMode="contain"
                objectFit="contain"
              />
              <H6>{seg.user}</H6>
            </View>
          </Pressable>
        </Link>
      ))}
    </ScrollView>
  );
}
