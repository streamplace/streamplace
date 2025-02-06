import { useState } from "react";
import { Button, Label, Paragraph, Switch, TextArea, View } from "tamagui";
import { useToastController } from "@tamagui/toast";
import { useIsFocused } from "@react-navigation/native";
import { selectUserProfile } from "features/bluesky/blueskySlice";
import { useAppSelector } from "store/hooks";

const Left = ({ children }: { children: React.ReactNode }) => {
  return (
    <View f={2} fg={2} fb={0}>
      {children}
    </View>
  );
};

const Right = ({ children }: { children: React.ReactNode }) => {
  return (
    <View f={6} fb={0} fg={6} backgroundColor="red">
      {children}
    </View>
  );
};

export default function CreateLivestream() {
  const toast = useToastController();
  // const { url } = useAquareumNode();
  const isFocused = useIsFocused();
  const [streamer, setStreamer] = useState("");
  const [title, setTitle] = useState("");
  const [loading, setLoading] = useState(false);
  const [postToBluesky, setPostToBluesky] = useState(true);
  const profile = useAppSelector(selectUserProfile);
  const disabled = loading || streamer === "" || title === "";
  return (
    <View
      f={1}
      ai="stretch"
      jc="center"
      gap="$4"
      w="100%"
      p="$4"
      maxWidth={500}
    >
      <Label asChild={true} display="flex">
        <View flexDirection="row">
          <Left>
            <Paragraph pb="$2">Streamer</Paragraph>
          </Left>
          <Right>
            <Paragraph pb="$2">@{profile?.handle}</Paragraph>
          </Right>
        </View>
      </Label>
      <Label asChild={true}>
        <View flexDirection="row">
          <Left>
            <Paragraph pb="$2">Title</Paragraph>
          </Left>
          <Right>
            <TextArea
              value={title}
              onChangeText={setTitle}
              size="$4"
              minHeight={100}
              maxLength={140}
            />
          </Right>
        </View>
      </Label>
      <Label asChild={true}>
        <View flexDirection="row">
          <Left>
            <Paragraph pb="$2" pr="$2" lh={16}>
              Post to Bluesky
            </Paragraph>
          </Left>
          <Right>
            <View backgroundColor="blue" f={1} jc="center">
              <Switch
                size="$3"
                checked={postToBluesky}
                onCheckedChange={setPostToBluesky}
              >
                <Switch.Thumb size="$3" animation="quicker" />
              </Switch>
            </View>
          </Right>
        </View>
      </Label>
      <View gap="$2" w="100%">
        <Button
          disabled={disabled}
          opacity={disabled ? 0.5 : 1}
          w="100%"
          size="$4"
          onPress={() => {
            setLoading(true);
            (async () => {
              try {
              } catch (e) {
                toast.show("Failed to save settings", {
                  message: e.message,
                });
                throw e;
              } finally {
                setLoading(false);
              }
            })();
          }}
        >
          {loading ? "Loading..." : "Announce Livestream!"}
        </Button>
      </View>
    </View>
  );
}
