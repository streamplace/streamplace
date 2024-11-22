import { Button, Form, H3, Input, View, XStack } from "tamagui";
import { Updates } from "./updates";
import useAquareumNode from "hooks/useAquareumNode";
import { useState } from "react";
import { useAppDispatch } from "store/hooks";
import { setURL } from "features/aquareum/aquareumSlice";

export function Settings() {
  const dispatch = useAppDispatch();
  const { url } = useAquareumNode();
  const [newUrl, setNewUrl] = useState("");
  const onSubmit = () => {
    dispatch(setURL(newUrl));
    setNewUrl("");
  };
  return (
    <View f={1} alignItems="stretch" justifyContent="center" fg={1}>
      <Updates />
      <Form
        fg={1}
        flexBasis={0}
        alignItems="center"
        justifyContent="center"
        padding="$4"
        onSubmit={onSubmit}
      >
        <H3 margin="$2">Change Aquareum Node URL</H3>
        <XStack alignItems="center" space="$2">
          <Input
            value={newUrl}
            flex={1}
            size="$3"
            placeholder={url}
            onChangeText={(t) => setNewUrl(t)}
            onSubmitEditing={onSubmit}
          />
          <Form.Trigger asChild>
            <Button size="$3">Go</Button>
          </Form.Trigger>
        </XStack>
      </Form>
    </View>
  );
}
