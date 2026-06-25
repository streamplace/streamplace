import type { Meta, StoryObj } from "@storybook/react-vite";
import { MentionList, type MentionItem } from "./mention-list";

const meta: Meta<typeof MentionList> = {
  component: MentionList,
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
type Story = StoryObj<typeof MentionList>;

const noop = () => {};

const items: MentionItem[] = [
  {
    did: "did:plc:alice",
    handle: "alice.bsky.social",
    displayName: "Alice",
    avatar: "https://placehold.co/48x48/e94560/fff?text=A",
    color: { red: 233, green: 69, blue: 96 },
  },
  {
    did: "did:plc:bob",
    handle: "bob.bsky.social",
    displayName: "Bob",
    avatar: null,
    color: null,
  },
  {
    did: "did:plc:charlie",
    handle: "charlie.example.com",
    displayName: "",
    avatar: "https://placehold.co/48x48/4a9eff/fff?text=C",
    color: { red: 74, green: 158, blue: 255 },
  },
  {
    did: "did:plc:dana",
    handle: "dana.bsky.social",
    displayName: "Dana the Dev",
    avatar: null,
    color: { red: 100, green: 200, blue: 100 },
  },
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

export const NoAvatar: Story = {
  args: {
    items: items.filter((i) => !i.avatar),
    command: noop,
  },
};

export const NoDisplayName: Story = {
  args: {
    items: [
      {
        did: "did:plc:nodisplay",
        handle: "nobody.bsky.social",
        displayName: "",
        avatar: null,
        color: null,
      },
    ],
    command: noop,
  },
};
