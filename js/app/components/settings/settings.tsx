import { setURL } from "features/streamplace/streamplaceSlice";
import useStreamplaceNode from "hooks/useStreamplaceNode";
import { useState } from "react";
import { useAppDispatch } from "store/hooks";
import { Button, Form, H3, Input, View, XStack } from "tamagui";
import { Updates } from "./updates";

export function Settings() {
  const dispatch = useAppDispatch();
  const { url } = useStreamplaceNode();
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
        <H3 margin="$2">Change Streamplace Node URL</H3>
        <XStack alignItems="center" space="$2">
          <Input
            value={newUrl}
            flex={1}
            size="$3"
            placeholder={url}
            onChangeText={(t) => setNewUrl(t)}
            onSubmitEditing={onSubmit}
            textContentType="URL"
            autoCapitalize="none"
            autoCorrect={false}
          />
          <Form.Trigger asChild>
            <Button size="$3">Go</Button>
          </Form.Trigger>
        </XStack>
      </Form>
    </View>
  );
}
