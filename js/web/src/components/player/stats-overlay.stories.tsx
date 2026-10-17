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

export const WithError: Story = {
  args: {
    src: "https://invalid.example.com/stream.m3u8",
    active: true,
    mode: "live",
  },
};
