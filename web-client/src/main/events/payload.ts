export interface ServerEventPayload {
  type: string;
  group_id?: string;
  data?: unknown;
}

export function parseEventPayload(raw: string): ServerEventPayload | null {
  const line = raw.split("\n").find((item) => item.startsWith("data: "));
  if (!line) return null;
  try {
    return JSON.parse(line.slice(6)) as ServerEventPayload;
  } catch {
    return null;
  }
}
