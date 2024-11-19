import { useEffect, useState } from "react";
import { Button, Label, Paragraph, TextArea, View } from "tamagui";
import Loading from "./loading/loading";
import { useToastController } from "@tamagui/toast";
import useAquareumNode from "hooks/useAquareumNode";
import { useIsFocused } from "@react-navigation/native";
import useWallet from "hooks/useWallet";
import { golivePost, selectUserProfile } from "features/bluesky/blueskySlice";
import { useAppDispatch, useAppSelector } from "store/hooks";
import AQLink from "./aqlink";

const Left = ({ children }: { children: React.ReactNode }) => {
  return (
    <View f={2} fb={0}>
      {children}
    </View>
  );
};

const Right = ({ children }: { children: React.ReactNode }) => {
  return (
    <View f={6} fb={0} alignItems="stretch">
      {children}
    </View>
  );
};
type Settings = {
  id: string;
  streamer: string;
  title: string;
};

export default function GoLive() {
  const toast = useToastController();
  const { url } = useAquareumNode();
  const isFocused = useIsFocused();
  const { address, signTypedData } = useWallet();
  const [refreshTime, setRefreshTime] = useState(0);
  const profile = useAppSelector(selectUserProfile);
  const dispatch = useAppDispatch();
  useEffect(() => {
    (async () => {
      const res = await fetch(`${url}/api/settings`);
      const data = (await res.json()) as Settings;
      setId(data.id);
      setTitle(data.title);
    })();
  }, [isFocused, refreshTime]);
  const [id, setId] = useState("");
  const [title, setTitle] = useState("");
  const [loading, setLoading] = useState(false);
  const disabled = !profile || loading || title === "";
  if (id === "") {
    return (
      <View f={1} ai="center" jc="center" w="100%" p="$4">
        <Loading />
      </View>
    );
  }
  return (
    <View f={1} ai="center" jc="center" gap="$4" w="100%" p="$4" maxWidth={500}>
      <Label w="100%">
        <Left>
          <Paragraph>Signing Key ID</Paragraph>
        </Left>
        <Right>
          <Paragraph>{id}</Paragraph>
        </Right>
      </Label>
      <Label w="100%">
        <Left>
          <Paragraph>Streamer</Paragraph>
        </Left>
        <Right>
          {!profile && (
            <AQLink to={{ screen: "Login" }} style={{ display: "flex" }}>
              <Paragraph color="$accentColor">Log in with Bluesky</Paragraph>
            </AQLink>
          )}
          {profile && <Paragraph>@{profile.handle}</Paragraph>}
        </Right>
      </Label>
      <Label w="100%">
        <Left>
          <Paragraph pb="$2">Title</Paragraph>
        </Left>
        <Right>
          <TextArea
            value={title}
            onChangeText={setTitle}
            w="100%"
            size="$4"
            minHeight={100}
          />
        </Right>
      </Label>
      <View gap="$2" w="100%">
        <Button
          disabled={disabled}
          opacity={disabled ? 0.5 : 1}
          w="100%"
          size="$4"
          onPress={async () => {
            setLoading(true);
            if (!url) {
              throw new Error("No node URL");
            }
            try {
              await dispatch(
                golivePost({ nodeUrl: url, signingKey: id, text: title }),
              );
              toast.show("Posted!", {
                message: `Great success!`,
              });
            } catch (e) {
              toast.show("Error creating post", {
                message: e.mesasge,
              });
            } finally {
              setLoading(false);
            }
          }}
        >
          {loading ? "Loading..." : "Save"}
        </Button>
      </View>
    </View>
  );
}
