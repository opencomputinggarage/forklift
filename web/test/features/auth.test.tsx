import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, test, vi } from "vitest";

import { api } from "@/api";
import { Login } from "@/components/auth/login";

vi.mock("@/api", () => ({
  api: {
    login: vi.fn(),
    version: vi.fn(),
  },
}));

const mockedApi = vi.mocked(api);

describe("auth login", () => {
  beforeEach(() => {
    window.localStorage.setItem("forklift.language", "en");
    mockedApi.version.mockResolvedValue({
      version: "test",
      commit: "test",
      oidc_enabled: false,
    });
    mockedApi.login.mockResolvedValue({ username: "admin" });
  });

  test("renders the local login form", async () => {
    render(<Login onLogin={vi.fn()} />);

    expect(screen.getByLabelText("Username")).toBeVisible();
    expect(screen.getByLabelText("Password")).toBeVisible();
    expect(screen.getByRole("button", { name: "Sign in" })).toBeVisible();
    await waitFor(() => expect(mockedApi.version).toHaveBeenCalledOnce());
    expect(screen.queryByRole("link", { name: "Sign in with Keycloak" })).not.toBeInTheDocument();
  });

  test("offers Keycloak login when OIDC is enabled", async () => {
    mockedApi.version.mockResolvedValueOnce({
      version: "test",
      commit: "test",
      oidc_enabled: true,
    });

    render(<Login onLogin={vi.fn()} />);

    const keycloakOption = await screen.findByText("Sign in with Keycloak");
    expect(keycloakOption.closest("a")).toHaveAttribute("href", "/auth/login");
  });

  test("submits local credentials and calls the login completion handler", async () => {
    const user = userEvent.setup();
    const onLogin = vi.fn();
    render(<Login onLogin={onLogin} />);

    await user.type(screen.getByLabelText("Username"), "admin");
    await user.type(screen.getByLabelText("Password"), "change-me");
    await user.click(screen.getByRole("button", { name: "Sign in" }));

    expect(mockedApi.login).toHaveBeenCalledWith("admin", "change-me");
    await waitFor(() => expect(onLogin).toHaveBeenCalledOnce());
  });

  test("submits with Enter only after username and password are both filled", async () => {
    const user = userEvent.setup();
    const onLogin = vi.fn();
    render(<Login onLogin={onLogin} />);

    await user.type(screen.getByLabelText("Username"), "admin{Enter}");
    expect(mockedApi.login).not.toHaveBeenCalled();

    await user.type(screen.getByLabelText("Password"), "change-me{Enter}");

    expect(mockedApi.login).toHaveBeenCalledWith("admin", "change-me");
    await waitFor(() => expect(onLogin).toHaveBeenCalledOnce());
  });

  test("shows the login error and does not complete login when credentials fail", async () => {
    const user = userEvent.setup();
    const onLogin = vi.fn();
    mockedApi.login.mockRejectedValue(new Error("bad credentials"));
    render(<Login onLogin={onLogin} />);

    await user.type(screen.getByLabelText("Username"), "admin");
    await user.type(screen.getByLabelText("Password"), "wrong-password");
    await user.click(screen.getByRole("button", { name: "Sign in" }));

    expect(await screen.findByText("bad credentials")).toBeVisible();
    expect(onLogin).not.toHaveBeenCalled();
  });
});
