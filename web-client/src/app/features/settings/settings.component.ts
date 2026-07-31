import { ChangeDetectionStrategy, Component, DestroyRef, inject, signal } from '@angular/core';
import { FormControl, FormGroup, ReactiveFormsModule } from '@angular/forms';
import { RouterLink } from '@angular/router';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { Room } from 'livekit-client';
import { PreferencesService } from '../../core/preferences/preferences.service';
import { NotificationStore } from '../../core/notifications/notification.store';

@Component({
  selector: 'app-settings',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [ReactiveFormsModule, RouterLink],
  template: `
    <section class="route-page settings-view">
      <header class="group-header"><a class="group-header__icon" routerLink="/" aria-label="Назад"><img src="/back.svg" alt=""></a><strong>Настройки</strong></header>
      <div class="settings-panel" [formGroup]="form">
        <section>
          <h2>Звук</h2>
          <label>Микрофон<select formControlName="inputDeviceId"><option value="">Системный микрофон</option>@for (device of inputs(); track device.deviceId) { <option [value]="device.deviceId">{{ device.label || 'Микрофон' }}</option> }</select></label>
          <label>Динамики<select formControlName="outputDeviceId"><option value="">Системные динамики</option>@for (device of outputs(); track device.deviceId) { <option [value]="device.deviceId">{{ device.label || 'Динамики' }}</option> }</select></label>
          <label class="settings-toggle"><input type="checkbox" formControlName="microphoneEnabled"><span>Включать микрофон при входе в комнату</span></label>
          <label>Громкость по умолчанию<input type="range" min="0" max="100" formControlName="outputVolume"></label>
          <button type="button" [disabled]="loading()" (click)="loadDevices()">{{ loading() ? 'Обновляем…' : 'Обновить устройства' }}</button>
        </section>
        <section>
          <h2>Интерфейс</h2>
          <label class="settings-toggle"><input type="checkbox" formControlName="debugErrors"><span>Показывать DEBUG-ошибки</span></label>
          <p>Тема: threaden по умолчанию. Клиент следует системной настройке уменьшения анимации.</p>
        </section>
      </div>
    </section>
  `,
})
export class SettingsComponent {
  private readonly preferences = inject(PreferencesService);
  private readonly notifications = inject(NotificationStore);
  protected readonly inputs = signal<MediaDeviceInfo[]>([]);
  protected readonly outputs = signal<MediaDeviceInfo[]>([]);
  protected readonly loading = signal(false);
  protected readonly form = new FormGroup({
    inputDeviceId: new FormControl(this.preferences.audio().inputDeviceId, { nonNullable: true }),
    outputDeviceId: new FormControl(this.preferences.audio().outputDeviceId, { nonNullable: true }),
    microphoneEnabled: new FormControl(this.preferences.audio().microphoneEnabled, { nonNullable: true }),
    outputVolume: new FormControl(this.preferences.audio().outputVolume, { nonNullable: true }),
    debugErrors: new FormControl(this.preferences.web().debugErrors, { nonNullable: true }),
  });

  constructor() {
    this.form.valueChanges.pipe(takeUntilDestroyed(inject(DestroyRef))).subscribe(() => {
      const value = this.form.getRawValue();
      this.preferences.updateAudio({
        inputDeviceId: value.inputDeviceId,
        outputDeviceId: value.outputDeviceId,
        microphoneEnabled: value.microphoneEnabled,
        outputVolume: value.outputVolume,
      });
      this.preferences.updateWeb({ debugErrors: value.debugErrors });
    });
    void this.loadDevices(false);
  }

  protected async loadDevices(announce = true): Promise<void> {
    this.loading.set(true);
    try {
      const [inputs, outputs] = await Promise.all([Room.getLocalDevices('audioinput', false), Room.getLocalDevices('audiooutput', false)]);
      this.inputs.set(inputs);
      this.outputs.set(outputs);
      if (announce) this.notifications.success('Устройства обновлены');
    } catch (error) { this.notifications.error(error, 'Не удалось получить список устройств'); }
    finally { this.loading.set(false); }
  }
}
