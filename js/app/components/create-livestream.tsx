import { useEffect, useState } from "react";
import { Button, Label, Paragraph, TextArea, View } from "tamagui";
import { Switch } from "react-native";
import { useToastController } from "@tamagui/toast";
import { useIsFocused } from "@react-navigation/native";
import {
  createLivestreamRecord,
  golivePost,
  selectNewLivestream,
  selectUserProfile,
} from "features/bluesky/blueskySlice";
import { useAppDispatch, useAppSelector } from "store/hooks";
import { useLiveUser } from "hooks/useLiveUser";

const Left = ({ children }: { children: React.ReactNode }) => {
  return (
    <View f={2} fg={2} fb={0}>
      {children}
    </View>
  );
};

const Right = ({ children }: { children: React.ReactNode }) => {
  return (
    <View f={6} fb={0} fg={6}>
      {children}
    </View>
  );
};

export default function CreateLivestream() {
  const dispatch = useAppDispatch();
  const toast = useToastController();
  // const { url } = useAquareumNode();
  const userIsLive = useLiveUser();
  const isFocused = useIsFocused();
  const [title, setTitle] = useState("");
  const [loading, setLoading] = useState(false);
  const [postToBluesky, setPostToBluesky] = useState(true);
  const profile = useAppSelector(selectUserProfile);
  const newLivestream = useAppSelector(selectNewLivestream);
  useEffect(() => {
    if (newLivestream?.record) {
      toast.show("Livestream announced", {
        message: newLivestream.record.title,
      });
      setTitle("");
    }
  }, [newLivestream?.record]);
  useEffect(() => {
    if (newLivestream?.error) {
      toast.show("Error creating livestream", {
        message: newLivestream.error,
      });
    }
  }, [newLivestream?.error]);
  const disabled = !userIsLive || loading || title === "";

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
            <View f={1} jc="center" alignItems="flex-start">
              <Switch
                value={postToBluesky}
                onValueChange={setPostToBluesky}
              ></Switch>
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
            dispatch(createLivestreamRecord({ title }));
            if (postToBluesky) {
              dispatch(golivePost({ text: title }));
            }
          }}
        >
          {buttonText(loading, userIsLive)}
        </Button>
      </View>
    </View>
  );
}

const buttonText = (loading: boolean, userIsLive: boolean) => {
  if (loading) {
    return "Loading...";
  }
  if (!userIsLive) {
    return "Waiting for stream to start...";
  }
  return "Announce Livestream!";
};
