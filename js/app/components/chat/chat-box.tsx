import { Send } from "@tamagui/lucide-icons";
import { useRef, useState } from "react";
import { TextInput } from "react-native";
import { Button, Form, isWeb, TextArea } from "tamagui";
import { Keyboard } from "react-native";

export default function ChatBox() {
  const [message, setMessage] = useState("");
  const textAreaRef = useRef<TextInput>(null);
  const submit = () => {
    Keyboard.dismiss();
    setMessage("");
    if (isWeb && textAreaRef.current) {
      const textarea = textAreaRef.current as unknown as HTMLTextAreaElement;
      textarea.style.height = "";
    }
  };
  return (
    <Form flexDirection="row" padding={2} alignItems="center">
      <TextArea
        flexGrow={1}
        flexShrink={1}
        borderRadius={0}
        numberOfLines={1}
        overflow="hidden"
        returnKeyType="send"
        returnKeyLabel="Send"
        rows={1}
        value={message}
        ref={textAreaRef}
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
