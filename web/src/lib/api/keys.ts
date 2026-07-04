import { createHash, randomBytes } from "node:crypto";

// Key material: URL-safe base64 of 32 random bytes, with a recognizable
// prefix. Only the sha256 hash of either kind is ever stored.
//   rrt_… registration tokens (minted in the dashboard, exchanged once)
//   rsk_… agent API keys (returned once at registration)

export const REGISTRATION_TOKEN_PREFIX = "rrt_";
export const API_KEY_PREFIX = "rsk_";

function randomSecret(prefix: string): string {
  return prefix + randomBytes(32).toString("base64url");
}

export function generateRegistrationToken(): string {
  return randomSecret(REGISTRATION_TOKEN_PREFIX);
}

export function generateApiKey(): string {
  return randomSecret(API_KEY_PREFIX);
}

// hashSecret is deterministic sha256 — appropriate for high-entropy random
// keys (unlike passwords, they cannot be dictionary-attacked, and equality
// lookup by hash needs determinism).
export function hashSecret(secret: string): string {
  return createHash("sha256").update(secret).digest("hex");
}
