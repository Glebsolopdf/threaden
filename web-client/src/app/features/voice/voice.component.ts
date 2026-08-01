import { ChangeDetectionStrategy, Component, computed, effect, inject, input, signal } from '@angular/core';
import { FormControl, FormGroup, ReactiveFormsModule, Validators } from '@angular/forms';
import { Router, RouterLink } from '@angular/router';
import { GroupsStore } from '../../core/events/groups.store';
import { NotificationStore } from '../../core/notifications/notification.store';
import { PreferencesService } from '../../core/preferences/preferences.service';
import { VoiceService, type VoiceParticipant } from '../../core/voice/voice.service';
import { SCREEN_SHARE_MODE_LABELS, type ScreenShareMode } from '../../core/voice/screen-share/screen-share.models';
import { AvatarComponent } from '../../shared/avatar/avatar.component';
import { ScreenShareCardsComponent } from './screen-share-cards/screen-share-cards.component';

@Component({
  selector: 'app-voice',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [ReactiveFormsModule, RouterLink, AvatarComponent, ScreenShareCardsComponent],
  template: `
    @if (!voice.activeRoom() && !temporaryCode() && !voiceRoomId()) {
      <section class="route-page voice-room voice-room--join">
        <header class="group-header"><a class="group-header__icon" routerLink="/" aria-label="Назад"><img src="/back.svg" alt=""></a><strong>Временная комната</strong></header>
        <div class="join-card panel auth-card">
          <h2>Один разговор без группы</h2>
          <p>Создайте комнату или введите 26-значный код.</p>
          <button class="button button--primary" type="button" [disabled]="pending()" (click)="createTemporary()">Создать комнату</button>
          <form class="auth-form" [formGroup]="joinForm" (ngSubmit)="joinTemporary()">
            <label>Код комнаты<input type="text" formControlName="code" maxlength="26" autocomplete="off" spellcheck="false"></label>
            <button class="button button--secondary" type="submit" [disabled]="joinForm.invalid || pending()">Войти</button>
          </form>
        </div>
      </section>
    } @else if (voice.activeRoom(); as room) {
      <section class="route-page voice-room" aria-labelledby="voice-room-title">
        <header class="voice-room__header">
          <button class="voice-room__icon-button" type="button" aria-label="Свернуть" (click)="minimize()"><img src="/back.svg" alt=""></button>
          <div class="voice-room__heading"><p>{{ room.kind === 'temporary' ? 'Временная комната' : 'Группа' }}</p><h2 id="voice-room-title">{{ room.title }}</h2></div>
          <div class="voice-room__summary" aria-live="polite"><span>Участники: {{ voice.participants().length }}</span><span class="voice-status">{{ voice.statusLabel() }}</span></div>
          <div class="voice-room__meta">
            <span>{{ voice.detailLabel() }}</span>
            @if (room.kind === 'temporary') {
              <span class="voice-room__code"><span>{{ room.id }}</span><button type="button" aria-label="Скопировать код комнаты" (click)="copyCode(room.id)"><img src="/copy.svg" alt=""></button></span>
            }
          </div>
          <button class="voice-room__leave" type="button" [disabled]="pending()" (click)="leave()"><img src="/exit.svg" alt=""><span>Выйти</span></button>
        </header>

        <div class="voice-room__content" [class.voice-room__content--sharing]="screenShares().length > 0">
          @if (screenShares().length) { <app-screen-share-cards [shares]="screenShares()" /> }
          <section class="voice-room__people" aria-label="Участники голосовой комнаты">
            <ul class="voice-participants">
              @for (participant of voice.participants(); track participant.identity) {
                <li class="voice-participant-card" animate.enter="voice-participant-enter" animate.leave="voice-participant-leave" [class.is-speaking]="participant.isSpeaking" [style.--participant-bg]="avatarColor(participant)" [style.--avatar-bg]="avatarColor(participant)">
                  <app-avatar class="voice-participant-card__avatar" [src]="participant.avatar" [label]="participant.name" />
                  <footer><strong>{{ participant.name }}{{ participant.isLocal ? ' (Вы)' : '' }}</strong></footer>
                  @if (participant.isMicrophoneMuted) { <span class="voice-participant-card__mic" aria-label="Микрофон выключен"></span> }
                </li>
              } @empty { <li class="empty-copy">Подключаем участников…</li> }
            </ul>
          </section>
          <section class="voice-room__controls" aria-label="Управление голосовой комнатой">
            <button class="voice-room__icon-button" type="button" [disabled]="pending() || voice.status() !== 'connected'" [attr.aria-pressed]="voice.microphoneEnabled()" [attr.aria-label]="voice.microphoneEnabled() ? 'Выключить микрофон' : 'Включить микрофон'" (click)="toggleMic()"><img [src]="voice.microphoneEnabled() ? '/microphone-on.svg' : '/microphone-off.svg'" alt=""></button>
            <button class="voice-room__icon-button" type="button" [disabled]="pending() || voice.status() !== 'connected'" aria-label="Настройки устройств" [attr.aria-expanded]="deviceMenuOpen()" (click)="toggleDeviceMenu()"><img src="/settings-icon.svg" alt=""></button>
            <div class="screen-share-control">
              <button class="voice-room__icon-button" type="button" aria-label="Выбрать качество демонстрации" [attr.aria-expanded]="screenShareMenuOpen()" [disabled]="voice.screenShare.isTransitioning() || voice.status() !== 'connected'" (click)="screenShareMenuOpen.update((open) => !open)"><img src="/screen-share.svg" alt=""></button>
              @if (screenShareMenuOpen()) { <div class="screen-share-mode-menu" role="menu">@for (mode of screenShareModes; track mode) { <button type="button" role="menuitemradio" [attr.aria-checked]="voice.screenShare.selectedMode() === mode" (click)="selectScreenShareMode(mode)">{{ screenShareLabels[mode] }}</button> } <label class="screen-share-mode-menu__audio"><input type="checkbox" [checked]="voice.screenShare.includeSystemAudio()" [disabled]="voice.screenShare.isActive()" (change)="voice.screenShare.includeSystemAudio.set($any($event.target).checked)">Демонстрировать звук</label> @if (voice.screenShare.isActive()) { <button type="button" role="menuitem" (click)="stopScreenShare()">Остановить показ</button> }</div> }
            </div>
            @if (deviceMenuOpen()) {
              <div class="voice-device-menu">
                <label>Микрофон<select [value]="preferences.audio().inputDeviceId" (change)="selectInput($any($event.target).value)"><option value="">Системный микрофон</option>@for (device of voice.inputDevices(); track device.deviceId) { <option [value]="device.deviceId">{{ device.label || 'Микрофон' }}</option> }</select></label>
                <label>Динамики<select [value]="preferences.audio().outputDeviceId" (change)="selectOutput($any($event.target).value)"><option value="">Системные динамики</option>@for (device of voice.outputDevices(); track device.deviceId) { <option [value]="device.deviceId">{{ device.label || 'Динамики' }}</option> }</select></label>
                <label>Громкость<input type="range" min="0" max="100" [value]="preferences.audio().outputVolume" (input)="voice.setOutputVolume(+$any($event.target).value)"></label>
                <button type="button" (click)="testMicrophone()">Проверить микрофон</button>
                <label class="voice-device-menu__check"><input type="checkbox" disabled>Шумоподавление недоступно</label>
              </div>
            }
          </section>
          @if (voice.screenShare.error(); as screenError) { <p class="screen-share-error" role="alert">{{ screenError.message }}</p> }
        </div>
      </section>
    } @else {
      <section class="voice-connect" aria-live="polite" aria-busy="true">
        <p class="voice-connect__copy">
          <strong>Секунду</strong>
          <span aria-hidden="true">
            <i></i><i></i><i></i>
          </span>
        </p>
        <small>{{ temporaryCode() ? 'Входим во временную комнату' : 'Подключаем голосовую комнату' }}</small>
      </section>
    }
  `,
})
export class VoiceComponent {
  readonly temporaryCode = input('');
  readonly voiceRoomId = input('');

  protected readonly voice = inject(VoiceService);
  protected readonly preferences = inject(PreferencesService);
  private readonly groups = inject(GroupsStore);
  private readonly notifications = inject(NotificationStore);
  private readonly router = inject(Router);
  protected readonly pending = signal(false);
  protected readonly deviceMenuOpen = signal(false);
  protected readonly screenShareMenuOpen = signal(false);
  protected readonly screenShareModes: ScreenShareMode[] = ['quality', 'balanced', 'smooth'];
  protected readonly screenShareLabels = SCREEN_SHARE_MODE_LABELS;
  protected readonly screenShares = computed(() => {
    const local = this.voice.screenShare.localPreview();
    return local ? [local, ...this.voice.screenShare.remoteShares()] : this.voice.screenShare.remoteShares();
  });
  private loadedKey = '';

  protected readonly joinForm = new FormGroup({
    code: new FormControl('', { nonNullable: true, validators: [Validators.required, Validators.pattern(/^[A-HJ-NP-Z2-9]{26}$/)] }),
  });

  constructor() {
    effect(() => {
      const key = this.temporaryCode() ? `temporary:${this.temporaryCode()}` : this.voiceRoomId() ? `group:${this.voiceRoomId()}` : '';
      if (!key || key === this.loadedKey) return;
      this.loadedKey = key;
      void this.connectFromRoute();
    });
  }

  private async connectFromRoute(): Promise<void> {
    const active = this.voice.activeRoom();
    if (active && (active.id === this.temporaryCode() || active.id === this.voiceRoomId())) {
      this.voice.restore();
      return;
    }
    this.pending.set(true);
    try {
      if (this.temporaryCode()) await this.voice.openTemporary(this.temporaryCode());
      if (this.voiceRoomId()) {
        await this.groups.refresh();
        const group = this.groups.groups().find((item) => (item.voice_rooms ?? []).some((room) => room.id === this.voiceRoomId()));
        const room = group?.voice_rooms?.find((item) => item.id === this.voiceRoomId());
        await this.voice.openGroup(this.voiceRoomId(), room?.name || 'Голосовая комната', group?.id);
      }
    } catch (error) {
      this.notifications.error(error, 'Не удалось подключиться к голосовой комнате');
      await this.router.navigate(['/']);
    } finally { this.pending.set(false); }
  }

  protected async createTemporary(): Promise<void> {
    this.pending.set(true);
    try {
      const code = await this.voice.createTemporary();
      await this.router.navigate(['/temporary', code]);
    } catch (error) { this.notifications.error(error, 'Не удалось создать комнату'); }
    finally { this.pending.set(false); }
  }

  protected async joinTemporary(): Promise<void> {
    if (this.joinForm.invalid) return;
    this.pending.set(true);
    try {
      const code = this.joinForm.controls.code.value.trim().toUpperCase();
      await this.router.navigate(['/temporary', code]);
    } catch (error) { this.notifications.error(error, 'Не удалось войти в комнату'); }
    finally { this.pending.set(false); }
  }

  protected minimize(): void {
    const room = this.voice.activeRoom();
    this.voice.minimize();
    void this.router.navigate(room?.groupId ? ['/groups', room.groupId] : ['/']);
  }

  protected async leave(): Promise<void> {
    const groupId = this.voice.activeRoom()?.groupId;
    this.pending.set(true);
    try { await this.voice.leave(); await this.router.navigate(groupId ? ['/groups', groupId] : ['/']); }
    catch (error) { this.notifications.error(error, 'Не удалось корректно выйти из комнаты'); }
    finally { this.pending.set(false); }
  }

  protected async toggleMic(): Promise<void> {
    try { await this.voice.toggleMicrophone(); }
    catch (error) { this.notifications.error(error, 'Не удалось изменить состояние микрофона'); }
  }
  protected async toggleDeviceMenu(): Promise<void> {
    this.deviceMenuOpen.update((value) => !value);
    if (this.deviceMenuOpen()) await this.voice.loadDevices().catch((error) => this.notifications.error(error, 'Не удалось получить список устройств'));
  }
  protected async selectInput(id: string): Promise<void> { try { await this.voice.selectInputDevice(id); } catch (error) { this.notifications.error(error, 'Не удалось выбрать микрофон'); } }
  protected async selectOutput(id: string): Promise<void> { try { await this.voice.selectOutputDevice(id); } catch (error) { this.notifications.error(error, 'Не удалось выбрать динамики'); } }
  protected async testMicrophone(): Promise<void> { try { await this.voice.testMicrophone(); this.notifications.success('Микрофон доступен'); } catch (error) { this.notifications.error(error, 'Не удалось проверить микрофон'); } }
  protected async selectScreenShareMode(mode: ScreenShareMode): Promise<void> {
    this.screenShareMenuOpen.set(false);
    try {
      await this.voice.changeScreenShareMode(mode);
      if (this.voice.screenShare.includeSystemAudio() && !this.voice.screenShare.localPreview()?.hasAudio) {
        this.notifications.neutral('Браузер начал демонстрацию без аудиотрека. В picker нужно выбрать вкладку/экран с доступным звуком и включить системный звук.');
      }
    }
    catch (error) { this.notifications.error(error, this.voice.screenShare.error()?.message || 'Не удалось изменить режим демонстрации'); }
  }
  protected async stopScreenShare(): Promise<void> {
    this.screenShareMenuOpen.set(false);
    try { await this.voice.stopScreenShare(); }
    catch (error) { this.notifications.error(error, 'Не удалось остановить демонстрацию экрана'); }
  }
  protected async copyCode(code: string): Promise<void> { try { await navigator.clipboard.writeText(code); this.notifications.success('Код скопирован'); } catch { this.notifications.error('Не удалось скопировать код'); } }

  protected avatarColor(participant: VoiceParticipant): string {
    let hash = 0;
    for (const char of participant.name || participant.identity) hash = ((hash << 5) - hash + char.charCodeAt(0)) | 0;
    return `hsl(${Math.abs(hash) % 360} 42% 32%)`;
  }
}
