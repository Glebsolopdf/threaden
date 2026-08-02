import { ChangeDetectionStrategy, Component, ElementRef, computed, effect, inject, input, signal, viewChild } from '@angular/core';
import { FormControl, FormGroup, ReactiveFormsModule, Validators } from '@angular/forms';
import { Router, RouterLink } from '@angular/router';
import { firstValueFrom } from 'rxjs';
import { ApiService } from '../../core/api/api.service';
import type { GroupMember, GroupProfile, GroupVoiceRoom } from '../../core/api/models';
import { AuthStore } from '../../core/auth/auth.store';
import { chatMessage, GroupsStore, isSystemMessage, systemMessageText, type MessageView } from '../../core/events/groups.store';
import { NotificationStore } from '../../core/notifications/notification.store';
import { TypingStore } from '../../core/events/typing.store';
import { AvatarComponent } from '../../shared/avatar/avatar.component';
import { GroupSpamWarningsComponent } from './group-spam-warnings.component';
@Component({
  selector: 'app-group',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [ReactiveFormsModule, RouterLink, AvatarComponent, GroupSpamWarningsComponent],
  template: `
    <section class="route-page" id="group-view">
      @if (groups.groupLoading()) {
        <div class="page-loading">Загрузка группы…</div>
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

        <div #messageList class="message-list">
          @if (groups.messagesLoading()) {
            @for (item of skeletons; track item) {
              <div class="skeleton-message" [class.skeleton-message--own]="item % 3 === 1">
                @if (item % 3 !== 1) { <span class="skeleton skeleton-message__avatar"></span> }
                <span class="skeleton-message__bubble"><span class="skeleton skeleton-line skeleton-line--1"></span><span class="skeleton skeleton-line skeleton-line--2"></span></span>
              </div>
            }
          } @else {
            @for (item of groups.messages(); track messageId(item); let index = $index) {
              @if (chatMessage(item); as chat) {
                <article
                  class="chat-message"
                  animate.leave="message-leave"
                  [class.chat-message--own]="isOwn(chat)"
                  [class.chat-message--other]="!isOwn(chat)"
                  [attr.data-status]="chat.status"
                  [attr.data-compact]="isCompact(index)"
                  [attr.data-animate]="chat.animate || null"
                >
                  @if (!isOwn(chat)) {
                    <app-avatar class="chat-message__avatar" [src]="isCompact(index) ? '' : (chat.message.author.avatar || '')" [label]="chat.message.author.display_name" />
                  }
                  <div class="chat-message__bubble">
                    @if (!isOwn(chat) && !isCompact(index)) { <strong class="chat-message__author">{{ chat.message.author.display_name }}</strong> }
                    <p>{{ chat.message.body }}</p>
                    <footer><time [attr.datetime]="chat.message.created_at">{{ formatTime(chat.message.created_at) }}</time>{{ statusSuffix(chat) }}</footer>
                  </div>
                </article>
              } @else {
                <article class="system-message" animate.leave="message-leave" [attr.data-animate]="item.animate"><span>{{ systemMessageText(item) }}</span></article>
              }
            } @empty { <p class="empty-copy">Сообщений пока нет</p> }
          }
        </div>

        @if (groups.currentIsMember()) {
          <form class="composer" [formGroup]="messageForm" (ngSubmit)="sendMessage()" autocomplete="off">
            <input formControlName="body" maxlength="2000" placeholder="Сообщение" autocomplete="off" spellcheck="true" (input)="typingState.notify(groupId(), messageForm.controls.body.value.trim().length > 0)">
            <button type="submit" [disabled]="messageForm.invalid || sending()">{{ sending() ? 'Отправка…' : 'Отправить' }}</button>
          </form>
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
  readonly inviteToken = input('');
  protected readonly groups = inject(GroupsStore);
  protected readonly auth = inject(AuthStore);
  private readonly api = inject(ApiService);
  private readonly notifications = inject(NotificationStore);
  protected readonly typingState = inject(TypingStore);
  private readonly router = inject(Router);
  private readonly messageList = viewChild<ElementRef<HTMLElement>>('messageList');
  private loadedKey = '';
  protected readonly skeletons = Array.from({ length: 7 }, (_, index) => index);
  protected readonly sending = signal(false);
  protected readonly joining = signal(false);
  protected readonly menuOpen = signal(false);
  protected readonly voiceListOpen = signal(false);
  protected readonly createVoiceOpen = signal(false);
  protected readonly voicePending = signal(false);
  protected readonly voiceJoining = signal(false);
  protected readonly profileOpen = signal(false);
  protected readonly deleteConfirmOpen = signal(false);
  protected readonly typingLabel = computed(() => this.typingState.labelFor(this.groupId(), this.auth.user()?.id));
  protected readonly profile = signal<GroupProfile | null>(null);
  protected readonly messageForm = new FormGroup({ body: new FormControl('', { nonNullable: true, validators: [Validators.required, Validators.maxLength(2000)] }) });
  protected readonly voiceForm = new FormGroup({ name: new FormControl('', { nonNullable: true, validators: [Validators.required, Validators.maxLength(80)] }) });
  constructor() {
    effect(() => {
      const key = this.groupId() ? `group:${this.groupId()}` : this.inviteToken() ? `invite:${this.inviteToken()}` : '';
      if (!key || key === this.loadedKey) return;
      this.loadedKey = key;
      void this.load();
    });
    effect(() => {
      this.groups.messages();
      window.requestAnimationFrame(() => {
        const element = this.messageList()?.nativeElement;
        if (element) element.scrollTop = element.scrollHeight;
      });
    });
  }

  private async load(): Promise<void> {
    try {
      if (this.groupId()) await this.groups.openGroup(this.groupId());
      else if (this.inviteToken()) await this.groups.openInvite(this.inviteToken());
    } catch (error) { this.notifications.error(error, 'Не удалось загрузить данные группы'); }
  }
  protected readonly chatMessage = chatMessage;
  protected readonly systemMessageText = systemMessageText;
  protected messageId(item: MessageView): string { return isSystemMessage(item) ? item.id : item.viewID ?? item.message.id; }
  protected isOwn(item: MessageView): boolean { return !isSystemMessage(item) && item.message.author.id === this.auth.user()?.id; }
  protected isCompact(index: number): boolean {
    const items = this.groups.messages();
    const previous = items[index - 1];
    const current = items[index];
    return Boolean(previous && current && !isSystemMessage(previous) && !isSystemMessage(current) && previous.message.author.id === current.message.author.id);
  }
  protected formatTime(value: string): string { return new Date(value).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }); }
  protected statusSuffix(item: MessageView): string {
    return isSystemMessage(item) ? '' : item.status === 'sending' ? ' · отправка' : item.status === 'error' ? ' · ошибка' : item.message.edited_at ? ' · изменено' : '';
  }
  protected voiceParticipants(rooms: GroupVoiceRoom[]): number { return rooms.reduce((sum, room) => sum + room.participant_count, 0); }
  protected async sendMessage(): Promise<void> {
    if (this.messageForm.invalid || this.sending()) return;
    const body = this.messageForm.controls.body.value;
    this.messageForm.reset({ body: '' }); this.typingState.notify(this.groupId(), false);
    this.sending.set(true);
    try { await this.groups.sendMessage(body); }
    catch (error) { this.messageForm.setValue({ body }); this.typingState.notify(this.groupId(), body.trim().length > 0); this.notifications.error(error, 'Не удалось отправить сообщение'); }
    finally { this.sending.set(false); }
  }

  protected async joinGroup(): Promise<void> {
    this.joining.set(true);
    try { await this.groups.joinCurrent(this.inviteToken()); await this.router.navigate(['/groups', this.groups.current()?.id]); }
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
