import { withAllProviders } from "@/../.storybook/decorators";
import type { Meta, StoryObj } from "@storybook/react-vite";
import type { PlaceStreamLivestream } from "streamplace";
import { StreamCard } from "./stream-card";

const meta: Meta<typeof StreamCard> = {
  component: StreamCard,
  parameters: {
    layout: "centered",
    backgrounds: { default: "dark" },
  },
  decorators: [
    withAllProviders,
    (Story) => (
      <div className="w-full max-w-sm">
        <Story />
      </div>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof StreamCard>;

function makeStream(
  overrides: Partial<PlaceStreamLivestream.LivestreamView> = {},
): PlaceStreamLivestream.LivestreamView {
  return {
    uri: "at://did:plc:streamer/place.stream.livestream/abc",
    author: {
      did: "did:plc:streamer",
      handle: "streamer.bsky.social",
    },
    record: {
      $type: "place.stream.livestream",
      title: "Building something cool",
      createdAt: "2024-01-01T00:00:00.000Z",
    },
    viewerCount: { count: 42 },
    ...overrides,
  } as PlaceStreamLivestream.LivestreamView;
}

export const Default: Story = {
  args: {
    stream: makeStream(),
    avatarUrl: "https://placehold.co/64x64/333/fff?text=S",
  },
};

export const NoAvatar: Story = {
  args: {
    stream: makeStream(),
  },
};

export const NoViewers: Story = {
  args: {
    stream: makeStream({ viewerCount: undefined }),
    avatarUrl: "https://placehold.co/64x64/333/fff?text=S",
  },
};

export const HighViewerCount: Story = {
  args: {
    stream: makeStream({ viewerCount: { count: 15400 } }),
    avatarUrl: "https://placehold.co/64x64/333/fff?text=S",
  },
};

export const WithTags: Story = {
  args: {
    stream: makeStream({
      record: {
        $type: "place.stream.livestream",
        title: "Coding a Rust project",
        createdAt: "2024-01-01T00:00:00.000Z",
        tags: ["rust", "backend", "lang:en"],
        activity: {
          $type: "place.stream.defs#activityLabel",
          label: "software_dev",
        },
      } as any,
    }),
    avatarUrl: "https://placehold.co/64x64/333/fff?text=S",
  },
};

export const WithGameActivity: Story = {
  args: {
    stream: makeStream({
      record: {
        $type: "place.stream.livestream",
        title: "Speedrunning Hollow Knight",
        createdAt: "2024-01-01T00:00:00.000Z",
        activity: {
          $type: "place.stream.defs#activityGame",
          name: "Hollow Knight",
        },
      } as any,
    }),
    avatarUrl: "https://placehold.co/64x64/333/fff?text=S",
  },
};

export const ManyTags: Story = {
  args: {
    stream: makeStream({
      record: {
        $type: "place.stream.livestream",
        title: "Variety stream",
        createdAt: "2024-01-01T00:00:00.000Z",
        tags: [
          "variety",
          "chatting",
          "fun",
          "english",
          "community",
          "music",
          "art",
        ],
        activity: {
          $type: "place.stream.defs#activityLabel",
          label: "just_chatting",
        },
      } as any,
    }),
    avatarUrl: "https://placehold.co/64x64/333/fff?text=S",
  },
};

export const LongTitle: Story = {
  args: {
    stream: makeStream({
      record: {
        $type: "place.stream.livestream",
        title:
          "This is a very long stream title that should truncate when displayed in the card",
        createdAt: "2024-01-01T00:00:00.000Z",
      } as any,
    }),
    avatarUrl: "https://placehold.co/64x64/333/fff?text=S",
  },
};
