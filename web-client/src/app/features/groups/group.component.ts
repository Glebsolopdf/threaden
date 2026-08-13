import { ChangeDetectionStrategy, Component, computed, DestroyRef, effect, inject, input, signal } from '@angular/core';
import { FormControl, FormGroup, ReactiveFormsModule, Validators } from '@angular/forms';
import { Router, RouterLink } from '@angular/router';
import { firstValueFrom } from 'rxjs';
import { ApiService } from '../../core/api/api.service';
import type { GroupMember, GroupMessage, GroupProfile, GroupVoiceRoom } from '../../core/api/models';
import { AuthStore } from '../../core/auth/auth.store';
import { GroupsStore } from '../../core/events/groups.store';
import { NotificationStore } from '../../core/notifications/notification.store';
import { TypingStore } from '../../core/events/typing.store';
import { AvatarComponent } from '../../shared/avatar/avatar.component';
import { GroupSpamWarningsComponent } from './group-spam-warnings.component';
import { GroupMessageListComponent } from './group-message-list.component';
import { MessageComposerComponent } from './message-composer.component';
import { HistoryNoticeState } from './history/history-notice';
@Component({
  selector: 'app-group',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [ReactiveFormsModule, RouterLink, AvatarComponent, GroupSpamWarningsComponent, GroupMessageListComponent, MessageComposerComponent],
  template: `
    <section class="route-page" id="group-view">
      @if (groups.groupLoading()) {
        <div class="group-loading" aria-busy="true" aria-label="Загрузка группы">
          <div class="group-loading__identity"><span class="skeleton group-loading__avatar"></span><span class="group-loading__copy"><span class="skeleton group-loading__line group-loading__line--title"></span><span class="skeleton group-loading__line group-loading__line--meta"></span></span></div>
          <div class="group-loading__messages"><span class="skeleton group-loading__message"></span><span class="skeleton group-loading__message group-loading__message--own"></span><span class="skeleton group-loading__message group-loading__message--short"></span></div>
        </div>
      } @else if (groups.current(); as group) {
        <header class="group-header">
          <a class="group-header__icon mobile-back" routerLink="/" aria-label="Назад к группам"><img src="/back.svg" alt=""></a>
          <button class="group-header__title clickable" type="button" (click)="openProfile()">
            <strong>{{ group.name }}</strong>
            <small aria-live="polite">@if (typingLabel()) { <span class="group-header__typing">{{ typingLabel() }}</span> } @else { {{ group.member_count }} участников · {{ group.online_count }} онлайн }</small>
          </button>
          @if (voiceParticipants(group.voice_rooms ?? []) > 0) {
            <div class="group-voice-strip"><img src="/microphone-on.svg" alt=""><span>{{ voiceParticipants(group.voice_rooms ?? []) }}</span></div>
          }
          @if (groups.currentIsMember()) {
            <button class="group-header__icon" type="button" aria-label="Голосовые комнаты" (click)="voiceListOpen.set(true)"><img src="/voicetalk.svg" alt=""></button>
          }
          <div class="group-menu">
            <button class="group-header__icon" type="button" aria-label="Меню группы" [attr.aria-expanded]="menuOpen()" (click)="menuOpen.update(v => !v)">⋯</button>
            @if (menuOpen()) {
              <div class="group-menu__dropdown"><button type="button" (click)="openProfile(); menuOpen.set(false)">Настройки группы</button></div>
            }
          </div>
        </header>

        @if (group.history_visible_from && historyBannerVisible()) {
          <aside class="group-info-banner group-info-banner--private" role="status"><span class="group-info-banner__mark">⌁</span><span><strong>История до вступления скрыта</strong><small>Вы видите сообщения, появившиеся после вашего входа в эту приватную группу.</small></span></aside>
        }
        <app-group-message-list [messages]="groups.messages()" [loading]="groups.messagesLoading()" [currentUserId]="auth.user()?.id" [groupOwnerId]="group.owner.id" (reply)="beginReply($event)" (remove)="deleteMessage($event)" />

        @if (groups.currentIsMember()) {
          <app-message-composer [groupId]="groupId()" [replyingTo]="replyingTo()" (cancelReply)="cancelReply()" />
        } @else if (group.join_blocked) {
          <div class="join-group-dock group-isolated" role="status">Эта группа временно изолирована</div>
        } @else {
          <div class="join-group-dock"><button type="button" [disabled]="joining()" (click)="joinGroup()">{{ joining() ? 'Присоединяем…' : 'Присоединиться к группе' }}</button></div>
        }
      } @else {
        <div class="empty-state">Группа не найдена</div>
      }
    </section>

    @if (voiceListOpen() && groups.current(); as group) {
      <div class="dialog-backdrop" animate.leave="dialog-leave" (click)="closeBackdrop($event, voiceListOpen)">
        <section class="group-voice-dialog group-voice-dialog__panel dialog-card" role="dialog" aria-modal="true">
          <header><div><h2>{{ group.name }}: голосовые комнаты</h2><p>{{ (group.voice_rooms ?? []).length }} комнат</p></div><button type="button" aria-label="Закрыть" (click)="voiceListOpen.set(false)">×</button></header>
          <div class="group-voice__list">
            @for (room of group.voice_rooms ?? []; track room.id) {
              <article class="group-voice__row">
                <span class="group-voice__details"><strong>{{ room.name }}</strong><span>{{ room.participant_count }} участников</span></span>
                <button class="group-voice__join themed-button" type="button" [disabled]="voiceJoining()" (click)="joinVoice(room)">{{ voiceJoining() ? 'Подключаем…' : 'Присоединиться' }}</button>
                @if (groups.currentIsOwner()) { <button class="group-voice__delete" type="button" aria-label="Удалить голосовую комнату" (click)="deleteVoice(room.id)"><img src="/trash.svg" alt=""></button> }
              </article>
            } @empty { <p class="empty-copy">Голосовых комнат пока нет</p> }
          </div>
          @if (groups.currentIsOwner()) { <footer><button class="themed-button" type="button" (click)="createVoiceOpen.set(true)">Создать комнату</button></footer> }
        </section>
      </div>
    }

    @if (createVoiceOpen()) {
      <div class="dialog-backdrop" animate.leave="dialog-leave" (click)="closeBackdrop($event, createVoiceOpen)">
        <section class="dialog-card" role="dialog" aria-modal="true">
          <form [formGroup]="voiceForm" (ngSubmit)="createVoice()">
            <h2>Новая голосовая комната</h2>
            <label>Название<input type="text" formControlName="name" maxlength="80" placeholder="Например: Общий голос"></label>
            <menu><button type="button" (click)="createVoiceOpen.set(false)">Отмена</button><button class="themed-button" type="submit" [disabled]="voiceForm.invalid || voicePending()">{{ voicePending() ? 'Создаём…' : 'Создать' }}</button></menu>
          </form>
        </section>
      </div>
    }

    @if (profileOpen() && profile(); as details) {
      <div class="dialog-backdrop" animate.leave="dialog-leave" (click)="closeBackdrop($event, profileOpen)">
        <section class="group-info-dialog group-info-dialog__panel dialog-card" role="dialog" aria-modal="true">
          <header class="group-info-dialog__header">
            <app-avatar id="group-profile-avatar" [src]="details.group.avatar" [label]="details.group.name" [identity]="details.group.id" [kind]="'group'" />
            <div class="group-info-dialog__title"><h2>{{ details.group.name }}</h2><p>{{ details.group.member_count }} участников · {{ details.group.online_count }} онлайн</p></div>
            <button class="group-info-dialog__close" type="button" aria-label="Закрыть" (click)="profileOpen.set(false)">×</button>
          </header>
          <nav class="group-info-dialog__actions" aria-label="Управление группой">
            <button type="button" (click)="voiceListOpen.set(true); profileOpen.set(false)"><img src="/voicetalk.svg" alt=""><span>Голосовые комнаты</span></button>
            @if (groups.currentIsMember() && !groups.currentIsOwner()) { <button id="leave-group-button" type="button" (click)="leaveGroup()"><img src="/exit.svg" alt=""><span>Покинуть группу</span></button> }
            @if (groups.currentIsOwner()) { <button id="delete-group-button" type="button" (click)="deleteConfirmOpen.set(true)"><img src="/trash.svg" alt=""><span>Удалить группу</span></button> }
          </nav>
          <div class="group-info-dialog__body">
            @if (details.group.invite_token) {
              <div class="group-invite-row"><span>Приглашение</span><code>{{ inviteUrl(details.group.invite_token) }}</code><button type="button" aria-label="Скопировать приглашение" (click)="copyInvite(details.group.invite_token)"><img src="/copy.svg" alt=""></button></div>
            }
            <section class="group-info-dialog__section" aria-labelledby="group-members-title">
              <h3 id="group-members-title">Участники</h3>
              <ul class="group-members">
                @for (member of details.members; track member.id) {
                  <li class="group-member"><app-avatar [src]="member.avatar || ''" [label]="member.display_name" /><span><strong>{{ member.display_name }}</strong><small>{{ member.role === 'owner' ? 'Владелец' : 'Участник' }}</small></span>
                    @if (groups.currentIsOwner() && member.role !== 'owner') { <button class="group-member__remove" type="button" (click)="removeMember(member)">Удалить</button> }
                  </li>
                }
              </ul>
            </section>
            <app-group-spam-warnings [warnings]="details.spam_warnings ?? []" />
          </div>
        </section>
      </div>
    }

    @if (deleteConfirmOpen()) {
      <div class="dialog-backdrop" animate.leave="dialog-leave">
        <section class="dialog-card confirm-card" role="alertdialog" aria-modal="true"><h2>Удалить группу?</h2><p>Это действие необратимо. Будут удалены сообщения, участники и голосовые комнаты.</p><menu><button type="button" (click)="deleteConfirmOpen.set(false)">Отмена</button><button class="button button--danger" type="button" (click)="deleteGroup()">Удалить группу</button></menu></section>
      </div>
    }
  `,
})
export class GroupComponent {
  readonly groupId = input('');
  protected readonly groups = inject(GroupsStore);
  protected readonly auth = inject(AuthStore);
  private readonly api = inject(ApiService);
  private readonly notifications = inject(NotificationStore);
  protected readonly typingState = inject(TypingStore);
  private readonly router = inject(Router);
  private readonly historyNotice = inject(HistoryNoticeState);
  private readonly destroyRef = inject(DestroyRef);
  private historyTimer?: number;
  private loadedKey = '';
  protected readonly joining = signal(false);
  protected readonly menuOpen = signal(false);
  protected readonly voiceListOpen = signal(false);
  protected readonly createVoiceOpen = signal(false);
  protected readonly voicePending = signal(false);
  protected readonly voiceJoining = signal(false);
  protected readonly profileOpen = signal(false);
  protected readonly deleteConfirmOpen = signal(false);
  protected readonly historyBannerVisible = signal(false);
  protected readonly typingLabel = computed(() => this.typingState.labelFor(this.groupId(), this.auth.user()?.id));
  protected readonly replyingTo = signal<GroupMessage | null>(null);
  protected readonly profile = signal<GroupProfile | null>(null);
  protected readonly voiceForm = new FormGroup({ name: new FormControl('', { nonNullable: true, validators: [Validators.required, Validators.maxLength(80)] }) });
  constructor() {
    this.destroyRef.onDestroy(() => { if (this.historyTimer !== undefined) window.clearTimeout(this.historyTimer); });
    effect(() => {
      const key = this.groupId() ? `group:${this.groupId()}` : '';
      if (!key || key === this.loadedKey) return;
      this.loadedKey = key;
      void this.load();
    });
  }

  private async load(): Promise<void> {
    try {
      if (this.groupId()) await this.groups.openGroup(this.groupId());
      if (this.groupId() && this.groups.current()?.history_visible_from && this.historyNotice.consume(this.groupId())) this.showHistoryNotice();
    } catch (error) { this.notifications.error(error, 'Не удалось загрузить данные группы'); }
  }
  private showHistoryNotice(): void {
    this.historyBannerVisible.set(true);
    if (this.historyTimer !== undefined) window.clearTimeout(this.historyTimer);
    this.historyTimer = window.setTimeout(() => this.historyBannerVisible.set(false), 10_000);
  }
  protected voiceParticipants(rooms: GroupVoiceRoom[]): number { return rooms.reduce((sum, room) => sum + room.participant_count, 0); }
  protected beginReply(message: GroupMessage): void {
    this.replyingTo.set(message);
  }
  protected cancelReply(): void { this.replyingTo.set(null); }
  protected async deleteMessage(message: GroupMessage): Promise<void> {
    try { await this.groups.deleteMessage(message.id); }
    catch (error) { this.notifications.error(error, 'Не удалось удалить сообщение'); }
  }

  protected async joinGroup(): Promise<void> {
    const group = this.groups.current();
    if (!group || (group.join_blocked && (!group.join_blocked_until || new Date(group.join_blocked_until).getTime() > Date.now()))) return;
    this.joining.set(true);
    try {
      await this.groups.joinCurrent();
      const joined = this.groups.current();
      if (joined?.visibility === 'private') this.historyNotice.markAfterJoin(joined.id);
      await this.router.navigate(['/groups', joined?.id]);
    }
    catch (error) { this.notifications.error(error, 'Не удалось присоединиться к группе'); }
    finally { this.joining.set(false); }
  }

  protected async joinVoice(room: GroupVoiceRoom): Promise<void> {
    if (this.voiceJoining()) return;
    this.voiceJoining.set(true);
    try {
      this.voiceListOpen.set(false);
      await this.router.navigate(['/group-voice-rooms', room.id]);
    } catch (error) { this.notifications.error(error, 'Не удалось подключиться'); }
    finally { this.voiceJoining.set(false); }
  }

  protected async createVoice(): Promise<void> {
    const group = this.groups.current();
    if (!group || this.voiceForm.invalid) return;
    this.voicePending.set(true);
    try {
      await firstValueFrom(this.api.createGroupVoice(group.id, this.voiceForm.controls.name.value.trim()));
      this.voiceForm.reset({ name: '' });
      this.createVoiceOpen.set(false);
      await this.groups.refresh(true);
    } catch (error) { this.notifications.error(error, 'Не удалось создать голосовую комнату'); }
    finally { this.voicePending.set(false); }
  }

  protected async deleteVoice(id: string): Promise<void> {
    try { await firstValueFrom(this.api.deleteGroupVoice(id)); await this.groups.refresh(true); }
    catch (error) { this.notifications.error(error, 'Не удалось удалить голосовую комнату'); }
  }

  protected async openProfile(): Promise<void> {
    const group = this.groups.current();
    if (!group) return;
    this.profileOpen.set(true);
    try { this.profile.set(await firstValueFrom(this.api.groupProfile(group.id))); }
    catch (error) { this.profileOpen.set(false); this.notifications.error(error, 'Не удалось загрузить профиль группы'); }
  }

  protected async removeMember(member: GroupMember): Promise<void> {
    const group = this.groups.current();
    if (!group) return;
    try { this.profile.set(await firstValueFrom(this.api.removeGroupMember(group.id, member.id))); await this.groups.refresh(true); }
    catch (error) { this.notifications.error(error, 'Не удалось удалить участника'); }
  }

  protected async leaveGroup(): Promise<void> {
    try { await this.groups.leaveCurrent(); this.profileOpen.set(false); await this.router.navigate(['/']); }
    catch (error) { this.notifications.error(error, 'Не удалось покинуть группу'); }
  }

  protected async deleteGroup(): Promise<void> {
    try { await this.groups.deleteCurrent(); this.deleteConfirmOpen.set(false); this.profileOpen.set(false); await this.router.navigate(['/']); }
    catch (error) { this.notifications.error(error, 'Не удалось удалить группу'); }
  }

  protected inviteUrl(token: string): string { return `${location.origin}/invite/${token}`; }
  protected async copyInvite(token: string): Promise<void> {
    try { await navigator.clipboard.writeText(this.inviteUrl(token)); this.notifications.success('Приглашение скопировано'); }
    catch { this.notifications.error('Не удалось скопировать приглашение'); }
  }

  protected closeBackdrop(event: MouseEvent, state: { set(value: boolean): void }): void { if (event.target === event.currentTarget) state.set(false); }
}
