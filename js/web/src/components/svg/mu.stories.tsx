import type { Meta, StoryObj } from "@storybook/react-vite";
import MuIcon from "./mu";

const meta: Meta<typeof MuIcon> = {
  component: MuIcon,
  parameters: {
    layout: "fullscreen",
    backgrounds: { default: "dark" },
  },
  decorators: [
    (Story) => (
      <div className="flex h-svh w-full items-center justify-center text-white">
        <Story />
      </div>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof MuIcon>;

export const Default: Story = {
  args: {},
};

export const Small: Story = {
  args: {
    size: 16,
  },
};

export const Large: Story = {
  args: {
    size: 48,
  },
};

export const Colored: Story = {
  args: {
    color: "#e94560",
    size: 32,
  },
};
