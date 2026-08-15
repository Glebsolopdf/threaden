export function getVoiceWaveform(level: number, count: number): number[] {
  const normalizedLevel = Math.max(0, Math.min(1, level));
  return Array.from({ length: count }, (_, index) => {
    const proximityToMicrophone = count > 1 ? (index + 1) / count : 1;
    return Number((0.18 + normalizedLevel * 0.65 * proximityToMicrophone).toFixed(2));
  });
}
