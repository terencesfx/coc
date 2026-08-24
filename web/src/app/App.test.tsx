import { render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { afterEach, expect, test, vi } from "vitest";
import { AuthProvider } from "../auth/AuthContext";
import { App } from "./App";

afterEach(() => vi.unstubAllGlobals());

test("shows login when there is no session", async () => {
  vi.stubGlobal(
    "fetch",
    vi.fn(() => Promise.resolve(new Response("{}", { status: 401 }))),
  );
  render(
    <MemoryRouter>
      <AuthProvider>
        <App />
      </AuthProvider>
    </MemoryRouter>,
  );
  expect(
    await screen.findByRole("heading", { name: "登录 COC7版人物卡" }),
  ).toBeInTheDocument();
});
