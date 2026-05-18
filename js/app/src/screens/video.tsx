import { Text, useStreamplaceStore, View, zero } from "@streamplace/components";
import { Platform } from "react-native";
import { Player } from "../../components/mobile/player";

const BBB_HLS_URL = "https://test-streams.mux.dev/x36xhzz/x36xhzz.m3u8";

function usePlaybackUrl(src?: string): string {
  const serverUrl = useStreamplaceStore((x) => x.url);
  if (!src) return BBB_HLS_URL;
  if (src.startsWith("http://") || src.startsWith("https://")) return src;
  return `${serverUrl}/api/playback/${src}/hls/index.m3u8`;
}

function useQuerySrc(routeSrc?: string): string | undefined {
  if (Platform.OS === "web") {
    const qUrl = new URLSearchParams(window.location.search).get("url");
    if (qUrl) return qUrl;
  }
  return routeSrc;
}

export default function VideoScreen({
  route,
}: {
  route?: { params?: { src?: string } };
}) {
  const src = useQuerySrc(route?.params?.src);
  const url = usePlaybackUrl(src);

  return (
    <View style={[zero.flex.values[1], { backgroundColor: "#000" }]}>
      <View style={{ flex: 1 }}>
        <Player src={url} mode="vod" />
      </View>
      {src && (
        <View style={[zero.px[4], zero.py[3], { backgroundColor: "#111" }]}>
          <Text size="sm" style={{ color: "#aaa" }}>
            {src}
          </Text>
        </View>
      )}
    </View>
  );
}
