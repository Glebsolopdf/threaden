import { ApiError } from "../api";
import { displayError } from "../settings/errors";

export function errorMessage(error: unknown): string {
  if (error instanceof ApiError) return displayError(error);
  if (error instanceof Error) return error.message;
  return "Произошла неизвестная ошибка";
}

export function validateEmail(input: HTMLInputElement): string {
  const email = input.value.trim().toLowerCase();
  if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email)) throw new Error("Введите корректный email");
  return email;
}

export function validatePassword(input: HTMLInputElement, minimum = 10): string {
  const bytes = new TextEncoder().encode(input.value).length;
  if (bytes < minimum || bytes > 72) {
    throw new Error(`Пароль должен содержать от ${minimum} до 72 байт`);
  }
  return input.value;
}
