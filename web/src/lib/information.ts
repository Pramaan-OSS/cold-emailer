import { runtimeEnv } from "./runtimeConfig";

// These read the container-injected runtime config first (so one built image
// works for any deployment), then the value Vite baked at build time.
export const APP_URL = runtimeEnv("APP_URL", import.meta.env.VITE_APP_URL);
export const API_URL = runtimeEnv("API_URL", import.meta.env.VITE_API_URL);
// The whole dashboard talks to the versioned API. VITE_API_URL is a bare origin
// (no path), so this is the single place the /v1 prefix is applied.
export const API_BASE_URL = `${API_URL}/v1`;
// Visibility only; the backend still decides whether a new mailbox can connect.

// Shown once in a dialog and then as a header pill. Empty means this is not a
// preview deployment and nothing renders, which is what production wants
// without anyone having to remember to unset it.
export const BETA_NOTICE = runtimeEnv("BETA_NOTICE", import.meta.env.VITE_BETA_NOTICE);
export const HUMAN_VERIFICATION_FAIL = "We couldn’t verify you’re human. Please try the security check again or reload the page.";
export const PASSWORD_FAIL = "The password must be at least 8 characters long and contain both uppercase and lowercase letters, as well as a number."
export const TOKEN_KEY = "auth_token";
export const DEFAULT_PAGINATION_LIMIT = 50;
export const SAVING = "Saving...";
export const REORDERING = "Reordering...";
export const SUCCESS = "Changes successfully changed.";
export const DELETING = "Deleting...";
export const DELETED = "Successfully deleted.";
export const REMOVING = "Removing...";
export const REMOVED = "Successfully removed.";
export const CREATING = "Creating...";
export const CREATED = "Successfully created."
export const ADDING = "Adding..."
export const ADDED = "Successfully added."
