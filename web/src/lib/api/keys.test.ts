import { describe, expect, it } from "vitest";

import {
  generateApiKey,
  generateRegistrationToken,
  hashSecret,
} from "./keys";

describe("key generation", () => {
  it("produces prefixed, high-entropy, unique values", () => {
    const token = generateRegistrationToken();
    const key = generateApiKey();
    expect(token).toMatch(/^rrt_[A-Za-z0-9_-]{40,}$/);
    expect(key).toMatch(/^rsk_[A-Za-z0-9_-]{40,}$/);
    expect(generateApiKey()).not.toBe(generateApiKey());
  });
});

describe("hashSecret", () => {
  it("is deterministic sha256 hex", () => {
    expect(hashSecret("rsk_x")).toBe(hashSecret("rsk_x"));
    expect(hashSecret("rsk_x")).toMatch(/^[a-f0-9]{64}$/);
    expect(hashSecret("rsk_x")).not.toBe(hashSecret("rsk_y"));
  });
});
