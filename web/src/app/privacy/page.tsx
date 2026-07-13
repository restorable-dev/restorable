import type { Metadata } from "next";

import { LegalPage } from "../legal/legal";

export const metadata: Metadata = { title: "Privacy Policy — Restorable" };

export default function PrivacyPage() {
  return (
    <LegalPage title="Privacy Policy" updated="July 2026">
      <p>
        Restorable is operated by ClevTech Solutions (&ldquo;we&rdquo;). This
        policy explains what we collect and why. The short version: your backup
        contents never reach us, and we collect the minimum needed to run the
        service.
      </p>

      <h2>Your backup data never leaves your machine</h2>
      <p>
        The Restorable agent runs on your own hardware. It restores and verifies
        your backups locally, in a disposable sandbox, and then reports only
        <strong> pass/fail metadata</strong> to us: a repository fingerprint (a
        one-way hash — never the repository location or credentials), snapshot
        IDs, per-check results, timings, and the agent version. File contents,
        file names from your data, and repository passwords are never
        transmitted and have no code path off your machine. Error messages are
        scrubbed of credentials before they leave the agent.
      </p>

      <h2>What we collect</h2>
      <ul>
        <li>
          <strong>Account:</strong> your email address, used to sign in and to
          send service and alert emails.
        </li>
        <li>
          <strong>Agent activity:</strong> the pass/fail run metadata described
          above, tied to your account.
        </li>
        <li>
          <strong>Alert settings:</strong> the destinations you configure
          (Telegram chat, Discord webhook, ntfy topic, or email address).
        </li>
        <li>
          <strong>Billing:</strong> if you subscribe, Stripe processes your
          payment. We store only a customer ID and subscription status — never
          your card details.
        </li>
        <li>
          <strong>Usage analytics:</strong> aggregate, cookieless page-view and
          event data via Vercel Web Analytics. It is not tied to your identity
          and is not used to track you across other sites.
        </li>
      </ul>

      <h2>Service providers</h2>
      <p>We rely on these processors, each handling only what their function needs:</p>
      <ul>
        <li>Supabase — database and authentication</li>
        <li>Vercel — application hosting and analytics</li>
        <li>Amazon Web Services (SES) — sending email</li>
        <li>Stripe — payment processing</li>
        <li>Telegram, Discord, ntfy — only if you configure those alert channels</li>
      </ul>

      <h2>Data retention and deletion</h2>
      <p>
        Run history is retained per your plan (3 months on Free, 12 months on
        Pro) and older records are pruned automatically. You can delete any
        agent, repository, or alert channel at any time from your dashboard. To
        delete your account and all associated data, contact us and we will
        remove it.
      </p>

      <h2>Contact</h2>
      <p>
        Questions about privacy: <a href="mailto:privacy@restorable.dev">privacy@restorable.dev</a>.
      </p>
    </LegalPage>
  );
}
