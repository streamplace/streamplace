import { useNavigation } from "@react-navigation/native";
import { useToastController } from "@tamagui/toast";
import {
  chatMessage,
  selectIsReady,
  selectUserProfile,
} from "features/bluesky/blueskySlice";
import {
  LivestreamViewHydrated,
  usePlayerLivestream,
} from "features/player/playerSlice";
import {
  chatWarn,
  selectChatWarned,
} from "features/streamplace/streamplaceSlice";
import { useRef, useState } from "react";
import { Keyboard } from "react-native";
import { useAppDispatch, useAppSelector } from "store/hooks";
import { Button, Form, Input, isWeb, TextArea, View } from "tamagui";
import { Palette, SquareArrowOutUpRight } from "@tamagui/lucide-icons";
import NameColorPicker from "components/name-color-picker/name-color-picker";

export default function ChatBox({
  isPopout,
  setIsChatVisible,
  isChatVisible,
}: {
  isPopout?: boolean;
  setIsChatVisible?: (visible: boolean) => void;
  isChatVisible?: boolean;
}) {
  const [message, setMessage] = useState("");
  const isReady = useAppSelector(selectIsReady);
  const userProfile = useAppSelector(selectUserProfile);
  const chatWarned = useAppSelector(selectChatWarned);
  const loggedOut = isReady && !userProfile;
  const livestream = useAppSelector(usePlayerLivestream());
  const textAreaRef = useRef<Input>(null);
  const dispatch = useAppDispatch();
  const navigate = useNavigation();

  const submit = () => {
    if (!isWeb) {
      Keyboard.dismiss();
    }
    if (message.length === 0) {
      return;
    }
    if (!livestream) {
      throw new Error("No livestream");
    }
    dispatch(chatMessage({ text: message, livestream }));
    setMessage("");
    if (isWeb && textAreaRef.current) {
      const textarea = textAreaRef.current as unknown as HTMLTextAreaElement;
      textarea.style.height = "";
    }
  };

  const toast = useToastController();

  return (
    <View position="relative">
      {loggedOut && (
        <View flexDirection="row" justifyContent="center">
          <Button
            backgroundColor="$accentColor"
            onPress={() => {
              navigate.navigate("Login");
            }}
          >
            Log in to chat
          </Button>
          <PopoutButton
            livestream={livestream}
            isPopout={isPopout}
            setIsChatVisible={setIsChatVisible}
          />
        </View>
      )}
      {!loggedOut && (
        <Form
          zIndex={1}
          flexDirection="column"
          padding={2}
          alignItems="stretch"
          opacity={loggedOut ? 0 : 1}
        >
          {isChatVisible && (
            <View flexGrow={1} flexShrink={0}>
              <TextArea
                borderRadius={0}
                overflow="hidden"
                returnKeyType="done"
                submitBehavior="blurAndSubmit"
                value={message}
                ref={textAreaRef}
                multiline={true}
                keyboardType="default"
                disabled={Boolean(loggedOut)}
                rows={1}
                onPress={() => {
                  if (!chatWarned) {
                    dispatch(chatWarn(true));
                    toast.show("Just so you know!", {
                      message: `Streamplace chat messages are public in the same way that Bluesky posts are public - they create records on your PDS.`,
                    });
                  }
                }}
                onChangeText={(text) => {
                  const newMessage = text.replaceAll("\n", "");
                  // const rt = new RichText({ text: newMessage });
                  // rt.detectFacetsWithoutResolution();
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
                    e.preventDefault();
                    submit();
                  }
                }}
                onSubmitEditing={submit}
              />
            </View>
          )}
          <View
            flexDirection="row"
            justifyContent="flex-end"
            flexGrow={1}
            flexShrink={0}
            gap="$2"
          >
            {isChatVisible && (
              <>
                <NameColorPicker
                  buttonProps={{ backgroundColor: "transparent" }}
                  text={(color) => <Palette size={16} color={color} />}
                />
                <PopoutButton
                  livestream={livestream}
                  isPopout={isPopout}
                  setIsChatVisible={setIsChatVisible}
                />
                <Button
                  flexShrink={0}
                  backgroundColor="transparent"
                  disabled={loggedOut}
                  onPress={() => {
                    submit();
                  }}
                >
                  Send
                </Button>
              </>
            )}
          </View>
        </Form>
      )}
    </View>
  );
}

const PopoutButton = ({
  livestream,
  isPopout,
  setIsChatVisible,
}: {
  livestream: LivestreamViewHydrated | null;
  isPopout?: boolean;
  setIsChatVisible?: (visible: boolean) => void;
}) => {
  if (!isWeb || isPopout) {
    return <></>;
  }
  return (
    <Button
      flexShrink={0}
      backgroundColor="transparent"
      onPress={() => {
        const u = new URL(window.location.href);
        u.pathname = `/chat-popout/${livestream?.author?.did}`;
        window.open(u.toString(), "_blank", "popup=true");
        setIsChatVisible?.(false);
      }}
    >
      <SquareArrowOutUpRight size={16} />
    </Button>
  );
};
