import CreateLivestream from "components/create-livestream";
import { View } from "tamagui";
import { Player } from "components/player/player";
import Loading from "components/loading/loading";
import {
  selectIsReady,
  selectUserProfile,
} from "features/bluesky/blueskySlice";
import { useAppSelector } from "store/hooks";
import { Redirect } from "components/aqlink";

export default function LiveDashboard() {
  const isReady = useAppSelector(selectIsReady);
  const userProfile = useAppSelector(selectUserProfile);
  if (!isReady) {
    return <Loading />;
  }
  if (!userProfile) {
    return <Redirect to={{ screen: "Login" }} />;
  }
  return (
    <View f={1} ai="stretch" jc="center">
      <View f={1} fb={0}>
        <Player src={userProfile.did} name={userProfile.handle} />
      </View>
      <View f={1} ai="center" jc="center" fb={0}>
        <CreateLivestream />
      </View>
    </View>
  );
}
