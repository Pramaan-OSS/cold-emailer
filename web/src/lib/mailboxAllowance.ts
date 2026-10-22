import type { AppError } from "@/lib/api/client/normalizeError";
export function isAllowanceError(e: unknown): boolean { return (e as AppError | undefined)?.code === "mailbox_allowance_reached"; }
