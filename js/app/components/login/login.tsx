import { Button, View, Text } from "tamagui";
import { useAppDispatch, useAppSelector } from "store/hooks";
import {
  login,
  logout,
  selectUserProfile,
} from "features/bluesky/blueskySlice";
import usePlatform from "hooks/usePlatform";

export default function Login() {
  const dispatch = useAppDispatch();
  const userProfile = useAppSelector(selectUserProfile);
  const { isWeb } = usePlatform();

  if (userProfile) {
    return (
      <View f={1} jc="center" ai="center">
        <Text>Logged in as @{userProfile.handle}</Text>
        <Button onPress={() => dispatch(logout())}>Log out</Button>
      </View>
    );
  }

  return (
    <View f={1} jc="center" ai="center">
      {/* <Text>{error}</Text> */}
      <Button
        onPress={async () => {
          dispatch(login("https://bsky.social"));
        }}
      >
        Log in with Bluesky
      </Button>
      {isWeb && (
        <a href="link.iameli.fairway:/bsky.social/login">Return to app?</a>
      )}
    </View>
  );
}
