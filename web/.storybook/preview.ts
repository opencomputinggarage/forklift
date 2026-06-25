import type { Preview } from "@storybook/react-vite";

import "../src/styles.css";

const preview: Preview = {
  parameters: {
    controls: {
      matchers: {
        color: /(background|color)$/i,
        date: /Date$/i,
      },
    },
    backgrounds: {
      default: "forklift",
      values: [
        { name: "forklift", value: "#090b0f" },
        { name: "panel", value: "#101318" },
      ],
    },
  },
};

export default preview;
