import type { Metadata } from "next";

import { LegalPage } from "../legal/legal";

export const metadata: Metadata = { title: "Terms of Service — Restorable" };

export default function TermsPage() {
  return (
    <LegalPage title="Terms of Service" updated="July 2026">
      <p>
        These terms govern your use of Restorable, operated by ClevTech
        Solutions (&ldquo;we&rdquo;). By creating an account or using the
        service you agree to them.
      </p>

      <h2>The service</h2>
      <p>
        Restorable restore-tests your backups and reports the results. The agent
        is open-source software (MIT licensed) that you run yourself; the hosted
        dashboard is an optional, additive service. Restorable performs only
        read operations against your backup repositories and is a verification
        tool — it is not itself a backup, and it does not replace keeping your
        own backups.
      </p>

      <h2>Beta</h2>
      <p>
        The hosted service is currently in beta. Paid plans are free during the
        beta period. Features may change, and availability is not guaranteed.
      </p>

      <h2>Your responsibilities</h2>
      <ul>
        <li>Keep your account credentials and agent API keys secure.</li>
        <li>Only connect the service to backup repositories you are authorized to access.</li>
        <li>Do not attempt to disrupt, reverse-engineer the hosted service, or abuse it.</li>
      </ul>

      <h2>Billing</h2>
      <p>
        If and when paid plans begin charging, subscriptions are billed through
        Stripe on the interval you select and renew until canceled. You can
        cancel anytime from the billing portal; access continues through the end
        of the paid period, and your data is retained.
      </p>

      <h2>No warranty; limitation of liability</h2>
      <p>
        The service is provided &ldquo;as is,&rdquo; without warranties of any
        kind. A passing test indicates the checks you configured succeeded on a
        given snapshot; it is not a guarantee against all possible data loss.
        You remain responsible for your own backup strategy. To the maximum
        extent permitted by law, we are not liable for indirect or consequential
        damages, or for data loss.
      </p>

      <h2>Changes and contact</h2>
      <p>
        We may update these terms; material changes will be reflected here.
        Questions: <a href="mailto:support@restorable.dev">support@restorable.dev</a>.
      </p>
    </LegalPage>
  );
}
