import { makeLivestreamStore, registerSlashCommand } from "@streamplace/core";
import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { ChatInput } from "./chat-input";

const { mockSend, mockText, mockUseSession, mockClearContent } = vi.hoisted(
  () => ({
    mockSend: vi.fn(),
    mockText: { current: "" },
    mockUseSession: vi.fn(),
    mockClearContent: vi.fn(),
  }),
);

vi.mock("@/lib/session", () => ({ useSession: mockUseSession }));
vi.mock("../../hooks/use-chat-send", () => ({ useChatSend: () => mockSend }));
vi.mock("../../hooks/use-skin-tone", () => ({
  skinToneIndex: () => 0,
  useSkinTone: () => [0, vi.fn()],
}));
vi.mock("../../lib/emoji-data", () => ({
  getEmojiData: vi.fn(),
  getSkinNative: vi.fn(),
  searchEmojis: vi.fn(),
  useEmojiData: () => null,
}));
vi.mock("react-i18next", () => ({
  useTranslation: () => ({ t: (key: string) => key }),
}));
vi.mock("@tiptap/react", () => ({
  EditorContent: () => (
    <textarea
      aria-label="message"
      onChange={(event) => {
        mockText.current = event.target.value;
      }}
    />
  ),
  ReactRenderer: class {},
  useEditor: () => ({
    getText: () => mockText.current,
    commands: {
      clearContent: () => {
        mockClearContent();
        mockText.current = "";
      },
    },
    chain: () => ({
      focus: () => ({
        insertContent: () => ({ run: vi.fn() }),
      }),
    }),
  }),
}));

describe("ChatInput slash commands", () => {
  let container: HTMLDivElement;
  let root: Root;

  beforeEach(() => {
    vi.stubGlobal("IS_REACT_ACT_ENVIRONMENT", true);
    mockText.current = "";
    mockSend.mockReset();
    mockClearContent.mockReset();
    mockUseSession.mockReturnValue({
      state: { status: "authenticated" },
      pdsAgent: null,
      did: null,
    });
    container = document.createElement("div");
    document.body.appendChild(container);
    root = createRoot(container);
  });

  afterEach(async () => {
    await act(async () => root.unmount());
    container.remove();
    mockUseSession.mockReset();
    vi.unstubAllGlobals();
  });

  it("preserves the draft and reply when a slash command fails", async () => {
    const store = makeLivestreamStore();
    const reply = {
      author: { did: "did:plc:author", handle: "author.example" },
      record: { text: "original message" },
    } as never;
    store.setState({ replyToMessage: reply });
    const unregister = registerSlashCommand({
      name: "teleport",
      description: "Start a teleport",
      usage: "/teleport @handle",
      handler: async () => ({
        handled: true,
        error: "Only the streamer can teleport",
      }),
    });

    await act(async () => root.render(<ChatInput store={store} />));
    const editor = container.querySelector("textarea");
    expect(editor).not.toBeNull();

    await act(async () => {
      const input = editor!;
      Object.getOwnPropertyDescriptor(
        HTMLTextAreaElement.prototype,
        "value",
      )?.set?.call(input, "/teleport");
      input.dispatchEvent(new Event("input", { bubbles: true }));
      input.dispatchEvent(new Event("change", { bubbles: true }));
    });
    await act(async () => {
      container
        .querySelector("form")
        ?.dispatchEvent(
          new Event("submit", { bubbles: true, cancelable: true }),
        );
      await Promise.resolve();
    });

    expect(mockText.current).toBe("/teleport");
    expect(mockClearContent).not.toHaveBeenCalled();
    expect(store.getState().replyToMessage).toBe(reply);
    expect(mockSend).not.toHaveBeenCalled();
    expect(container.querySelector('[role="alert"]')?.textContent).toBe(
      "Only the streamer can teleport",
    );
    unregister();
  });
});
