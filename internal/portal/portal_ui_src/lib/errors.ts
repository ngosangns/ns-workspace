import { ApiError } from "../api";

/** Narrow unknown thrown values to a user-facing message. */
export function errMessage(e: unknown): string {
  if (e instanceof ApiError) return e.message;
  if (e instanceof Error) return e.message;
  return String(e);
}
