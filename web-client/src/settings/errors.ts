import { ApiError } from "../api";
import { loadWebPreferences } from "./web-preferences";

const NETWORK_CODES = new Set(["network_error", "events_unavailable"]);

export function displayError(error: unknown, fallback = "Произошла ошибка. Попробуйте ещё раз"): string {
  if (typeof error === "string") return error;
  if (loadWebPreferences().debugErrors) return debugError(error, fallback);
  if (error instanceof ApiError && NETWORK_CODES.has(error.code)) return "Соединение потеряно";
  return fallback;
}

export function debugError(error: unknown, fallback: string): string {
  if (error instanceof ApiError) {
    const parts = [`${error.message} [${error.status}/${error.code}]`];
    if (error.requestId) parts.push(`request_id: ${error.requestId}`);
    return parts.join(" ");
  }
  if (error instanceof Error) return error.stack || error.message;
  return fallback;
}
