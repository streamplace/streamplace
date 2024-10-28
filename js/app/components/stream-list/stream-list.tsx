import { Link } from "@react-navigation/native";
import ErrorBox from "components/error/error";
import Loading from "components/loading/loading";
import useAquareumNode from "hooks/useAquareumNode";
import { useEffect, useState } from "react";
import { Pressable } from "react-native";
import { H6, Image, ScrollView, View } from "tamagui";

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
    <ScrollView contentContainerStyle={{ alignItems: "center" }}>
      {streams.map((seg) => (
        <Link key={seg.user} to={`/stream/${seg.user}`}>
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
