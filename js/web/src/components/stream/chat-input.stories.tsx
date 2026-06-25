import { withAllProviders } from "@/../.storybook/decorators";
import type { Meta, StoryObj } from "@storybook/react-vite";
import { makeLivestreamStore } from "@streamplace/core";
import { useEffect } from "react";
import { useStore } from "../../lib/store";
import { ChatInput } from "./chat-input";

// Mock a logged-in session so ChatInput shows the editor instead of
// the "log in to chat" prompt. We set the Zustand store state directly;
// SessionProvider reads from the same store.
const MOCK_DID = "did:plc:storybook-user";
const MOCK_HANDLE = "storybook.bsky.social";

function MockSession({ children }: { children: React.ReactNode }) {
  useEffect(() => {
    useStore.setState({
      authStatus: "loggedIn",
      oauthSession: {
        did: MOCK_DID,
        sub: MOCK_HANDLE,
      } as any,
      pdsAgent: {
        did: MOCK_DID,
        place: {
          stream: {
            live: { getRecommendations: async () => ({ data: {} }) },
          },
          server: {},
        },
        getProfile: async () => ({
          data: {
            did: MOCK_DID,
            handle: MOCK_HANDLE,
            displayName: "Storybook",
          },
        }),
        com: {
          atproto: {
            repo: {
              createRecord: async () => ({ success: true }),
            },
          },
        },
      } as any,
    });
  }, []);
  return <>{children}</>;
}

const meta: Meta<typeof ChatInput> = {
  component: ChatInput,
  parameters: {
    layout: "fullscreen",
    backgrounds: { default: "dark" },
  },
  decorators: [
    withAllProviders,
    (Story) => (
      <MockSession>
        <div className="flex h-svh w-full items-end justify-center bg-[#0c0a14] p-8">
          <div className="w-full max-w-md">
            <Story />
          </div>
        </div>
      </MockSession>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof ChatInput>;

export const Empty: Story = {
  render: () => {
    const store = makeLivestreamStore();
    return <ChatInput store={store} />;
  },
};

export const WithReply: Story = {
  render: () => {
    const store = makeLivestreamStore();
    // Set up a reply target so the reply banner shows.
    useEffect(() => {
      store.setState((s) => ({
        ...s,
        replyToMessage: {
          uri: "at://did:plc:someone/place.stream.chat.message/abc",
          cid: "cid-abc",
          author: {
            did: "did:plc:someone",
            handle: "someone.bsky.social",
          },
          record: {
            $type: "place.stream.chat.message",
            text: "hey everyone!",
            createdAt: new Date().toISOString(),
            streamer: "did:plc:streamer",
          },
          indexedAt: new Date().toISOString(),
          chatProfile: null,
        } as any,
      }));
    }, [store]);
    return <ChatInput store={store} />;
  },
};

export const LoggedOut: Story = {
  render: () => {
    const store = makeLivestreamStore();
    // Override the MockSession's auth state for this story.
    useEffect(() => {
      useStore.setState({ authStatus: "loggedOut", oauthSession: null });
    }, []);
    return <ChatInput store={store} />;
  },
};
