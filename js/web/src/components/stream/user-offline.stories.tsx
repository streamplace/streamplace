import type { Meta, StoryObj } from "@storybook/react-vite";
import { UserOffline } from "./user-offline";

const meta: Meta<typeof UserOffline> = {
  component: UserOffline,
  parameters: {
    layout: "fullscreen",
    backgrounds: { default: "dark" },
  },
  decorators: [
    (Story) => (
      <div className="flex h-svh w-full items-center justify-center">
        <div className="relative h-64 w-full max-w-2xl overflow-hidden rounded-lg bg-black">
          <Story />
        </div>
      </div>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof UserOffline>;

export const Default: Story = {
  args: {
    user: "alice.bsky.social",
  },
};

export const LongHandle: Story = {
  args: {
    user: "very-long-handle-name.bsky.social",
  },
};
