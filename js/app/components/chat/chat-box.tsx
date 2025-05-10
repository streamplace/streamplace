import { useNavigation } from "@react-navigation/native";
import { useToastController } from "@tamagui/toast";
import {
  chatMessage,
  selectIsReady,
  selectUserProfile,
  selectChatProfile,
} from "features/bluesky/blueskySlice";
import {
  LivestreamViewHydrated,
  usePlayerLivestream,
  addLocalChatMessage,
  usePlayerId,
  useReplyToMessage,
  usePlayerActions,
  useChat,
  MessageViewHydrated,
} from "features/player/playerSlice";
import {
  chatWarn,
  selectChatWarned,
} from "features/streamplace/streamplaceSlice";
import { useRef, useState, useEffect } from "react";
import { Keyboard } from "react-native";
import { useAppDispatch, useAppSelector } from "store/hooks";
import { Button, Form, Input, isWeb, TextArea, View, Text } from "tamagui";
import {
  Palette,
  SquareArrowOutUpRight,
  X as XIcon,
} from "@tamagui/lucide-icons";
import NameColorPicker from "components/name-color-picker/name-color-picker";
import MentionSuggestions, { MentionSuggestion } from "./mention-suggestions";

const getParticipantSuggestions = (
  chat: MessageViewHydrated[],
  currentUserDid?: string,
) => {
  const participants = new Set<string>();
  chat.forEach((message) => {
    if (message.author.handle && message.author.did !== currentUserDid) {
      participants.add(message.author.handle);
    }
  });

  return Array.from(participants).map((handle) => {
    const message = chat.find((m) => m.author.handle === handle);
    return {
      did: message?.author.did || "",
      handle,
      color: message?.chatProfile?.color,
    };
  });
};

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
  const [showSuggestions, setShowSuggestions] = useState(false);
  const [suggestions, setSuggestions] = useState<MentionSuggestion[]>([]);
  const [lastAtPosition, setLastAtPosition] = useState(-1);
  const [highlightedIndex, setHighlightedIndex] = useState(0);
  const isReady = useAppSelector(selectIsReady);
  const userProfile = useAppSelector(selectUserProfile);
  const chatProfile = useAppSelector(selectChatProfile);
  const chatWarned = useAppSelector(selectChatWarned);
  const loggedOut = isReady && !userProfile;
  const livestream = useAppSelector(usePlayerLivestream());
  const chat = useAppSelector(useChat());
  const textAreaRef = useRef<Input>(null);
  const dispatch = useAppDispatch();
  const navigate = useNavigation();
  const playerId = usePlayerId();
  const playerActions = usePlayerActions();
  const replyTo = useAppSelector(useReplyToMessage());
  const [isTextAreaFocused, setIsTextAreaFocused] = useState(false);
  const enterHandledRef = useRef(false);
  const [cursorPosition, setCursorPosition] = useState(0);

  useEffect(() => {
    if (replyTo && textAreaRef.current) {
      textAreaRef.current.focus();
    }
  }, [replyTo]);

  useEffect(() => {
    if (!chat) return;

    const allSuggestions = getParticipantSuggestions(chat, userProfile?.did);
    setSuggestions(allSuggestions);
  }, [chat, userProfile?.did]);

  useEffect(() => {
    if (!showSuggestions) {
      setHighlightedIndex(0);
    }
  }, [showSuggestions]);

  const updateSuggestions = (text: string, cursorPosition: number) => {
    const atIndex = text.lastIndexOf("@", cursorPosition);

    if (atIndex === -1 || !chat) {
      setShowSuggestions(false);
      return;
    }

    let allSuggestions = getParticipantSuggestions(chat, userProfile?.did);

    // Filter suggestions based on input after @
    const searchText = text.slice(atIndex + 1, cursorPosition).toLowerCase();
    if (searchText) {
      allSuggestions = allSuggestions.filter((suggestion) =>
        suggestion.handle.toLowerCase().includes(searchText),
      );
    }

    setSuggestions(allSuggestions);
    setLastAtPosition(atIndex);
    setShowSuggestions(true);
  };

  const handleMentionSelect = (suggestion: MentionSuggestion) => {
    if (lastAtPosition === -1) return;

    const beforeAt = message.slice(0, lastAtPosition);
    const afterAt = message.slice(lastAtPosition);
    const wordEndIndex = afterAt.search(/\s|$/);
    const afterWord = afterAt.slice(wordEndIndex);

    setMessage(`${beforeAt}@${suggestion.handle}${afterWord} `);
    setShowSuggestions(false);
  };

  const handleMentionSelectByIndex = (index: number) => {
    if (suggestions.length > 0) {
      handleMentionSelect(suggestions[index]);
    }
  };

  const submit = () => {
    if (!isWeb) Keyboard.dismiss();
    if (!message.length || !livestream || !userProfile) return;

    // Add local message
    dispatch(
      addLocalChatMessage({
        playerId,
        message,
        ...(replyTo ? { replyTo } : {}),
        author: {
          did: userProfile.did,
          handle: userProfile.handle,
        },
        chatProfile: chatProfile?.profile?.color
          ? {
              color: {
                red: chatProfile.profile.color.red,
                green: chatProfile.profile.color.green,
                blue: chatProfile.profile.color.blue,
              },
            }
          : undefined,
      }),
    );

    // Send to server
    dispatch(
      chatMessage({
        text: message,
        livestream,
        ...(replyTo ? { replyTo } : {}),
      }),
    );

    setMessage("");
    playerActions.setReplyToMessage(null);
    setShowSuggestions(false);
    if (isWeb && textAreaRef.current) {
      const textarea = textAreaRef.current as unknown as HTMLTextAreaElement;
      textarea.style.height = "";
      textarea.focus();
    }
  };

  const toast = useToastController();

  useEffect(() => {
    if (!isWeb || !textAreaRef.current) return;
    const textarea = textAreaRef.current as unknown as HTMLTextAreaElement;
    const handleKeyDown = (e: KeyboardEvent) => {
      if (!isTextAreaFocused) return;
      if (showSuggestions && suggestions.length > 0) {
        if (e.key === "ArrowDown") {
          e.preventDefault();
          setHighlightedIndex((prev) => (prev + 1) % suggestions.length);
        } else if (e.key === "ArrowUp") {
          e.preventDefault();
          setHighlightedIndex(
            (prev) => (prev - 1 + suggestions.length) % suggestions.length,
          );
        } else if (e.key === "Tab" || e.key === "Enter") {
          e.preventDefault();
          handleMentionSelectByIndex(highlightedIndex);
          enterHandledRef.current = true;
        } else if (e.key === "Escape") {
          setShowSuggestions(false);
        }
      }
    };
    textarea.addEventListener("keydown", handleKeyDown);
    return () => {
      textarea.removeEventListener("keydown", handleKeyDown);
    };
  }, [isTextAreaFocused, showSuggestions, suggestions, highlightedIndex]);

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
          position="relative"
        >
          <View flexDirection="column" gap="$2">
            {replyTo && (
              <View
                flexDirection="row"
                alignItems="flex-start"
                gap="$2"
                padding="$2"
                backgroundColor="$backgroundHover"
                borderRadius="$2"
              >
                <View flex={1}>
                  <Text fontSize={12} color="$color">
                    Replying to{" "}
                    <Text
                      fontSize={12}
                      color={
                        replyTo.chatProfile?.color
                          ? `rgb(${replyTo.chatProfile.color.red}, ${replyTo.chatProfile.color.green}, ${replyTo.chatProfile.color.blue})`
                          : "$accentColor"
                      }
                      fontWeight="bold"
                    >
                      @{replyTo.author.handle}
                    </Text>
                  </Text>
                  <Text
                    fontSize={12}
                    color="$color"
                    opacity={0.7}
                    numberOfLines={1}
                  >
                    {replyTo.record.text}
                  </Text>
                </View>
                <Button
                  size="$2"
                  circular
                  onPress={() => playerActions.setReplyToMessage(null)}
                  backgroundColor="transparent"
                >
                  <XIcon size={16} />
                </Button>
              </View>
            )}
            <View flexDirection="row" gap="$2" position="relative">
              <View flexGrow={1} flexShrink={0} position="relative">
                <TextArea
                  borderRadius={0}
                  overflow="hidden"
                  returnKeyType="done"
                  submitBehavior="blurAndSubmit"
                  value={message}
                  ref={textAreaRef}
                  multiline={true}
                  keyboardType="default"
                  disabled={loggedOut}
                  rows={1}
                  onFocus={() => setIsTextAreaFocused(true)}
                  onBlur={() => setIsTextAreaFocused(false)}
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
                    if (newMessage.length > 300) {
                      return;
                    }
                    setMessage(text.replaceAll("\n", ""));
                    if (isWeb && textAreaRef.current) {
                      const textarea =
                        textAreaRef.current as unknown as HTMLTextAreaElement;
                      textarea.style.height = "";
                      textarea.style.height = textarea.scrollHeight + "px";

                      // Update suggestions based on cursor position
                      const cursorPosition = textarea.selectionStart;
                      updateSuggestions(text, cursorPosition);
                    } else {
                      // On native, use tracked cursor position
                      updateSuggestions(text, cursorPosition);
                    }
                  }}
                  onSelectionChange={(e) => {
                    // For native platforms, track cursor position
                    if (!isWeb) {
                      setCursorPosition(e.nativeEvent.selection.start);
                    }
                  }}
                  onKeyPress={(e) => {
                    if (enterHandledRef.current) {
                      enterHandledRef.current = false;
                      e.preventDefault();
                      return;
                    }
                    if (
                      showSuggestions &&
                      suggestions.length > 0 &&
                      (e.nativeEvent.key === "Enter" ||
                        e.nativeEvent.key === "Tab")
                    ) {
                      e.preventDefault();
                      return;
                    }
                    if (e.nativeEvent.key === "Enter") {
                      e.preventDefault();
                      submit();
                    } else if (
                      e.nativeEvent.key === "Tab" &&
                      showSuggestions &&
                      suggestions.length > 0
                    ) {
                      e.preventDefault();
                      handleMentionSelect(suggestions[0]);
                    }
                  }}
                  onSubmitEditing={submit}
                />
                {showSuggestions && suggestions.length > 0 && (
                  <View
                    position="absolute"
                    top="100%"
                    left={0}
                    right={0}
                    pointerEvents="box-none"
                    style={{
                      zIndex: 100000,
                    }}
                  >
                    <MentionSuggestions
                      suggestions={suggestions}
                      onSelect={handleMentionSelect}
                      highlightedIndex={highlightedIndex}
                      setHighlightedIndex={setHighlightedIndex}
                    />
                  </View>
                )}
              </View>
            </View>
          </View>
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
