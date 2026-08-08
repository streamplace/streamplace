import type { Preview } from "@storybook/react-vite";
import "../src/lib/i18n";
import { useStore } from "../src/lib/store";
import "../src/styles.css";

// Initialise the Zustand store so hooks like useStreamplaceUrl work.
useStore.getState().initialize();

// Ensure the dark class is on <html> so the theme CSS variables resolve.
// Storybook's `backgrounds` parameter only sets the canvas background
// color; it doesn't toggle the .dark class the app's CSS expects.
const preview: Preview = {
  parameters: {
    backgrounds: {
      default: "dark",
      values: [
        { name: "dark", value: "#0c0a14" },
        { name: "light", value: "#ffffff" },
      ],
    },
    controls: {
      matchers: {
        color: /(background|color)$/i,
        date: /Date$/i,
      },
    },
    a11y: {
      test: "todo",
    },
  },
  globalTypes: {
    theme: {
      name: "Theme",
      description: "Light or dark theme",
      defaultValue: "dark",
      toolbar: {
        icon: "circlehollow",
        items: [
          { value: "dark", icon: "moon", title: "Dark" },
          { value: "light", icon: "sun", title: "Light" },
        ],
        showName: true,
      },
    },
  },
  decorators: [
    (Story, context) => {
      const theme = context.globals?.theme ?? "dark";
      const root = document.documentElement;
      if (theme === "dark") {
        root.classList.add("dark");
      } else {
        root.classList.remove("dark");
      }
      return <Story />;
    },
  ],
};

export default preview;
