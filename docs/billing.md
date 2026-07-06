# Plans & billing

| | Free | Pro ($8/mo or $80/yr) |
|---|---|---|
| Repositories | 1 | Unlimited |
| Test schedule | Monthly | Any |
| Alert channels | 1 | All |
| Run history | 3 months | 12 months |

The free tier is genuinely usable — one repo, tested monthly, with a real
alert channel. Pro removes the limits and adds monthly confidence reports
(post-launch).

## How limits behave

Limits are enforced server-side, at the moment of growth:

- A run for a **new** repository beyond your limit is rejected with a clear
  `plan limit` error. Repositories that already report are never blocked.
- Adding an alert channel beyond your limit is refused — enforced by the
  database itself, not just the UI.

**Downgrades are graceful.** Cancelling keeps every repo, run, and channel
you have; the limits simply apply to *new* additions again. Nothing is
deleted, hidden, or broken.

Upgrades take effect immediately — entitlements are read live from the
subscription on every request, so there is nothing to redeploy or restart.

## Mechanics

Checkout and subscription management happen on Stripe (Checkout + customer
portal — card details never touch Restorable's servers). Webhooks update the
subscription state; every webhook is signature-verified, schema-validated,
and processed exactly once (replays are acknowledged no-ops).
