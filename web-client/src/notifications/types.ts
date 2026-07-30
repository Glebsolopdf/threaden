export type NotificationKind = "success" | "error" | "neutral";

export interface NotificationOptions {
  message: string;
  kind?: NotificationKind;
}

export interface NotificationCenter {
  show(options: NotificationOptions): void;
  success(message: string): void;
  error(message: string): void;
  neutral(message: string): void;
  clear(): void;
}
