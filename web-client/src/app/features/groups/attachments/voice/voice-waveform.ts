export function getVoiceWaveform(level: number, count: number): number[] {
  const normalizedLevel = Math.max(0, Math.min(1, level));
  const center = (count - 1) / 2;
  return Array.from({ length: count }, (_, index) => {
    const distance = center ? Math.abs(index - center) / center : 0;
    return Number((0.18 + normalizedLevel * 0.65 * (1 - distance)).toFixed(2));
  });
}
