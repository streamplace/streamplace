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
  const { url } = useAquareumNode();
  useEffect(() => {
    (async () => {
      const res = await fetch(`${url}/api/segment/recent`);
      const data = (await res.json()) as Segment[];
      setStreams(data);
    })();
  }, [url]);
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
