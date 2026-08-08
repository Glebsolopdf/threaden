export async function testMicrophone(): Promise<void> {
  const stream = await navigator.mediaDevices.getUserMedia({ audio: true });
  window.setTimeout(() => stream.getTracks().forEach((track) => track.stop()), 1500);
}
