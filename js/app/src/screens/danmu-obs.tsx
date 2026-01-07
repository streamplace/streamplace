import {
  DanmuOverlayOBS,
  LivestreamProvider,
  PlayerProvider,
  usePlayerStore,
} from "@streamplace/components";
import { useEffect } from "react";
import { Platform, View } from "react-native";
import { useStore } from "store";

const isWeb = Platform.OS === "web";

interface DanmuParams {
  opacity?: number;
  speed?: number;
  laneCount?: number;
  maxMessages?: number;
  enabled?: boolean;
}

const parseDanmuParams = (query: URLSearchParams): DanmuParams => {
  const params: DanmuParams = {};

  const opacity = query.get("opacity");
  if (opacity) params.opacity = parseInt(opacity);

  const speed = query.get("speed");
  if (speed) params.speed = parseFloat(speed);

  const laneCount = query.get("laneCount");
  if (laneCount) params.laneCount = parseInt(laneCount);

  const maxMessages = query.get("maxMessages");
  if (maxMessages) params.maxMessages = parseInt(maxMessages);

  const enabled = query.get("enabled");
  if (enabled !== null) params.enabled = enabled !== "false";

  return params;
};

export default function DanmuOBSScreen({ route }) {
  const user = route.params?.user;
  const setSidebarHidden = useStore((state) => state.setSidebarHidden);
  const setSidebarUnhidden = useStore((state) => state.setSidebarUnhidden);

  let danmuParams: DanmuParams = {};
  if (isWeb) {
    danmuParams = parseDanmuParams(new URLSearchParams(window.location.search));
  }

  if (typeof user !== "string") {
    return <View />;
  }

  useEffect(() => {
    setSidebarHidden();
    () => {
      // on unmount, unhide the sidebar
      setSidebarUnhidden();
    };
  }, []);

  return (
    <LivestreamProvider src={user}>
      <PlayerProvider>
        <DanmuOBSInner user={user} {...danmuParams} />
      </PlayerProvider>
    </LivestreamProvider>
  );
}

export function DanmuOBSInner({
  user,
  ...danmuParams
}: {
  user: string;
} & DanmuParams) {
  const setSrc = usePlayerStore((x) => x.setSrc);
  useEffect(() => {
    setSrc(user);
  }, [user]);

  return (
    <View style={{ position: "absolute", width: "100%", height: "100%" }}>
      <DanmuOverlayOBS channelDid={user} {...danmuParams} />
    </View>
  );
}
