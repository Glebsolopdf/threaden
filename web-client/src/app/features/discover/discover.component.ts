import { ChangeDetectionStrategy, Component, inject, signal } from '@angular/core';
import { FormControl, ReactiveFormsModule } from '@angular/forms';
import { RouterLink } from '@angular/router';
import { toSignal } from '@angular/core/rxjs-interop';
import { catchError, debounceTime, distinctUntilChanged, finalize, of, startWith, switchMap, tap } from 'rxjs';
import { ApiService } from '../../core/api/api.service';
import { NotificationStore } from '../../core/notifications/notification.store';
import { AvatarComponent } from '../../shared/avatar/avatar.component';

@Component({
  selector: 'app-discover',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [ReactiveFormsModule, RouterLink, AvatarComponent],
  template: `
    <section class="route-page" id="discover-view">
      <header class="group-header"><a class="group-header__icon" routerLink="/" aria-label="Назад"><img src="/back.svg" alt=""></a><div class="group-header__title"><strong>Поиск групп</strong><small>Публичные сообщества</small></div></header>
      <input id="discover-search" class="discover-search" [formControl]="search" placeholder="Поиск публичных групп" autocomplete="off">
      <div class="group-list">
        @if (loading()) { <div class="page-loading">Ищем группы…</div> }
        @else {
          @for (group of results(); track group.id) {
            <a class="group-row" [routerLink]="['/groups', group.id]">
              <app-avatar [src]="group.avatar" [label]="group.name" />
              <span><strong>{{ group.name }}</strong><small>{{ group.member_count }} участников · {{ group.online_count }} онлайн</small></span>
            </a>
          } @empty { <p class="empty-copy">Ничего не найдено</p> }
        }
      </div>
    </section>
  `,
})
export class DiscoverComponent {
  private readonly api = inject(ApiService);
  private readonly notifications = inject(NotificationStore);
  protected readonly search = new FormControl('', { nonNullable: true });
  protected readonly loading = signal(false);
  protected readonly results = toSignal(
    this.search.valueChanges.pipe(
      startWith(''),
      debounceTime(250),
      distinctUntilChanged(),
      switchMap((query) => {
        const normalized = query.trim();
        if (normalized.length === 1) return of([]);
        this.loading.set(true);
        return this.api.discover(normalized).pipe(
          catchError((error) => { this.notifications.error(error, 'Не удалось выполнить поиск'); return of([]); }),
          finalize(() => this.loading.set(false)),
        );
      }),
    ),
    { initialValue: [] },
  );
}
