import { Send } from "@tamagui/lucide-icons";
import { useRef, useState } from "react";
import { Button, Form, isWeb, View, Input, TextArea } from "tamagui";
import { Keyboard } from "react-native";
import { usePlayerLivestream } from "features/player/playerSlice";
import { useAppDispatch, useAppSelector } from "store/hooks";
import { chatPost } from "features/bluesky/blueskySlice";

export default function ChatBox() {
  const [message, setMessage] = useState("");
  const livestream = useAppSelector(usePlayerLivestream());
  const textAreaRef = useRef<Input>(null);
  const dispatch = useAppDispatch();
  const submit = () => {
    Keyboard.dismiss();
    if (message.length === 0) {
      return;
    }
    if (!livestream) {
      return;
    }
    dispatch(chatPost({ text: message, livestream }));
    setMessage("");
    if (isWeb && textAreaRef.current) {
      const textarea = textAreaRef.current as unknown as HTMLTextAreaElement;
      textarea.style.height = "";
    }
    if (!isWeb) {
      console.log(textAreaRef.current);
    }
  };
  return (
    <Form flexDirection="row" padding={2} alignItems="center">
      <View flexGrow={1} flexShrink={1}>
        <TextArea
          borderRadius={0}
          overflow="hidden"
          returnKeyType="done"
          submitBehavior="blurAndSubmit"
          value={message}
          ref={textAreaRef}
          multiline={true}
          keyboardType="default"
          rows={1}
          onChangeText={(text) => {
            const newMessage = text.replaceAll("\n", "");
            if (newMessage.length > 300) {
              return;
            }
            setMessage(text.replaceAll("\n", ""));
            if (isWeb && textAreaRef.current) {
              const textarea =
                textAreaRef.current as unknown as HTMLTextAreaElement;
              textarea.style.height = "";
              textarea.style.height = textarea.scrollHeight + "px";
            }
          }}
          onKeyPress={(e) => {
            if (e.nativeEvent.key === "Enter") {
              submit();
            }
          }}
        />
      </View>
      <Button
        flexShrink={0}
        backgroundColor="transparent"
        onPress={() => {
          submit();
        }}
      >
        <Send />
      </Button>
    </Form>
  );
}
