import { HttpInterceptorFn } from '@angular/common/http';
import { apiBaseUrl } from './runtime-config';

export const apiInterceptor: HttpInterceptorFn = (request, next) => {
  const apiRequest = request.url.startsWith('/v1/')
    ? request.clone({ url: `${apiBaseUrl()}${request.url}`, withCredentials: true })
    : request;
  return next(apiRequest);
};
