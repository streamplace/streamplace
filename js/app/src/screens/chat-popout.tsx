import {
  Chat,
  ChatBox,
  LivestreamProvider,
  PlayerProvider,
  ThemeProvider,
  usePlayerStore,
} from "@streamplace/components";
import emojiData from "assets/emoji-data.json";
import { selectUserProfile } from "features/bluesky/blueskySlice";
import { useEffect } from "react";
import { useAppSelector } from "store/hooks";
import { View } from "tamagui";

export default function PopoutChat({ route }) {
  const user = route.params?.user;
  if (typeof user !== "string") {
    return <View />;
  }

  return (
    <ThemeProvider>
      <LivestreamProvider src={user}>
        <PlayerProvider>
          <PopoutChatInner user={user} />
        </PlayerProvider>
      </LivestreamProvider>
    </ThemeProvider>
  );
}

export function PopoutChatInner({ user }: { user: string }) {
  const setSrc = usePlayerStore((x) => x.setSrc);
  const profile = useAppSelector(selectUserProfile);
  useEffect(() => {
    setSrc(user);
  }, [user]);
  return (
    <View position="relative" f={1} margin="$2">
      <View f={1} position="absolute" width="100%" minHeight="100%" bottom={0}>
        <Chat canModerate={profile?.handle === user} />
        {profile && <ChatBox emojiData={emojiData} isPopout={true} />}
      </View>
    </View>
  );
}
