import { withFullscreen } from "@/../.storybook/decorators";
import type { Meta, StoryObj } from "@storybook/react-vite";
import { Player } from "./player";

const meta: Meta<typeof Player> = {
  component: Player,
  parameters: {
    layout: "fullscreen",
    backgrounds: { default: "dark" },
  },
  decorators: [
    withFullscreen,
    (Story) => (
      <div className="flex h-svh w-full items-center justify-center bg-black">
        <div className="aspect-video w-full max-w-4xl">
          <Story />
        </div>
      </div>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof Player>;

/**
 * Inactive player; shows the poster image only, no video backend.
 */
export const Inactive: Story = {
  args: {
    src: "",
    active: false,
    poster: "https://placehold.co/1280x720/000000/666666?text=Streamplace",
    mode: "live",
  },
};

/**
 * Active player with a test HLS stream. The player will attempt to
 * load and play automatically.
 */
export const LiveHLS: Story = {
  args: {
    src: "https://kiryu.cloud/aipc/main.m3u8",
    active: true,
    mode: "vod",
  },
};

/**
 * VOD mode; same backend but the chrome shows a scrubber instead of
 * the "LIVE" indicator.
 */
export const VOD: Story = {
  args: {
    src: "https://kiryu.cloud/aipc/main.m3u8",
    active: true,
    mode: "vod",
  },
};

/**
 * Inactive with a fallback poster; shown when the stream is offline.
 */
export const Offline: Story = {
  args: {
    src: "natalie.sh",
    active: false,
    poster: "https://placehold.co/1280x720/1a1a2e/e94560?text=Offline",
    fallbackPoster:
      "https://placehold.co/1280x720/0f0f1a/e94560?text=Stream+Ended",
    mode: "live",
  },
};
