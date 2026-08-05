import type { LocalVideoTrack, RemoteVideoTrack } from 'livekit-client';

export type ScreenShareMode = 'quality' | 'balanced' | 'smooth';

export interface ScreenShareProfile {
  width: number;
  height: number;
  frameRate: number;
  maxBitrate: number;
  contentHint: 'detail' | 'motion';
}

export const SCREEN_SHARE_PROFILES: Readonly<Record<ScreenShareMode, ScreenShareProfile>> = {
  quality: { width: 1920, height: 1080, frameRate: 15, maxBitrate: 3_500_000, contentHint: 'detail' },
  balanced: { width: 1920, height: 1080, frameRate: 30, maxBitrate: 6_000_000, contentHint: 'detail' },
  smooth: { width: 1280, height: 720, frameRate: 60, maxBitrate: 6_000_000, contentHint: 'motion' },
};

export const SCREEN_SHARE_MODE_LABELS: Readonly<Record<ScreenShareMode, string>> = {
  quality: 'Чёткость — 1080p, 15 FPS',
  balanced: 'Стандарт — 1080p, 30 FPS',
  smooth: 'Плавность — 720p, 60 FPS',
};

export interface ScreenCaptureSettings {
  width?: number;
  height?: number;
  frameRate?: number;
  displaySurface?: string;
}

export interface ScreenShare {
  participantIdentity: string;
  participantName: string;
  publicationSid: string;
  trackSid: string;
  videoTrack: LocalVideoTrack | RemoteVideoTrack;
  hasAudio: boolean;
  isMuted: boolean;
  dimensions?: { width: number; height: number };
  isLocal: boolean;
}

export interface ScreenShareError {
  message: string;
  technicalMessage?: string;
}
