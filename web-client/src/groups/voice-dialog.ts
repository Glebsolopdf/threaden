import { byId } from "../dom";

export function createVoiceRoomDialog(options: {
  onCreate: (name: string) => Promise<void>;
  onError: (error: unknown) => void;
  onNeutral: (message: string) => void;
}) {
  const dialog = byId<HTMLDialogElement>("create-voice-dialog");
  const form = byId<HTMLFormElement>("create-voice-form");
  const name = byId<HTMLInputElement>("create-voice-name");
  const submit = byId<HTMLButtonElement>("confirm-create-voice");
  const cancel = byId<HTMLButtonElement>("cancel-create-voice");

  cancel.onclick = () => dialog.close();
  dialog.addEventListener("click", (event) => {
    if (event.target === dialog) dialog.close();
  });
  form.onsubmit = (event) => {
    event.preventDefault();
    void submitRoom();
  };

  async function submitRoom(): Promise<void> {
    const value = name.value.trim();
    if (!value) {
      options.onNeutral("Введите название комнаты");
      name.focus();
      return;
    }
    submit.disabled = true;
    submit.dataset.state = "loading";
    try {
      await options.onCreate(value);
      dialog.close();
    } catch (error) {
      options.onError(error);
    } finally {
      submit.disabled = false;
      submit.dataset.state = "idle";
    }
  }
  return {
    open(): void {
      form.reset();
      dialog.showModal();
      name.focus();
    },
  };
}
