export function populateDevices(select: HTMLSelectElement, devices: MediaDeviceInfo[], fallback: string): void {
  const options = [
    new Option(fallback, ""),
    ...devices.map((device, index) => new Option(device.label || `Устройство ${index + 1}`, device.deviceId)),
  ];
  select.replaceChildren(...options);
  select.disabled = devices.length === 0;
}

export function setDeviceMenuOpen(button: HTMLButtonElement, menu: HTMLElement, open: boolean): void {
  button.setAttribute("aria-expanded", String(open));
  menu.hidden = !open;
}
