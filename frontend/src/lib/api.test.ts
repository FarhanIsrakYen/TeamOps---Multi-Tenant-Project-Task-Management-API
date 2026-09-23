import { describe, expect, it } from "vitest";
import axios from "axios";
import { apiMessage, queryString, session } from "./api";

describe("apiMessage", () => {
  it("uses the API error envelope message", () => {
    const error = new axios.AxiosError(
      "request failed",
      "400",
      undefined,
      undefined,
      {
        data: { error: { message: "organization slug is already in use" } },
        status: 409,
        statusText: "Conflict",
        headers: {},
        config: { headers: new axios.AxiosHeaders() },
      },
    );
    expect(apiMessage(error)).toBe("organization slug is already in use");
  });

  it("does not expose arbitrary thrown values", () => {
    expect(apiMessage(new Error("database details"))).toBe(
      "Something went wrong",
    );
  });
});

describe("queryString", () => {
  it("omits empty filters and encodes meaningful values", () => {
    expect(
      queryString({
        page: 2,
        search: "release plan",
        status: "",
        archived: false,
      }),
    ).toBe("?page=2&search=release+plan&archived=false");
  });
});

describe("session", () => {
  it("keeps credentials in memory and notifies subscribers", () => {
    const stored = {
      accessToken: "access",
      accessTokenExpiresAt: "2099-01-01T00:00:00Z",
      refreshToken: "refresh",
      user: {
        id: "user-id",
        email: "person@example.com",
        name: "Person",
        createdAt: "2026-01-01T00:00:00Z",
        updatedAt: "2026-01-01T00:00:00Z",
      },
    };
    const values: Array<typeof stored | null> = [];
    const unsubscribe = session.subscribe((value) => values.push(value));

    session.set(stored);
    session.clear();
    unsubscribe();

    expect(values).toEqual([stored, null]);
    expect(session.get()).toBeNull();
  });

  it("does not let an obsolete refresh overwrite a newer session", () => {
    const original = {
      accessToken: "old-access",
      accessTokenExpiresAt: "2026-01-01T00:00:00Z",
      refreshToken: "old-refresh",
      user: {
        id: "old-user",
        email: "old@example.com",
        name: "Old",
        createdAt: "2026-01-01T00:00:00Z",
        updatedAt: "2026-01-01T00:00:00Z",
      },
    };
    const replacement = {
      ...original,
      accessToken: "new-access",
      refreshToken: "new-refresh",
      user: { ...original.user, id: "new-user", email: "new@example.com" },
    };
    const staleRefresh = {
      ...original,
      accessToken: "stale-access",
      refreshToken: "stale-refresh",
    };

    session.set(original);
    const snapshot = session.snapshot();
    session.set(replacement);

    expect(session.replaceIfCurrent(snapshot, staleRefresh)).toBe(false);
    expect(session.clearIfCurrent(snapshot)).toBe(false);
    expect(session.get()).toEqual(replacement);
    session.clear();
  });
});
