import { ChangeDetectionStrategy, Component, DestroyRef, HostListener, computed, inject, signal } from '@angular/core';
import { ReactiveFormsModule, FormControl, FormGroup, Validators } from '@angular/forms';
import { NavigationEnd, Router, RouterLink, RouterLinkActive, RouterOutlet } from '@angular/router';
import { takeUntilDestroyed, toSignal } from '@angular/core/rxjs-interop';
import { debounceTime, distinctUntilChanged, filter, startWith } from 'rxjs';
import { AuthStore } from '../../core/auth/auth.store';
import { EventStreamService } from '../../core/events/event-stream.service';
import { GroupsStore } from '../../core/events/groups.store';
import { VoiceService } from '../../core/voice/voice.service';
import { NotificationStore } from '../../core/notifications/notification.store';
import { AvatarComponent } from '../../shared/avatar/avatar.component';
import { AccountDialogComponent } from '../account/account-dialog.component';

type ShellPage = 'home' | 'group' | 'discover' | 'settings' | 'profile' | 'temporary' | 'voice';

@Component({
  selector: 'app-shell',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [RouterOutlet, RouterLink, RouterLinkActive, ReactiveFormsModule, AvatarComponent, AccountDialogComponent],
  template: `
    <main class="messenger" [attr.data-sidebar-open]="sidebarOpen()" [attr.data-page]="page()">
      <button
        class="mobile-sidebar-button"
        type="button"
        [attr.aria-label]="sidebarOpen() ? 'Закрыть навигацию' : 'Открыть навигацию'"
        [attr.aria-expanded]="sidebarOpen()"
        aria-controls="primary-navigation group-navigation"
        (click)="sidebarOpen.update(value => !value)"
      ><span></span></button>
      @if (sidebarOpen()) {
        <button class="mobile-sidebar-backdrop" type="button" aria-label="Закрыть навигацию" (click)="sidebarOpen.set(false)"></button>
      }

      <aside id="primary-navigation" class="rail" aria-label="Основная навигация">
        <a class="rail-logo" routerLink="/" aria-label="Группы"><img src="/threaden-logo.svg" alt=""></a>
        <nav>
          <a class="rail-icon" routerLink="/discover" routerLinkActive="is-active" aria-label="Поиск групп"><img src="/discover.svg" alt=""></a>
          <button class="rail-icon" type="button" aria-label="Создать группу" (click)="createDialogOpen.set(true)"><img src="/create-group-icon.svg" alt=""></button>
          <button class="rail-icon" type="button" aria-label="Временные комнаты" (click)="temporaryDialogOpen.set(true)"><img src="/temporary-room-v2.svg" alt=""></button>
        </nav>
        <div class="rail-spacer"></div>
        <button class="rail-icon" type="button" aria-label="Настройки" (click)="openAccount('settings')"><img src="/settings-icon.svg" alt=""></button>
      </aside>

      <aside id="group-navigation" class="list-panel" aria-label="Список групп">
        <header class="list-panel__header">
          <div class="list-panel__heading">
            <strong>Группы</strong>
            <span>{{ groups.groups().length }}</span>
          </div>
          <label class="list-panel__search">
            <span class="visually-hidden">Фильтр групп</span>
            <input class="group-filter" [formControl]="groupFilter" placeholder="Найти группу" autocomplete="off">
          </label>
        </header>
        <div class="group-list">
          @if (groups.groupsLoading()) {
            @for (item of skeletons; track item) {
              <div class="skeleton-chat-row"><span class="skeleton skeleton-chat-row__avatar"></span><span class="skeleton-chat-row__text"><span class="skeleton skeleton-line skeleton-line--1"></span><span class="skeleton skeleton-line skeleton-line--2"></span></span></div>
            }
          } @else if (filteredGroups().length) {
            @for (group of filteredGroups(); track group.id) {
              <a class="group-row" animate.leave="list-item-leave" [routerLink]="['/groups', group.id]" routerLinkActive="is-active" (click)="sidebarOpen.set(false)">
                <app-avatar [src]="group.avatar" [label]="group.name" [identity]="group.id" [kind]="'group'" />
                <span class="group-row__copy"><strong>{{ group.name }}</strong><small>{{ group.last_message?.body || 'Нет сообщений' }}</small></span>
              </a>
            }
          } @else if (groups.groups().length) {
            <div class="list-empty"><strong>Ничего не найдено</strong><span>Измените запрос поиска.</span></div>
          } @else {
            <div class="list-empty"><strong>Групп пока нет</strong><span>Создайте группу или найдите публичную.</span></div>
          }
        </div>
        <footer class="list-panel__profile">
          <button class="profile-popover" type="button" aria-label="Открыть профиль" (click)="openAccount('profile')">
            <app-avatar [src]="auth.user()?.avatar || ''" [label]="auth.user()?.display_name || 'Профиль'" />
            <span class="profile-popover__copy"><strong class="profile-popover__name">{{ auth.user()?.display_name || 'Профиль' }}</strong><small>{{ events.status().label }}</small></span>
            <span class="connection-status" [attr.data-state]="events.status().state" [attr.aria-label]="events.status().label"><span class="connection-status__dot"></span></span>
          </button>
        </footer>
      </aside>

      <section class="workspace"><router-outlet /></section>
    </main>
    <app-account-dialog [open]="accountDialogOpen()" [initialTab]="accountTab()" (closed)="accountDialogOpen.set(false)" />

    @if (createDialogOpen()) {
      <div class="dialog-backdrop" animate.leave="dialog-leave" (click)="closeBackdrop($event, createDialogOpen)">
        <section class="dialog-card" role="dialog" aria-modal="true" aria-labelledby="create-title">
          <form [formGroup]="createGroupForm" (ngSubmit)="createGroup()">
            <header class="dialog-card__header"><div><h2 id="create-title">Новая группа</h2><p>Создайте пространство для переписки и голосовых комнат.</p></div><button class="dialog-close" type="button" aria-label="Закрыть" (click)="createDialogOpen.set(false)">×</button></header>
            <label>Название<input type="text" formControlName="name" maxlength="80" autocomplete="off" placeholder="Например, Команда проекта"></label>
            <label>Видимость<select formControlName="visibility"><option value="public">Публичная</option><option value="private">Частная</option></select></label>
            <menu><button type="button" (click)="createDialogOpen.set(false)">Отмена</button><button class="themed-button" type="submit" [disabled]="createGroupForm.invalid || createPending()">{{ createPending() ? 'Создаём…' : 'Создать' }}</button></menu>
          </form>
        </section>
      </div>
    }

    @if (temporaryDialogOpen()) {
      <div class="dialog-backdrop" animate.leave="dialog-leave" (click)="closeBackdrop($event, temporaryDialogOpen)">
        <section class="dialog-card" role="dialog" aria-modal="true" aria-labelledby="temporary-title">
          <form [formGroup]="temporaryForm" (ngSubmit)="joinTemporary()">
            <header class="dialog-card__header"><div><h2 id="temporary-title">Временная комната</h2><p>Один разговор без создания группы.</p></div><button class="dialog-close" type="button" aria-label="Закрыть" (click)="temporaryDialogOpen.set(false)">×</button></header>
            <button class="themed-button temporary-create" type="button" [disabled]="temporaryPending()" (click)="createTemporary()">Создать новую комнату</button>
            <div class="dialog-divider"><span>или войти по коду</span></div>
            <label>Код комнаты<input type="text" formControlName="code" maxlength="26" placeholder="26-значный код" autocomplete="off" spellcheck="false"></label>
            <menu><button type="button" (click)="temporaryDialogOpen.set(false)">Отмена</button><button type="submit" [disabled]="temporaryForm.invalid || temporaryPending()">Войти</button></menu>
          </form>
        </section>
      </div>
    }
  `,
})
export class ShellComponent {
  protected readonly auth = inject(AuthStore);
  protected readonly groups = inject(GroupsStore);
  protected readonly events = inject(EventStreamService);
  private readonly voice = inject(VoiceService);
  private readonly notifications = inject(NotificationStore);
  private readonly router = inject(Router);
  private readonly destroyRef = inject(DestroyRef);

  protected readonly sidebarOpen = signal(false);
  protected readonly page = signal<ShellPage>('home');
  protected readonly createDialogOpen = signal(false);
  protected readonly temporaryDialogOpen = signal(false);
  protected readonly accountDialogOpen = signal(false);
  protected readonly accountTab = signal<'profile' | 'settings' | 'security' | 'customization'>('profile');
  protected readonly createPending = signal(false);
  protected readonly temporaryPending = signal(false);
  protected readonly skeletons = Array.from({ length: 8 }, (_, index) => index);

  protected readonly groupFilter = new FormControl('', { nonNullable: true });
  private readonly filterValue = toSignal(this.groupFilter.valueChanges.pipe(startWith(''), debounceTime(100), distinctUntilChanged()), { initialValue: '' });
  protected readonly filteredGroups = computed(() => {
    const query = this.filterValue().trim().toLocaleLowerCase('ru');
    return query ? this.groups.groups().filter((group) => group.name.toLocaleLowerCase('ru').includes(query)) : this.groups.groups();
  });

  protected readonly createGroupForm = new FormGroup({
    name: new FormControl('', { nonNullable: true, validators: [Validators.required, Validators.maxLength(80)] }),
    visibility: new FormControl<'public' | 'private'>('public', { nonNullable: true }),
  });
  protected readonly temporaryForm = new FormGroup({
    code: new FormControl('', { nonNullable: true, validators: [Validators.required, Validators.pattern(/^[A-HJ-NP-Z2-9]{26}$/)] }),
  });

  constructor() {
    void this.groups.refresh().catch((error) => this.notifications.error(error, 'Не удалось загрузить список групп'));
    this.events.connect();
    this.events.messageCreated.pipe(takeUntilDestroyed(this.destroyRef)).subscribe((message) => this.groups.mergeMessage(message));
    this.events.memberEvent.pipe(takeUntilDestroyed(this.destroyRef)).subscribe((event) => this.groups.addSystemMessage(event.type, event.groupID, event.member));
    this.events.profileUpdated.pipe(takeUntilDestroyed(this.destroyRef)).subscribe((profile) => { this.groups.updateProfile(profile); this.voice.updateProfile(profile); });
    this.events.refreshRequested.pipe(takeUntilDestroyed(this.destroyRef)).subscribe(() => this.groups.scheduleRefresh());
    this.router.events.pipe(
      filter((event): event is NavigationEnd => event instanceof NavigationEnd),
      takeUntilDestroyed(this.destroyRef),
    ).subscribe((event) => {
      this.page.set(this.pageFromUrl(event.urlAfterRedirects));
      this.sidebarOpen.set(false);
    });
    this.page.set(this.pageFromUrl(this.router.url));
    this.initViewportHeight();
    this.destroyRef.onDestroy(() => {
      this.events.disconnect();
      this.teardownViewportHeight();
    });
    void this.voice.restoreSaved(false);
  }

  @HostListener('document:keydown.escape')
  protected closeTransientUi(): void {
    if (this.createDialogOpen()) this.createDialogOpen.set(false);
    else if (this.temporaryDialogOpen()) this.temporaryDialogOpen.set(false);
    else if (this.accountDialogOpen()) this.accountDialogOpen.set(false);
    else this.sidebarOpen.set(false);
  }

  protected async createGroup(): Promise<void> {
    if (this.createGroupForm.invalid) return;
    this.createPending.set(true);
    try {
      const value = this.createGroupForm.getRawValue();
      const group = await this.groups.createGroup(value.name, value.visibility);
      this.createDialogOpen.set(false);
      this.createGroupForm.reset({ name: '', visibility: 'public' });
      await this.router.navigate(['/groups', group.id]);
    } catch (error) {
      this.notifications.error(error, 'Не удалось создать группу');
    } finally { this.createPending.set(false); }
  }

  protected async createTemporary(): Promise<void> {
    this.temporaryPending.set(true);
    try {
      const code = await this.voice.createTemporary();
      this.temporaryDialogOpen.set(false);
      await this.router.navigate(['/temporary', code]);
    } catch (error) {
      this.notifications.error(error, 'Не удалось создать комнату');
    } finally { this.temporaryPending.set(false); }
  }

  protected async joinTemporary(): Promise<void> {
    if (this.temporaryForm.invalid) return;
    this.temporaryPending.set(true);
    try {
      const code = this.temporaryForm.controls.code.value.trim().toUpperCase();
      this.temporaryDialogOpen.set(false);
      await this.router.navigate(['/temporary', code]);
    } catch (error) {
      this.notifications.error(error, 'Не удалось войти в комнату');
    } finally { this.temporaryPending.set(false); }
  }

  protected closeBackdrop(event: MouseEvent, state: { set(value: boolean): void }): void {
    if (event.target === event.currentTarget) state.set(false);
  }

  protected openAccount(tab: 'profile' | 'settings' | 'security' | 'customization'): void {
    this.accountTab.set(tab);
    this.accountDialogOpen.set(true);
    this.sidebarOpen.set(false);
  }

  private pageFromUrl(url: string): ShellPage {
    const path = url.split(/[?#]/, 1)[0];
    if (path.startsWith('/group-voice-rooms/') || /^\/temporary\/[A-HJ-NP-Z2-9]{26}$/.test(path)) return 'voice';
    if (path === '/temporary') return 'temporary';
    if (path.startsWith('/groups/') || path.startsWith('/invite/')) return 'group';
    if (path === '/discover') return 'discover';
    if (path === '/settings') return 'settings';
    if (path === '/profile') return 'profile';
    return 'home';
  }

  private readonly handleViewportChange = (): void => this.updateViewportHeight();
  private readonly handleOrientationChange = (): void => this.updateViewportHeight();

  private initViewportHeight(): void {
    if (typeof window === 'undefined') return;
    this.updateViewportHeight();
    window.addEventListener('resize', this.handleViewportChange, { passive: true });
    window.addEventListener('orientationchange', this.handleOrientationChange, { passive: true });
    window.visualViewport?.addEventListener('resize', this.handleViewportChange, { passive: true });
    window.visualViewport?.addEventListener('scroll', this.handleViewportChange, { passive: true });
  }

  private teardownViewportHeight(): void {
    if (typeof window === 'undefined') return;
    window.removeEventListener('resize', this.handleViewportChange);
    window.removeEventListener('orientationchange', this.handleOrientationChange);
    window.visualViewport?.removeEventListener('resize', this.handleViewportChange);
    window.visualViewport?.removeEventListener('scroll', this.handleViewportChange);
  }

  private updateViewportHeight(): void {
    if (typeof window === 'undefined') return;
    const height = Math.round(window.visualViewport?.height || window.innerHeight);
    document.documentElement.style.setProperty('--app-height', `${height}px`);
  }
}
