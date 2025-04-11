import Chat from "components/chat/chat";
import ChatBox from "components/chat/chat-box";
import PlayerProvider from "components/player/provider";
import { selectUserProfile } from "features/bluesky/blueskySlice";
import { useAppSelector } from "store/hooks";
import { View } from "tamagui";

export default function PopoutChat({ route }) {
  const user = route.params?.user;
  if (typeof user !== "string") {
    return <View />;
  }
  const profile = useAppSelector(selectUserProfile);
  return (
    <PlayerProvider src={user}>
      <View position="relative" f={1}>
        <View
          f={1}
          position="absolute"
          width="100%"
          minHeight="100%"
          bottom={0}
        >
          <Chat />
          {profile && <ChatBox isPopout={true} />}
        </View>
      </View>
    </PlayerProvider>
  );
}
