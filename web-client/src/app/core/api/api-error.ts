import { HttpErrorResponse } from '@angular/common/http';

interface ErrorEnvelope {
  error?: { code?: string; message?: string; request_id?: string };
}

export class ApiError extends Error {
  constructor(
    readonly status: number,
    readonly code: string,
    message: string,
    readonly requestId = '',
  ) {
    super(message);
    this.name = 'ApiError';
  }
}

export function toApiError(error: unknown): ApiError {
  if (error instanceof ApiError) return error;
  if (error instanceof HttpErrorResponse) {
    const envelope = error.error as ErrorEnvelope | undefined;
    return new ApiError(
      error.status,
      envelope?.error?.code ?? (error.status === 0 ? 'network_error' : 'request_failed'),
      envelope?.error?.message ?? (error.status === 0 ? 'Не удалось связаться с сервером' : `Ошибка сервера (${error.status})`),
      envelope?.error?.request_id ?? '',
    );
  }
  if (error instanceof Error) return new ApiError(0, 'client_error', error.message);
  return new ApiError(0, 'unknown_error', 'Произошла неизвестная ошибка');
}
