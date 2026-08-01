import { ChangeDetectionStrategy, Component, inject, input, signal } from '@angular/core';
import { VoiceService } from '../../../core/voice/voice.service';
import type { ScreenShare } from '../../../core/voice/screen-share/screen-share.models';
import { ScreenShareStageComponent } from '../screen-share-stage/screen-share-stage.component';

@Component({
  selector: 'app-screen-share-cards',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [ScreenShareStageComponent],
  template: `
    @if (shares().length) {
      <section class="screen-share-cards" animate.enter="screen-share-cards-enter" animate.leave="screen-share-cards-leave" [class.screen-share-cards--single]="shares().length === 1" aria-label="Демонстрации экрана">
        <h3>Демонстрации</h3>
        <div class="screen-share-cards__list">
          @for (share of shares(); track share.publicationSid) {
            <article class="screen-share-card" animate.enter="screen-share-card-enter" animate.leave="screen-share-card-leave" [attr.data-screen-share-id]="share.publicationSid">
              <button class="screen-share-card__preview" type="button" [style.aspect-ratio]="previewRatio(share)" [attr.aria-label]="'Открыть демонстрацию ' + share.participantName + ' на весь экран'" (click)="openFullscreen(share.publicationSid)">
                <app-screen-share-stage [share]="share" />
              </button>
              <footer><strong>{{ share.participantName }}{{ share.isLocal ? ' (Вы)' : '' }}</strong><span>{{ share.dimensions ? share.dimensions.width + '×' + share.dimensions.height : 'Загрузка…' }}{{ share.hasAudio ? ' · звук' : ' · без звука' }}</span></footer>
              @if (!share.isLocal) {
                <button class="screen-share-card__menu-button" type="button" aria-label="Действия с демонстрацией" [attr.aria-expanded]="openMenuSid() === share.publicationSid" (click)="toggleMenu(share.publicationSid)">⋯</button>
                @if (openMenuSid() === share.publicationSid) {
                  <div class="screen-share-card__menu" role="menu">
                    @if (share.hasAudio) {
                      <button type="button" role="menuitem" (click)="toggleMute(share)">{{ isMuted(share) ? 'Включить звук' : 'Заглушить звук' }}</button>
                      <label>Громкость <input type="range" min="0" max="100" [value]="volumeFor(share)" (input)="setVolume(share, +$any($event.target).value)"></label>
                    }
                    <button type="button" role="menuitem" (click)="disableForMe(share)">Отключить демонстрацию для меня</button>
                  </div>
                }
              }
            </article>
          }
        </div>
      </section>
    }
  `,
})
export class ScreenShareCardsComponent {
  readonly shares = input.required<ScreenShare[]>();
  private readonly voice = inject(VoiceService);
  protected readonly openMenuSid = signal('');
  private readonly muted = signal(new Set<string>());
  private readonly volumes = signal(new Map<string, number>());

  protected toggleMenu(sid: string): void { this.openMenuSid.update((open) => open === sid ? '' : sid); }
  protected isMuted(share: ScreenShare): boolean { return this.muted().has(share.publicationSid); }
  protected volumeFor(share: ScreenShare): number { return this.volumes().get(share.publicationSid) ?? 100; }
  protected previewRatio(share: ScreenShare): string {
    return share.dimensions ? `${share.dimensions.width} / ${share.dimensions.height}` : '16 / 9';
  }
  protected openFullscreen(sid: string): void {
    const selector = `[data-screen-share-id="${CSS.escape(sid)}"] app-screen-share-stage`;
    void document.querySelector<HTMLElement>(selector)?.requestFullscreen?.();
  }
  protected toggleMute(share: ScreenShare): void {
    const muted = !this.isMuted(share);
    this.voice.setScreenShareAudioMuted(share.participantIdentity, muted);
    this.muted.update((items) => {
      const next = new Set(items);
      muted ? next.add(share.publicationSid) : next.delete(share.publicationSid);
      return next;
    });
  }
  protected setVolume(share: ScreenShare, volume: number): void {
    this.voice.setScreenShareAudioVolume(share.participantIdentity, volume);
    this.volumes.update((items) => new Map(items).set(share.publicationSid, volume));
  }
  protected disableForMe(share: ScreenShare): void {
    this.voice.screenShare.disableRemoteShare(share);
    this.openMenuSid.set('');
  }
}
