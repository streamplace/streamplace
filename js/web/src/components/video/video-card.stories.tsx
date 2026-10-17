import { withAllProviders } from "@/../.storybook/decorators";
import type { Meta, StoryObj } from "@storybook/react-vite";
import type { VideoView } from "../../hooks/use-video-list";
import { VideoCard } from "./video-card";

const meta: Meta<typeof VideoCard> = {
  component: VideoCard,
  parameters: {
    layout: "fullscreen",
    backgrounds: { default: "dark" },
  },
  decorators: [
    withAllProviders,
    (Story) => (
      <div className="flex h-svh w-full items-center justify-center bg-[#0c0a14] p-4">
        <div className="w-full max-w-sm">
          <Story />
        </div>
      </div>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof VideoCard>;

function makeVideo(overrides: Partial<VideoView> = {}): VideoView {
  return {
    uri: "at://did:plc:creator/place.stream.video/3jx7abc123",
    author: {
      did: "did:plc:creator",
      handle: "creator.bsky.social",
    },
    record: {
      $type: "place.stream.video",
      title: "My latest video",
      createdAt: "2024-06-01T00:00:00.000Z",
      durationMs: 345000,
      source: {
        $type: "place.stream.media.defs#sourceTracks",
        tracks: [],
      },
    },
    viewCounts: { count: 1234 },
    likeCount: 56,
    ...overrides,
  } as VideoView;
}

export const Default: Story = {
  args: {
    video: makeVideo(),
    avatarUrl: "https://placehold.co/64x64/333/fff?text=C",
  },
};

export const NoAvatar: Story = {
  args: {
    video: makeVideo(),
  },
};

export const LongTitle: Story = {
  args: {
    video: makeVideo({
      record: {
        $type: "place.stream.video",
        title:
          "Building a Full Stack App from Scratch with React, Go, and PostgreSQL in 2024",
        createdAt: "2024-06-01T00:00:00.000Z",
        durationMs: 7200000,
        source: {
          $type: "place.stream.media.defs#sourceTracks",
          tracks: [],
        },
      } as any,
    }),
    avatarUrl: "https://placehold.co/64x64/333/fff?text=C",
  },
};

export const NoViews: Story = {
  args: {
    video: makeVideo({ viewCounts: undefined, likeCount: 0 }),
    avatarUrl: "https://placehold.co/64x64/333/fff?text=C",
  },
};

export const LongDuration: Story = {
  args: {
    video: makeVideo({
      record: {
        $type: "place.stream.video",
        title: "2 hour livestream archive",
        createdAt: "2024-06-01T00:00:00.000Z",
        durationMs: 7380000,
        source: {
          $type: "place.stream.media.defs#sourceTracks",
          tracks: [],
        },
      } as any,
    }),
    avatarUrl: "https://placehold.co/64x64/333/fff?text=C",
  },
};
