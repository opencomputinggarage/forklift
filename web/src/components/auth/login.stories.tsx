import type { Meta, StoryObj } from "@storybook/react-vite";
import { api } from "@/api";
import { Login } from "@/components/auth/login";

const meta: Meta<typeof Login> = {
  title: "Forklift/Auth/Login",
  component: Login,
  parameters: {
    layout: "fullscreen",
  },
};

export default meta;

type Story = StoryObj<typeof Login>;

export const LocalOnly: Story = {
  render: () => {
    api.version = async () => ({ version: "storybook", commit: "none", oidc_enabled: false });
    api.login = async (username: string) => ({ username });

    return <Login onLogin={async () => undefined} />;
  },
};

export const WithKeycloak: Story = {
  render: () => {
    api.version = async () => ({ version: "storybook", commit: "none", oidc_enabled: true });
    api.login = async (username: string) => ({ username });

    return <Login onLogin={async () => undefined} />;
  },
};
