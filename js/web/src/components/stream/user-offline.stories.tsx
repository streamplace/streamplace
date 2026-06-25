import { withAllProviders } from "@/../.storybook/decorators";
import type { Meta, StoryObj } from "@storybook/react-vite";
import { UserOffline } from "./user-offline";

const meta: Meta<typeof UserOffline> = {
  component: UserOffline,
  parameters: {
    layout: "centered",
    backgrounds: { default: "dark" },
  },
  decorators: [
    withAllProviders,
    (Story) => (
      <div className="relative h-64 w-full max-w-2xl overflow-hidden rounded-lg bg-black">
        <Story />
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
