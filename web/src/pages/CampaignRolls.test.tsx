import { render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { afterEach, expect, test, vi } from "vitest";
import { CampaignRolls } from "./CampaignRolls";
import { AuthProvider } from "../auth/AuthContext";

afterEach(() => vi.unstubAllGlobals());

test("shows recent campaign check rolls", async () => {
  vi.stubGlobal(
    "fetch",
    vi.fn((input: RequestInfo | URL) => {
      const url = String(input);
      return Promise.resolve(
        new Response(
          JSON.stringify(
            url.endsWith("/auth/me")
              ? {
                  id: "account-1",
                  username: "player",
                  displayName: "玩家甲",
                  role: "user",
                  mustChangePassword: false,
                }
              : {
                  items: [
                    {
                      id: "roll-1",
                      requestId: "request-1",
                      actorAccountId: "account-1",
                      actorName: "玩家甲",
                      characterId: "character-1",
                      characterName: "调查员甲",
                      campaignId: "campaign-1",
                      visibility: "public",
                      kind: "check",
                      label: "侦查",
                      expression: "1d100",
                      rerollOfId: null,
                      rerollKind: null,
                      result: {
                        target: 60,
                        value: 32,
                        outcome: "hard",
                        candidates: [32],
                        bonusPenalty: 0,
                        units: 2,
                        tens: [3],
                        hard: 30,
                        extreme: 12,
                      },
                      createdAt: "2026-08-23T10:00:00Z",
                    },
                  ],
                },
          ),
          { status: 200, headers: { "Content-Type": "application/json" } },
        ),
      );
    }),
  );
  render(
    <MemoryRouter>
      <AuthProvider>
        <CampaignRolls campaignID="campaign-1" />
      </AuthProvider>
    </MemoryRouter>,
  );
  expect(await screen.findByText("玩家甲")).toBeInTheDocument();
  expect(
    screen.getByText("侦查").parentElement?.nextElementSibling,
  ).toHaveTextContent("32/ 60 · 困难成功");
});
