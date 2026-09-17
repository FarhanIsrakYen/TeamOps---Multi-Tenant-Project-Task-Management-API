import { describe, expect, it } from "vitest";
import axios from "axios";
import { apiMessage } from "./api";

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
