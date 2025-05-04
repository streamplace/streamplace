import NameColorPicker from "components/name-color-picker/name-color-picker";
import {
  login,
  logout,
  selectLogin,
  selectPDS,
  selectUserProfile,
  setPDS,
} from "features/bluesky/blueskySlice";
import { useState } from "react";
import { Keyboard } from "react-native";
import { useAppDispatch, useAppSelector } from "store/hooks";
import { Button, Form, H3, Input, Sheet, Spinner, Text, View } from "tamagui";

export default function Login() {
  const dispatch = useAppDispatch();
  const userProfile = useAppSelector(selectUserProfile);
  const pds = useAppSelector(selectPDS);
  const loginState = useAppSelector(selectLogin);
  const [open, setOpen] = useState(false);
  const onOpenChange = (open: boolean) => {
    setOpen(open);
    Keyboard.dismiss();
  };

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
    <View f={1} jc="center" ai="center" backgroundColor="$gray1" padding="$4">
      <ChangePDS open={open} onOpenChange={onOpenChange} />
      {/* <Text>{error}</Text> */}
      <Button
        width="100%"
        onPress={async () => {
          await dispatch(login(`https://${pds.url}`));
        }}
        margin="$4"
        backgroundColor="$accentColor"
        disabled={loginState.loading}
      >
        <Text>
          {loginState.loading ? <Spinner /> : `Log in with ${pds.url}`}
        </Text>
      </Button>
      <Button width="100%" onPress={() => onOpenChange(true)} margin="$4">
        Change PDS
      </Button>
    </View>
  );
}

export function ChangePDS({
  open,
  onOpenChange,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
}) {
  const pds = useAppSelector(selectPDS);
  const dispatch = useAppDispatch();
  const [newURL, setNewURL] = useState("");
  return (
    <Sheet
      forceRemoveScrollEnabled={open}
      modal={true}
      open={open}
      onOpenChange={onOpenChange}
      dismissOnSnapToBottom
      zIndex={100_000}
      animation="medium"
    >
      <Sheet.Overlay
        animation="lazy"
        enterStyle={{ opacity: 0 }}
        exitStyle={{ opacity: 0 }}
      />

      <Sheet.Handle />
      <Sheet.Frame
        padding="$4"
        justifyContent="center"
        alignItems="center"
        gap="$5"
        backgroundColor="$accentBackground"
      >
        <Form
          justifyContent="center"
          alignItems="stretch"
          width="100%"
          gap="$5"
          f={1}
          display="flex"
          onSubmit={async () => {
            await dispatch(setPDS(newURL));
            onOpenChange(false);
          }}
        >
          {/* <Button
          size="$6"
          circular
          icon={ChevronDown}
          onPress={() => onOpenChange(false)}
        /> */}
          <H3 width="100%" textAlign="left">
            Custom PDS URL:
          </H3>
          <Input
            width="100%"
            placeholder="example.com"
            textContentType="URL"
            keyboardType="url"
            value={newURL}
            onChangeText={(text) => setNewURL(text)}
          />
          <Form.Trigger asChild disabled={pds.loading}>
            <Button width="100%" size="$6" backgroundColor="$accentColor">
              {pds.loading ? <Spinner /> : <Text>Save</Text>}
            </Button>
          </Form.Trigger>
          <Button
            width="100%"
            size="$6"
            onPress={async () => {
              await dispatch(setPDS("bsky.social"));
              onOpenChange(false);
            }}
          >
            <Text>Use default (bsky.social)</Text>
          </Button>
        </Form>
      </Sheet.Frame>
    </Sheet>
  );
}
