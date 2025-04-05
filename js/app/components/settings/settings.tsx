import {
  selectTelemetry,
  setURL,
  telemetryOpt,
} from "features/streamplace/streamplaceSlice";
import useStreamplaceNode from "hooks/useStreamplaceNode";
import { useState } from "react";
import { useAppDispatch, useAppSelector } from "store/hooks";
import { Button, Form, H3, Input, View, XStack, Label } from "tamagui";
import { Updates } from "./updates";
import { Switch } from "react-native";

export function Settings() {
  const dispatch = useAppDispatch();
  const { url } = useStreamplaceNode();
  const [newUrl, setNewUrl] = useState("");
  const onSubmit = () => {
    dispatch(setURL(newUrl));
    setNewUrl("");
  };
  const telemetry = useAppSelector(selectTelemetry);
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
      <View
        alignItems="center"
        justifyContent="center"
        gap="$2"
        fg={1}
        flexBasis={0}
      >
        <Label>
          <H3>Optional Player Telemetry</H3>
        </Label>
        <View backgroundColor="$outlineColor" padding="$5" borderRadius="$20">
          <Switch
            style={{
              transform: [{ scaleX: 1.5 }, { scaleY: 1.5 }],
            }}
            value={telemetry === true}
            onValueChange={(checked) => {
              if (checked === true) {
                dispatch(telemetryOpt(true));
              } else {
                dispatch(telemetryOpt(false));
              }
            }}
          />
        </View>
      </View>
    </View>
  );
}
