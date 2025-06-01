import { useNavigation } from "@react-navigation/native";
import {
  Palette,
  SquareArrowOutUpRight,
  X as XIcon,
} from "@tamagui/lucide-icons";
import { useToastController } from "@tamagui/toast";
import { emojiEmitter } from "components/emoji-picker/emoji-emitter";
import { EmojiPicker } from "components/emoji-picker/emoji-picker";
import NameColorPicker from "components/name-color-picker/name-color-picker";
import {
  chatMessage,
  selectChatProfile,
  selectIsReady,
  selectUserProfile,
} from "features/bluesky/blueskySlice";
import {
  addLocalChatMessage,
  LivestreamViewHydrated,
  MessageViewHydrated,
  useChat,
  usePlayerActions,
  usePlayerId,
  usePlayerLivestream,
  useReplyToMessage,
} from "features/player/playerSlice";
import {
  chatWarn,
  selectChatWarned,
} from "features/streamplace/streamplaceSlice";
import { usePreloadEmoji } from "hooks/usePreloadEmoji";
import { useEffect, useRef, useState } from "react";
import { Keyboard } from "react-native";
import { useAppDispatch, useAppSelector } from "store/hooks";
import { Button, Form, Input, isWeb, Text, TextArea, View } from "tamagui";
import EmojiSuggestions, { EmojiSuggestion } from "./emoji-suggestions";
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
  if (isWeb) usePreloadEmoji({ immediate: true });
  const [isTextAreaFocused, setIsTextAreaFocused] = useState(false);
  const [pickerState, setPickerState] = useState({
    isOpen: false,
    position: { top: 0, left: 0 },
  });
  const [emojiSuggestions, setEmojiSuggestions] = useState<EmojiSuggestion[]>(
    [],
  );
  const [showEmojiSuggestions, setShowEmojiSuggestions] = useState(false);
  const [emojiQuery, setEmojiQuery] = useState("");
  const [emojiHighlightedIndex, setEmojiHighlightedIndex] = useState(0);
  const [lastColonPosition, setLastColonPosition] = useState(-1);

  const emojiList = useRef<EmojiSuggestion[]>([]);
  const [emojiDataLoaded, setEmojiDataLoaded] = useState(false);
  useEffect(() => {
    if (emojiList.current.length === 0) {
      (async () => {
        let emojiDataRaw;
        if (isWeb) {
          emojiDataRaw = (await import("../../assets/emoji-data.json")).default;
        } else {
          emojiDataRaw = require("../../assets/emoji-data.json");
        }
        const emojis = emojiDataRaw.emojis;
        emojiList.current = Object.keys(emojis).map((id) => {
          const e = emojis[id];
          return {
            emoji: e.skins[0].native,
            shortcode: `:${id}:`,
            name: e.name,
          };
        });
        setEmojiDataLoaded(true);
      })();
    }
  }, []);

  function getEmojiQuery(text: string, cursor: number) {
    const match = /(^|\s):([a-zA-Z0-9_+\-]*)$/;
    const before = text.slice(0, cursor);
    const m = before.match(match);
    if (m) {
      return { query: m[2], start: cursor - m[2].length - 1 };
    }
    return null;
  }

  const updateEmojiSuggestions = (text: string, cursor: number) => {
    if (!emojiDataLoaded) return;
    const result = getEmojiQuery(text, cursor);
    if (result && result.query.length > 0) {
      const exact = emojiList.current.find(
        (e) => e.shortcode === `:${result.query}:`,
      );
      let filtered = emojiList.current.filter((e) =>
        e.shortcode.startsWith(`:${result.query}`),
      );
      if (exact) {
        filtered = [
          exact,
          ...filtered.filter((e) => e.shortcode !== exact.shortcode),
        ];
      }
      setEmojiSuggestions(filtered.slice(0, 5));
      setShowEmojiSuggestions(filtered.length > 0);
      setEmojiQuery(result.query);
      setLastColonPosition(result.start);
      setEmojiHighlightedIndex(0);
    } else {
      setShowEmojiSuggestions(false);
      setEmojiSuggestions([]);
      setEmojiQuery("");
      setLastColonPosition(-1);
    }
  };

  function getSelectionStart() {
    if (isWeb && textAreaRef.current) {
      const textarea = textAreaRef.current as unknown as HTMLTextAreaElement;
      return textarea.selectionStart || 0;
    }
    return 0;
  }

  const handleEmojiSelect = (suggestion: EmojiSuggestion) => {
    if (lastColonPosition === -1) return;
    if (isWeb && textAreaRef.current) {
      const before = message.slice(0, lastColonPosition);
      const after = message.slice(getSelectionStart());
      setMessage(before + suggestion.emoji + " " + after);
    } else {
      let endOfTrigger = lastColonPosition;
      if (emojiQuery) {
        const regex = new RegExp(`:${emojiQuery}\b:?`);
        const match = regex.exec(message.slice(lastColonPosition));
        if (match) {
          endOfTrigger = lastColonPosition + match[0].length;
        } else {
          endOfTrigger = lastColonPosition + emojiQuery.length + 1;
        }
      }
      const before = message.slice(0, lastColonPosition);
      const after = message.slice(endOfTrigger);
      setMessage(before + suggestion.emoji + " " + after);
    }
    setShowEmojiSuggestions(false);
    setEmojiSuggestions([]);
    setEmojiQuery("");
    setLastColonPosition(-1);
    setEmojiHighlightedIndex(0);
    setTimeout(() => textAreaRef.current?.focus(), 0);
  };

  useEffect(() => {
    if (!showEmojiSuggestions && emojiQuery) {
      if (!emojiDataLoaded) return;
      const valid = emojiList.current.find(
        (e) => e.shortcode === `:${emojiQuery}:`,
      );
      if (valid && lastColonPosition !== -1) {
        const cursor = getSelectionStart();
        const afterColon = message.slice(lastColonPosition, cursor);
        if (afterColon === `:${emojiQuery}:`) {
          const before = message.slice(0, lastColonPosition);
          const after = message.slice(cursor);
          setMessage(before + valid.emoji + after);
          setEmojiQuery("");
          setLastColonPosition(-1);
        }
      }
    }
  }, [showEmojiSuggestions]);

  useEffect(() => {
    if (!isWeb || !textAreaRef.current) return;
    const textarea = textAreaRef.current as unknown as HTMLTextAreaElement;
    const handleKeyDown = (e: KeyboardEvent) => {
      if (!isTextAreaFocused) return;
      if (showEmojiSuggestions && emojiSuggestions.length > 0) {
        if (e.key === "ArrowDown") {
          e.preventDefault();
          setEmojiHighlightedIndex(
            (prev) => (prev + 1) % emojiSuggestions.length,
          );
        } else if (e.key === "ArrowUp") {
          e.preventDefault();
          setEmojiHighlightedIndex(
            (prev) =>
              (prev - 1 + emojiSuggestions.length) % emojiSuggestions.length,
          );
        } else if (e.key === "Tab" || e.key === "Enter") {
          e.preventDefault();
          handleEmojiSelect(emojiSuggestions[emojiHighlightedIndex]);
        } else if (e.key === "Escape") {
          setShowEmojiSuggestions(false);
        }
        return;
      } else if (showSuggestions && suggestions.length > 0) {
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
        } else if (e.key === "Escape") {
          setShowSuggestions(false);
        }
        return;
      }
      if (e.key === "Enter") {
        e.preventDefault();
        submit();
      }
    };
    textarea.addEventListener("keydown", handleKeyDown);
    return () => {
      textarea.removeEventListener("keydown", handleKeyDown);
    };
  }, [
    isTextAreaFocused,
    showSuggestions,
    suggestions,
    highlightedIndex,
    showEmojiSuggestions,
    emojiSuggestions,
    emojiHighlightedIndex,
  ]);

  useEffect(() => {
    const onEmojiSelected = (emoji: { native: string }) => {
      setMessage((prev) => prev + emoji.native);
    };

    emojiEmitter.addListener("emoji-selected", onEmojiSelected);
    return () => {
      emojiEmitter.removeListener("emoji-selected", onEmojiSelected);
    };
  }, []);

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

  const openEmojiPicker = () => {
    if (textAreaRef.current) {
      if (isWeb) {
        setPickerState({
          isOpen: true,
          position: {
            top: 0,
            left: 0,
          },
        });
      } else {
        textAreaRef.current.focus?.();
        setEmojiSuggestions(emojiList.current.slice(0, 5));
        setShowEmojiSuggestions(!showEmojiSuggestions);
        setEmojiQuery("");
        setLastColonPosition(message.length);
        setEmojiHighlightedIndex(0);
      }
    }
  };

  const openMentionSuggestions = () => {
    if (textAreaRef.current) {
      textAreaRef.current.focus?.();
      setSuggestions(getParticipantSuggestions(chat || [], userProfile?.did));
      setShowSuggestions(!showSuggestions);
      setHighlightedIndex(0);
      setLastAtPosition(message.length);
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
                    if (!emojiDataLoaded) return;
                    const newMessage = text.replaceAll("\n", "");
                    if (newMessage.length > 300) {
                      return;
                    }
                    let cursorPosition = 0;
                    if (isWeb && textAreaRef.current) {
                      const textarea =
                        textAreaRef.current as unknown as HTMLTextAreaElement;
                      cursorPosition = textarea.selectionStart;
                    } else {
                      cursorPosition = newMessage.length;
                    }
                    const match = /(^|\s):([a-zA-Z0-9_+\-]+):/g;
                    let replaced = false;
                    let updated = newMessage;
                    let offset = 0;
                    let m;
                    while ((m = match.exec(newMessage)) !== null) {
                      const shortcode = m[2];
                      const emojiObj = emojiList.current.find(
                        (e) => e.shortcode === `:${shortcode}:`,
                      );
                      if (emojiObj) {
                        // Only replace if the cursor is at or after the end of the shortcode
                        const end = m.index + m[0].length;
                        if (cursorPosition >= end) {
                          updated =
                            updated.slice(0, m.index + offset) +
                            emojiObj.emoji +
                            updated.slice(m.index + offset + m[0].length);
                          offset += emojiObj.emoji.length - m[0].length;
                          replaced = true;
                        }
                      }
                    }
                    setMessage(updated);
                    updateSuggestions(
                      updated,
                      cursorPosition + (updated.length - newMessage.length),
                    );
                    updateEmojiSuggestions(
                      updated,
                      cursorPosition + (updated.length - newMessage.length),
                    );
                  }}
                  onKeyPress={(e) => {
                    if (!isWeb) {
                      if (e.nativeEvent.key === "Enter") {
                        submit();
                      }
                    }
                  }}
                  onSubmitEditing={submit}
                />
                {showEmojiSuggestions && emojiSuggestions.length > 0 && (
                  <View
                    position="absolute"
                    top="100%"
                    left={0}
                    right={0}
                    pointerEvents="box-none"
                    style={{ zIndex: 100000 }}
                  >
                    <EmojiSuggestions
                      suggestions={emojiSuggestions}
                      onSelect={handleEmojiSelect}
                      highlightedIndex={emojiHighlightedIndex}
                      setHighlightedIndex={setEmojiHighlightedIndex}
                    />
                  </View>
                )}
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
                <Button
                  onPress={openMentionSuggestions}
                  backgroundColor="transparent"
                  flexShrink={0}
                >
                  @
                </Button>
                <View position="relative" flexShrink={0}>
                  <Button
                    onPress={openEmojiPicker}
                    backgroundColor="transparent"
                    flexShrink={0}
                  >
                    😊
                  </Button>
                  <EmojiPicker
                    isOpen={pickerState.isOpen}
                    onClose={() =>
                      setPickerState((prev) => ({ ...prev, isOpen: false }))
                    }
                  />
                </View>
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
