import type { Meta, StoryObj } from "@storybook/react-vite";
import StreamplaceSvg from "./streamplace-bw";

const meta: Meta<typeof StreamplaceSvg> = {
  component: StreamplaceSvg,
  parameters: {
    layout: "fullscreen",
    backgrounds: { default: "dark" },
  },
  decorators: [
    (Story) => (
      <div className="flex h-svh w-full items-center justify-center bg-[#0c0a14]">
        <Story />
      </div>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof StreamplaceSvg>;

export const Default: Story = {
  args: {},
};

export const Small: Story = {
  args: {
    width: 32,
    height: 32,
  },
};

export const Large: Story = {
  args: {
    width: 128,
    height: 128,
  },
};

export const OnLight: Story = {
  parameters: {
    backgrounds: { default: "light" },
  },
  args: {},
};
