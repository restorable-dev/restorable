import { z } from "zod";

// Per-type channel config schemas. Validated at channel creation AND again
// before every send (config lives in jsonb; never trust stored shape).

export const telegramConfigSchema = z.strictObject({
  chat_id: z.string().regex(/^-?\d{1,20}$/, "chat_id must be numeric"),
});

export const discordConfigSchema = z.strictObject({
  webhook_url: z
    .url()
    .refine(
      (u) => {
        try {
          const host = new URL(u).host;
          return (
            (host === "discord.com" || host === "discordapp.com") &&
            new URL(u).pathname.startsWith("/api/webhooks/")
          );
        } catch {
          return false;
        }
      },
      { message: "must be a Discord webhook URL (https://discord.com/api/webhooks/…)" },
    ),
});

// ntfy servers are commonly self-hosted, so any https host is allowed —
// but never localhost/private-network literals (we make the request).
const privateHost =
  /^(localhost|127\.|10\.|192\.168\.|169\.254\.|0\.|\[?::1\]?$|172\.(1[6-9]|2\d|3[01])\.)/i;

export const ntfyConfigSchema = z.strictObject({
  topic: z.string().regex(/^[A-Za-z0-9_-]{1,64}$/, "topic must be alphanumeric/-/_"),
  server: z
    .url()
    .refine(
      (u) => {
        try {
          const parsed = new URL(u);
          return parsed.protocol === "https:" && !privateHost.test(parsed.hostname);
        } catch {
          return false;
        }
      },
      { message: "server must be a public https:// URL" },
    )
    .optional(),
});

export const emailConfigSchema = z.strictObject({
  to: z.email(),
});

export const channelTypeSchema = z.enum(["telegram", "discord", "ntfy", "email"]);
export type ChannelType = z.infer<typeof channelTypeSchema>;

const schemaByType = {
  telegram: telegramConfigSchema,
  discord: discordConfigSchema,
  ntfy: ntfyConfigSchema,
  email: emailConfigSchema,
} as const;

export interface AlertChannel {
  id: string;
  type: ChannelType;
  config: unknown;
}

// parseChannelConfig validates a channel's stored config for its type.
export function parseChannelConfig<T extends ChannelType>(
  type: T,
  config: unknown,
): z.infer<(typeof schemaByType)[T]> {
  return schemaByType[type].parse(config) as z.infer<(typeof schemaByType)[T]>;
}
