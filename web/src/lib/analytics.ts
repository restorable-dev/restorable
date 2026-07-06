"use client";

// Plausible custom events, safe to call whether or not analytics is loaded
// (the script is env-gated; local dev and self-hosters simply no-op).
declare global {
  interface Window {
    plausible?: (event: string, options?: { props?: Record<string, string> }) => void;
  }
}

export function track(event: string, props?: Record<string, string>) {
  if (typeof window !== "undefined" && typeof window.plausible === "function") {
    window.plausible(event, props ? { props } : undefined);
  }
}
