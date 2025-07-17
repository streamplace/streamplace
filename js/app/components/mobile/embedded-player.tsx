import {
  LivestreamProvider,
  Player as PlayerInnerInner,
  PlayerProps,
  PlayerProvider,
  View,
} from "@streamplace/components";
import { DesktopUi } from "./desktop-ui";
import { OfflineCounter } from "./offline-counter";

export function EmbeddedPlayer(
  props: Partial<PlayerProps> & {
    width?: number;
    height?: number;
  },
) {
  const { width = 854, height = 480 } = props;

  return (
    <LivestreamProvider src={props.src ?? ""}>
      <PlayerProvider defaultId={props.playerId || undefined}>
        <View
          style={{
            width,
            height,
            position: "relative",
          }}
        >
          <PlayerInnerInner {...props}>
            <DesktopUi />
            <OfflineCounter isMobile={false} />
          </PlayerInnerInner>
        </View>
      </PlayerProvider>
    </LivestreamProvider>
  );
}
