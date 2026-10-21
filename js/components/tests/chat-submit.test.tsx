import {
  act,
  createElement,
  useImperativeHandle,
  useRef,
  useSyncExternalStore,
  type ReactNode,
  type Ref,
} from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { ChatBox } from "../src/components/chat/chat-box";

interface NativeInput {
  clear(): void;
  focus(): void;
  blur(): void;
  isFocused(): boolean;
}

interface ComposerInput {
  ref: Ref<NativeInput>;
  testID: string;
  value: string;
  onChangeText(text: string): void;
  onKeyPress?(event: {
    nativeEvent: { key: string };
    preventDefault(): void;
  }): void;
  onSubmitEditing(event: {
    nativeEvent: { text: string };
    preventDefault(): void;
  }): void;
  onEndEditing(event: { nativeEvent: { text: string } }): void;
}

const fixture = vi.hoisted(() => ({
  platform: "ios",
  draft: "",
  focusHistory: ["chat-input"] as Array<string | null>,
  listeners: new Set<() => void>(),
  input: {} as ComposerInput,
  send: vi.fn(),
}));

function setDraft(text: string) {
  fixture.draft = text;
  fixture.listeners.forEach((listener) => listener());
}
function View({ children }: { children?: ReactNode }) {
  return createElement("div", null, children);
}
vi.mock("react-native", () => ({
  Platform: {
    get OS() {
      return fixture.platform;
    },
  },
  Pressable: View,
  StyleSheet: { create: (styles: unknown) => styles },
}));
vi.mock("@streamplace/core", () => ({
  createTeleportCommands: () => [],
  handleSlashCommand: async () => ({ handled: false }),
}));
vi.mock("lucide-react-native", () => ({
  AtSignIcon: View,
  ExternalLink: View,
  X: View,
}));
vi.mock("../src/index", () => ({
  View,
  Text: View,
  Button: ({
    children,
    onPress,
    disabled,
  }: {
    children?: ReactNode;
    onPress(): void;
    disabled?: boolean;
  }) => createElement("button", { onClick: onPress, disabled }, children),
  Loader: () => "Sending",
  toast: { show: vi.fn() },
  useTheme: () => ({
    theme: { colors: {}, borderRadius: {} },
    zero: {},
  }),
}));
vi.mock("../src/livestream-store", () => ({
  useChatDraft: () =>
    useSyncExternalStore(
      (listener) => {
        fixture.listeners.add(listener);
        return () => fixture.listeners.delete(listener);
      },
      () => fixture.draft,
    ),
  useSetChatDraft: () => setDraft,
  useChat: () => [],
  useCreateChatMessage: () => fixture.send,
  useAddSystemMessage: () => vi.fn(),
  useLivestream: () => null,
  useLivestreamStore: () => vi.fn(),
  useProfile: () => null,
  useReplyToMessage: () => null,
  useSetReplyToMessage: () => vi.fn(),
}));
vi.mock("../src/streamplace-store", () => ({
  useDID: () => null,
  usePDSAgent: () => null,
}));
vi.mock("../src/streamplace-store/branding", () => ({
  useNetworkName: () => "Streamplace",
}));
vi.mock("../src/lib/stream-notifications", () => ({ StreamNotifications: {} }));
vi.mock("../src/components/chat/chat-message", () => ({
  RenderChatMessage: View,
}));
vi.mock("../src/components/chat/emoji-suggestions", () => ({
  EmojiSuggestions: View,
}));
vi.mock("../src/components/chat/mention-suggestions", () => ({
  MentionSuggestions: View,
}));
vi.mock("../src/components/chat/teleport-modal", () => ({
  TeleportModal: () => null,
}));
vi.mock("../src/components/ui/textarea", () => ({
  Textarea: (props: ComposerInput) => {
    if (props.testID === "chat-input") fixture.input = props;
    const input = useRef<HTMLInputElement>(null);
    useImperativeHandle(props.ref, () => ({
      clear() {
        if (input.current) input.current.value = "";
      },
      focus() {
        fixture.focusHistory.push(props.testID);
      },
      blur() {
        fixture.focusHistory.push(null);
      },
      isFocused: () => fixture.focusHistory.at(-1) === props.testID,
    }));
    return createElement("input", {
      ref: input,
      "data-testid": props.testID,
      value: props.value,
      onChange: (event: { target: { value: string } }) =>
        props.onChangeText(event.target.value),
    });
  },
}));

let container: HTMLDivElement;
let chatInput: HTMLInputElement;
let root: Root;
beforeEach(async () => {
  Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true });
  fixture.platform = "ios";
  fixture.draft = "hello teh";
  fixture.focusHistory = ["chat-input"];
  fixture.send.mockReset();
  container = document.createElement("div");
  document.body.append(container);
  root = createRoot(container);
  await act(async () => root.render(<ChatBox emojiData={null} />));
  chatInput = container.querySelector<HTMLInputElement>(
    '[data-testid="chat-input"]',
  )!;
});
afterEach(async () => {
  await act(async () => root.unmount());
  container.remove();
});

it.each([
  ["ios", "keyboard"],
  ["ios", "button twice"],
  ["android", "keyboard"],
])(
  "%s %s sends final native text once, clears, and keeps the next draft",
  async (platform, control) => {
    fixture.platform = platform;
    await act(async () => {
      if (control === "button twice") {
        container.querySelector("button")!.click();
        // The first handoff is requested, but onEndEditing is still pending.
        container.querySelector("button")!.click();
      } else {
        fixture.input.onKeyPress?.({
          nativeEvent: { key: "Enter" },
          preventDefault() {},
        });
        fixture.input.onSubmitEditing({
          nativeEvent: { text: platform === "ios" ? "hello teh" : "hello the" },
          preventDefault() {},
        });
      }
    });
    if (platform === "ios") {
      expect(fixture.send).not.toHaveBeenCalled();
      // Switching responders must never leave the keyboard with no input.
      expect(fixture.focusHistory).not.toContain(null);
      expect(fixture.focusHistory.at(-1)).toBe("chat-input");
      // UIKit commits the correction while ending editing, after the Send
      // gesture. The React draft can still be stale when this event arrives.
      await act(async () => {
        fixture.input.onChangeText("hello the");
        fixture.input.onEndEditing({ nativeEvent: { text: "hello the" } });
      });
      expect(fixture.focusHistory.at(-1)).toBe("chat-input");
    }
    expect(fixture.send.mock.calls).toEqual([
      [{ text: "hello the", reply: undefined }],
    ]);
    expect(chatInput.value).toBe("");
    await act(async () => fixture.input.onChangeText("next message"));
    expect(chatInput.value).toBe("next message");
    // A later ordinary blur must neither resend nor clear the new draft.
    await act(async () =>
      fixture.input.onEndEditing({
        nativeEvent: { text: "next message" },
      }),
    );
    expect(fixture.send).toHaveBeenCalledTimes(1);
    expect(chatInput.value).toBe("next message");
    if (control === "button twice") {
      await act(async () => container.querySelector("button")!.click());
      expect(fixture.send).toHaveBeenCalledTimes(1);
      await act(async () =>
        fixture.input.onEndEditing({
          nativeEvent: { text: "next message" },
        }),
      );
      expect(fixture.send.mock.calls).toEqual([
        [{ text: "hello the", reply: undefined }],
        [{ text: "next message", reply: undefined }],
      ]);
      expect(chatInput.value).toBe("");
    }
    expect(container.querySelector('[data-testid="chat-input"]')).toBe(
      chatInput,
    );
    expect(fixture.focusHistory).not.toContain(null);
  },
);

it("validates the submitted native text rather than the stale draft", async () => {
  await act(async () => {
    fixture.input.onSubmitEditing({
      nativeEvent: { text: "hello teh" },
      preventDefault() {},
    });
    fixture.input.onEndEditing({
      nativeEvent: { text: "x".repeat(301) },
    });
  });
  expect(fixture.send).not.toHaveBeenCalled();
  expect(chatInput.value).toBe("hello teh");
});
