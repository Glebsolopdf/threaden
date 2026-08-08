import type { Room } from 'livekit-client';

export class VoicePing {
  private timer?: number;

  constructor(private readonly room: Room, private readonly onUpdate: () => void) {}

  value(): number | undefined {
    const rtt = this.room.engine.client.rtt;
    if (rtt <= 0) return undefined;
    return Math.round(rtt < 1 ? rtt * 1000 : rtt);
  }

  start(): void {
    this.stop();
    this.timer = window.setInterval(this.onUpdate, 180);
  }

  stop(): void {
    if (this.timer === undefined) return;
    window.clearInterval(this.timer);
    this.timer = undefined;
  }
}
