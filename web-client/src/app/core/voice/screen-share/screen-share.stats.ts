import type { LocalVideoTrack } from 'livekit-client';
import type { ScreenCaptureSettings } from './screen-share.models';

export interface ScreenShareStats {
  capture?: ScreenCaptureSettings;
  encodedWidth?: number;
  encodedHeight?: number;
  outgoingFps?: number;
  bitrate?: number;
  packetsSent?: number;
  packetsLost?: number;
  retransmittedPackets?: number;
  rttMs?: number;
  codec?: string;
  nackCount?: number;
  pliCount?: number;
  qualityLimitationReason?: string;
  totalEncodeTime?: number;
}

export async function readScreenShareStats(track: LocalVideoTrack, capture: ScreenCaptureSettings, rttMs?: number): Promise<ScreenShareStats> {
  const stats = await track.getSenderStats();
  const primary = stats.reduce((best, current) => current.frameWidth * current.frameHeight > best.frameWidth * best.frameHeight ? current : best, stats[0]);
  if (!primary) return { capture, rttMs };
  const sender = track.sender;
  const rawStats = sender ? await sender.getStats() : undefined;
  let codec: string | undefined;
  let totalEncodeTime: number | undefined;
  rawStats?.forEach((report) => {
    if (report.type === 'outbound-rtp' && report.kind === 'video') totalEncodeTime = Number(report.totalEncodeTime) || undefined;
    if (report.type === 'codec') codec = typeof report.mimeType === 'string' ? report.mimeType.replace('video/', '').toUpperCase() : undefined;
  });
  return {
    capture, encodedWidth: primary.frameWidth, encodedHeight: primary.frameHeight,
    outgoingFps: primary.framesPerSecond, bitrate: primary.targetBitrate,
    packetsSent: primary.packetsSent, packetsLost: primary.packetsLost,
    retransmittedPackets: primary.retransmittedPacketsSent, rttMs: primary.roundTripTime ? primary.roundTripTime * 1000 : rttMs,
    codec, nackCount: primary.nackCount, pliCount: primary.pliCount,
    qualityLimitationReason: primary.qualityLimitationReason, totalEncodeTime,
  };
}
