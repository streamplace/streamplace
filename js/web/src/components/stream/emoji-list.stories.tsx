import type { Meta, StoryObj } from "@storybook/react-vite";
import type { Emoji } from "../../lib/emoji-data";
import { EmojiList } from "./emoji-list";

const meta: Meta<typeof EmojiList> = {
  component: EmojiList,
  parameters: {
    layout: "fullscreen",
    backgrounds: { default: "dark" },
  },
  decorators: [
    (Story) => (
      <div className="flex h-svh w-full items-start justify-center bg-[#0c0a14] p-8">
        <div className="w-80">
          <Story />
        </div>
      </div>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof EmojiList>;

const noop = () => {};

const makeEmoji = (
  id: string,
  m: string,
  native: string,
  k: string[] = [],
): Emoji => ({
  id,
  m,
  k,
  s: [{ n: native }],
});

const items: Emoji[] = [
  makeEmoji("grinning-face", "Grinning Face", "😀", ["happy", "smile"]),
  makeEmoji("joy", "Face with Tears of Joy", "😂", ["lol", "laugh"]),
  makeEmoji("red-heart", "Red Heart", "❤️", ["love", "heart"]),
  makeEmoji("fire", "Fire", "🔥", ["hot", "lit"]),
  makeEmoji("thumbs-up", "Thumbs Up", "👍", ["yes", "like"]),
  makeEmoji("party-popper", "Party Popper", "🎉", ["celebrate", "party"]),
  makeEmoji("rocket", "Rocket", "🚀", ["launch", "space"]),
  makeEmoji("clapping-hands", "Clapping Hands", "👏", ["clap", "applause"]),
];

export const Default: Story = {
  args: {
    items,
    command: noop,
  },
};

export const SingleItem: Story = {
  args: {
    items: [items[0]],
    command: noop,
  },
};

export const LongList: Story = {
  args: {
    items: [
      ...items,
      makeEmoji("partying-face", "Partying Face", "🥳"),
      makeEmoji("smiling-face-with-hearts", "Smiling Face with Hearts", "🥰"),
      makeEmoji("star-struck", "Star Struck", "🤩"),
      makeEmoji("face-blowing-a-kiss", "Face Blowing a Kiss", "😘"),
      makeEmoji("winking-face", "Winking Face", "😉"),
      makeEmoji("slightly-smiling-face", "Slightly Smiling Face", "🙂"),
      makeEmoji("hugging-face", "Hugging Face", "🤗"),
      makeEmoji("thinking-face", "Thinking Face", "🤔"),
    ],
    command: noop,
  },
};
