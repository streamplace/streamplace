import { useEffect, useState } from "react";
import { Button, Input, Label, Paragraph, TextArea, View } from "tamagui";
import Loading from "./loading/loading";
import { useToastController } from "@tamagui/toast";
import useAquareumNode from "hooks/useAquareumNode";
import { useIsFocused } from "@react-navigation/native";
import schema from "generated/eip712-schema.json";
import useWallet from "hooks/useWallet";

const Left = ({ children }: { children: React.ReactNode }) => {
  return (
    <View f={2} fb={0}>
      {children}
    </View>
  );
};

const Right = ({ children }: { children: React.ReactNode }) => {
  return (
    <View f={6} fb={0}>
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
  useEffect(() => {
    (async () => {
      const res = await fetch(`${url}/api/settings`);
      const data = (await res.json()) as Settings;
      setId(data.id);
      setStreamer(data.streamer);
      setTitle(data.title);
    })();
  }, [isFocused, refreshTime]);
  const [id, setId] = useState("");
  const [streamer, setStreamer] = useState("");
  const [title, setTitle] = useState("");
  const [loading, setLoading] = useState(false);
  const disabled = loading || streamer === "" || title === "";
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
          <Paragraph pb="$2">Streamer</Paragraph>
        </Left>
        <Right>
          <Input
            value={streamer}
            onChangeText={setStreamer}
            w="100%"
            size="$4"
          />
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
          onPress={() => {
            setLoading(true);
            console.log(address);
            (async () => {
              try {
                const message = {
                  signer: address,
                  time: Date.now(),
                  data: { streamer, title },
                };
                const toSign = {
                  types: schema.types,
                  domain: schema.domain as any,
                  primaryType: "GoLive",
                  message: message,
                };
                const signature = await signTypedData(toSign);
                const res = await fetch(`${url}/api/settings/${id}`, {
                  method: "PUT",
                  body: JSON.stringify({
                    primaryType: "GoLive",
                    domain: schema.domain,
                    message: message,
                    signature: signature,
                  }),
                });
                if (!res.ok) {
                  const text = await res.text();
                  throw new Error(`http ${res.status} ${text}`);
                }
                toast.show("Settings Saved", {
                  message: "Great job.",
                });
                setRefreshTime(Date.now());
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
          {loading ? "Loading..." : "Save"}
        </Button>
      </View>
    </View>
  );
}
