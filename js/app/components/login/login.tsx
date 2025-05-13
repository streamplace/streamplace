import { AtpBaseClient } from "lexicons";
import NameColorPicker from "components/name-color-picker/name-color-picker";
import {
  login,
  logout,
  selectIsReady,
  selectLogin,
  selectPDS,
  selectUserProfile,
  setPDS,
} from "features/bluesky/blueskySlice";
import { useEffect, useState } from "react";
import { Keyboard, KeyboardAvoidingView } from "react-native";
import { useAppDispatch, useAppSelector } from "store/hooks";
import {
  Button,
  Form,
  H3,
  H5,
  Input,
  Sheet,
  Spinner,
  Text,
  View,
} from "tamagui";
import useStreamplaceNode from "hooks/useStreamplaceNode";
import Loading from "components/loading/loading";
import { useToastController } from "@tamagui/toast";

export default function Login() {
  const dispatch = useAppDispatch();
  const userProfile = useAppSelector(selectUserProfile);
  const pds = useAppSelector(selectPDS);
  const loginState = useAppSelector(selectLogin);
  const [open, setOpen] = useState(false);
  const [handle, setHandle] = useState("");
  const isReady = useAppSelector(selectIsReady);
  const toast = useToastController();
  const onOpenChange = (open: boolean) => {
    setOpen(open);
    Keyboard.dismiss();
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
    return (
      <View f={1} jc="center" ai="stretch" gap="$3">
        <Text textAlign="center">Logged in as @{userProfile.handle}</Text>
        <View flexDirection="row" gap="$2" justifyContent="center">
          <Button
            onPress={() => dispatch(logout())}
            maxWidth={300}
            textAlign="center"
            marginHorizontal="auto"
            flexBasis={250}
          >
            Log out
          </Button>
        </View>
        <NameColorPicker
          buttonProps={{
            textAlign: "center",
            flexBasis: 250,
            maxWidth: 300,
            marginHorizontal: "auto",
          }}
        />
      </View>
    );
  }

  return (
    <KeyboardAvoidingView style={{ flex: 1 }} behavior="padding">
      <View
        f={1}
        jc="center"
        ai="center"
        backgroundColor="$gray1"
        padding="$4"
        width="100%"
        maxWidth={800}
        marginHorizontal="auto"
      >
        <Form
          width="100%"
          maxWidth={800}
          jc="center"
          ai="center"
          onSubmit={async () => {
            await dispatch(login(handle));
          }}
        >
          <H3>Log in with ATProto | Bluesky</H3>
          <H5 alignSelf="flex-start">Handle:</H5>
          <Input
            width="100%"
            placeholder="example.bsky.social"
            value={handle}
            onChangeText={(text) => setHandle(text)}
            keyboardType="url"
            autoCapitalize="none"
            autoComplete="off"
            autoCorrect={false}
          />
          <Form.Trigger asChild>
            <Button
              width="100%"
              margin="$4"
              backgroundColor="$accentColor"
              disabled={loginState.loading}
            >
              <Text>
                {loginState.loading ? <Spinner /> : `Log in with ATProto`}
              </Text>
            </Button>
          </Form.Trigger>
        </Form>
      </View>
    </KeyboardAvoidingView>
  );
}
