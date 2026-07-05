import { describe, expect, it } from "vitest";

import { parseChannelConfig } from "./config";

describe("channel config validation", () => {
  it("accepts valid configs per type", () => {
    expect(() => parseChannelConfig("telegram", { chat_id: "123456" })).not.toThrow();
    expect(() => parseChannelConfig("telegram", { chat_id: "-100987" })).not.toThrow();
    expect(() =>
      parseChannelConfig("discord", {
        webhook_url: "https://discord.com/api/webhooks/1/abc",
      }),
    ).not.toThrow();
    expect(() => parseChannelConfig("ntfy", { topic: "my-backups" })).not.toThrow();
    expect(() =>
      parseChannelConfig("ntfy", { topic: "t", server: "https://ntfy.example.com" }),
    ).not.toThrow();
    expect(() => parseChannelConfig("email", { to: "a@example.com" })).not.toThrow();
  });

  it.each([
    ["telegram non-numeric chat", "telegram", { chat_id: "abc" }],
    ["discord non-discord host", "discord", { webhook_url: "https://evil.com/api/webhooks/1/x" }],
    ["discord non-webhook path", "discord", { webhook_url: "https://discord.com/channels/1" }],
    ["ntfy bad topic", "ntfy", { topic: "has spaces" }],
    ["ntfy http server", "ntfy", { topic: "t", server: "http://ntfy.example.com" }],
    ["ntfy localhost server", "ntfy", { topic: "t", server: "https://localhost:8443" }],
    ["ntfy private ip server", "ntfy", { topic: "t", server: "https://192.168.1.10" }],
    ["email invalid", "email", { to: "not-an-email" }],
    ["unknown extra field", "telegram", { chat_id: "1", extra: "x" }],
  ] as const)("rejects %s", (_name, type, config) => {
    expect(() => parseChannelConfig(type, config)).toThrow();
  });
});
