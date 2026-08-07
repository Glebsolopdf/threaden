import { signal } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { beforeEach } from 'vitest';
import { of } from 'rxjs';
import { ApiService } from '../../core/api/api.service';
import { AuthStore } from '../../core/auth/auth.store';
import { WelcomeComponent } from './welcome.component';

const user = { id: 'user-1', email: 'gleb@example.com', display_name: 'Глеб', created_at: '' };

async function mount(welcome: object) {
  TestBed.configureTestingModule({
    imports: [WelcomeComponent],
    providers: [
      { provide: ApiService, useValue: { welcome: () => of(welcome) } },
      { provide: AuthStore, useValue: { user: signal(user) } },
    ],
  });
  const fixture = TestBed.createComponent(WelcomeComponent);
  fixture.detectChanges();
  await fixture.whenStable();
  fixture.detectChanges();
  return fixture.nativeElement as HTMLElement;
}

describe('WelcomeComponent', () => {
  beforeEach(() => sessionStorage.clear());

  it('renders the logo, greeting, and activity metrics every time', async () => {
    const el = await mount({ messages: 1284, new_users: 12, new_groups: 4 });

    expect(el.querySelector('.welcome__brand img')?.getAttribute('src')).toBe('/threaden-logo.svg');
    expect(el.textContent).not.toContain('ACTIVITY BRIEF');
    expect(el.textContent).not.toContain('Другие пользователи отправили');
    expect(el.textContent).toContain('Привет, Глеб!');
    expect(el.textContent).toContain('За последние сутки в Threaden');
    expect(el.textContent).toMatch(/1[\s\u00a0]284/);
    expect(el.textContent).toContain('12');
    expect(el.textContent).toContain('4');
  });

  it('renders empty activity metrics instead of hiding the section', async () => {
    const el = await mount({ messages: 0, new_users: 0, new_groups: 0 });

    expect(el.textContent).toContain('Привет, Глеб!');
    expect(el.querySelector('[data-welcome-stats]')).not.toBeNull();
    expect(el.textContent).toContain('0');
  });
});
