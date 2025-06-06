import { Player } from "components";
import { Redirect } from "components/aqlink";
import Loading from "components/loading/loading";
import {
  selectIsReady,
  selectUserProfile,
} from "features/bluesky/blueskySlice";
import { useAppSelector } from "store/hooks";
import { View } from "tamagui";

export default function PhoneLive() {
  const isReady = useAppSelector(selectIsReady);
  const userProfile = useAppSelector(selectUserProfile);
  if (!isReady) {
    return <Loading />;
  }
  if (!userProfile) {
    return <Redirect to={{ screen: "Login" }} />;
  }
  return (
    <View f={1}>
      <Player src={userProfile.did} name={userProfile.handle} ingest={true} />
    </View>
  );
}
