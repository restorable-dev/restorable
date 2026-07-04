"use server";

import { redirect } from "next/navigation";

import { generateRegistrationToken, hashSecret } from "@/lib/api/keys";
import { createClient } from "@/lib/supabase/server";

// Mints a one-time agent registration token for the signed-in user. The
// plaintext is returned for a single display; only the hash is stored.
// Inserted with the user's own client, so RLS guarantees ownership.
export async function mintRegistrationToken(): Promise<{ token?: string; error?: string }> {
  const supabase = await createClient();
  const {
    data: { user },
  } = await supabase.auth.getUser();
  if (!user) {
    return { error: "not signed in" };
  }

  const token = generateRegistrationToken();
  const { error } = await supabase.from("registration_tokens").insert({
    user_id: user.id,
    token_hash: hashSecret(token),
  });
  if (error) {
    return { error: "could not create registration token" };
  }
  return { token };
}

export async function signOut() {
  const supabase = await createClient();
  await supabase.auth.signOut();
  redirect("/login");
}
