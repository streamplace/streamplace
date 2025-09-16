import { KeepAwake } from "@streamplace/components";
import { Redirect } from "components/aqlink";
import { Player } from "components/mobile/player";
import { FullscreenProvider } from "contexts/FullscreenContext";
import { selectUserProfile } from "features/bluesky/blueskySlice";
import { useAppSelector } from "store/hooks";

export default function MobileGoLive() {
  const userProfile = useAppSelector(selectUserProfile);

  if (!userProfile) {
    return <Redirect to={{ screen: "Login" }} />;
  }
  // get player
  return (
    <>
      <KeepAwake />
      <FullscreenProvider>
        <Player ingest src={userProfile.did} name={userProfile.handle} />
      </FullscreenProvider>
    </>
  );
}
