import { useEffect, useState } from "react";
import { Button, H5, Input, Label, Paragraph, TextArea, View } from "tamagui";
import Loading from "./loading/loading";
import { useToastController } from "@tamagui/toast";

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
  creator: string;
  title: string;
};

export default function GoLive() {
  const toast = useToastController();
  useEffect(() => {
    (async () => {
      const res = await fetch(`http://localhost:39090/settings`);
      const data = (await res.json()) as Settings;
      setId(data.id);
      setStreamer(data.creator);
      setTitle(data.title);
    })();
  }, []);
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
          <Paragraph pb="$2">Creator</Paragraph>
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
            (async () => {
              try {
                setLoading(true);
                const res = await fetch(
                  `http://localhost:39090/settings/${id}`,
                  {
                    method: "PUT",
                    body: JSON.stringify({ creator: streamer, title }),
                  },
                );
                if (!res.ok) {
                  const text = await res.text();
                  throw new Error(`http ${res.status} ${text}`);
                }
                toast.show("Settings Saved", {
                  message: "Great job.",
                });
              } catch (e) {
                toast.show("Failed to save settings", {
                  message: e.message,
                });
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
