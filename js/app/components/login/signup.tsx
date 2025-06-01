import { useNavigation } from "@react-navigation/native";
import { CircleHelp } from "@tamagui/lucide-icons";
import { useToastController } from "@tamagui/toast";
import Loading from "components/loading/loading";
import {
  login,
  selectIsReady,
  selectLogin,
  selectUserProfile,
} from "features/bluesky/blueskySlice";
import { useEffect, useState } from "react";
import { KeyboardAvoidingView, Linking, Pressable } from "react-native";
import { useAppDispatch, useAppSelector } from "store/hooks";
import {
  Button,
  Form,
  Input,
  Spinner,
  Text,
  View,
  XStack,
  YStack,
} from "tamagui";

export default function SignUp() {
  const dispatch = useAppDispatch();
  const userProfile = useAppSelector(selectUserProfile);
  const loginState = useAppSelector(selectLogin);
  const [pds, setPDS] = useState<string>("https://bsky.social");
  const isReady = useAppSelector(selectIsReady);
  const toast = useToastController();
  const navigation = useNavigation();

  const onSubmit = () => {
    let thisPds = pds;
    if (thisPds === undefined) {
      thisPds = "https://bsky.social";
    }
    dispatch(login(thisPds));
  };

  const onEnterPress = (e: any) => {
    if (e.nativeEvent.key === "Enter") {
      onSubmit();
    }
  };

  useEffect(() => {
    if (loginState?.error) {
      toast.show("Login error", {
        message: loginState.error,
      });
    }
  }, [loginState?.error]);

  if (!isReady) {
    return (
      <View f={1} jc="center" ai="stretch" gap="$3">
        <Loading />
      </View>
    );
  }

  if (userProfile) {
    // navigate to /login
    navigation.navigate("Login");
    return (
      <View f={1} jc="center" ai="stretch" gap="$3">
        <Loading />
      </View>
    );
  }

  return (
    <KeyboardAvoidingView style={{ flex: 1 }} behavior="padding">
      <Form flex={1} onSubmit={onSubmit}>
        <View
          f={1}
          jc="center"
          ai="center"
          padding="$4"
          width="100%"
          marginHorizontal="auto"
        >
          <YStack
            px="$6"
            py="$6"
            br="$4"
            backgroundColor="$color2"
            width="100%"
            maxWidth={600}
            gap="$2"
          >
            <Text fontSize="$9" fontWeight="200">
              Sign up
            </Text>
            <View flexWrap="wrap" flexDirection="row" gap="$1.5">
              <Text color="$color11">
                We'll redirect you to your chosen PDS
              </Text>
              <Pressable
                onPress={() => {
                  const u = new URL(
                    "https://atproto.academy/docs/glossary#pds-personal-data-server",
                  );
                  Linking.openURL(u.toString());
                }}
              >
                <CircleHelp size={18} color="lightskyblue" />
              </Pressable>{" "}
              <Text color="$color11">to sign up.</Text>
              <Text color="$color11">
                Then, log in with the account you just created.
              </Text>
            </View>
            <YStack gap="$2" py="$4">
              <Text htmlFor="pdsUrl" color="$color11">
                PDS URL
              </Text>
              <Input
                id="pdsUrl"
                value={pds}
                onChangeText={setPDS}
                backgroundColor="$color2"
                onSubmitEditing={onEnterPress}
              />
            </YStack>

            <XStack justifyContent="space-between">
              <Button
                onPress={() => navigation.navigate("Login")}
                backgroundColor="$gray3"
                color="$color"
              >
                Log In
              </Button>
              <Form.Trigger asChild>
                <Button
                  px="$6"
                  // @ts-expect-error Not in the type definition but required for web.
                  type="submit"
                  backgroundColor="$accentColor"
                  disabled={loginState.loading}
                >
                  <Text>{loginState.loading ? <Spinner /> : `Sign up`}</Text>
                </Button>
              </Form.Trigger>
            </XStack>
          </YStack>
        </View>
      </Form>
    </KeyboardAvoidingView>
  );
}
