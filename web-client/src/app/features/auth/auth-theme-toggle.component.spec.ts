import { ComponentFixture, TestBed } from '@angular/core/testing';
import { PreferencesService } from '../../core/preferences/preferences.service';
import { AuthThemeToggleComponent } from './auth-theme-toggle.component';

describe('AuthThemeToggleComponent', () => {
  let fixture: ComponentFixture<AuthThemeToggleComponent>;

  beforeEach(() => {
    localStorage.clear();
    TestBed.configureTestingModule({ imports: [AuthThemeToggleComponent], providers: [PreferencesService] });
    fixture = TestBed.createComponent(AuthThemeToggleComponent);
    fixture.detectChanges();
  });

  it('toggles between dark and light themes', () => {
    const button = fixture.nativeElement.querySelector('button') as HTMLButtonElement;

    expect(button.getAttribute('aria-label')).toContain('светлую');
    button.click();
    fixture.detectChanges();
    expect(document.documentElement.dataset['theme']).toBe('light');
    expect(button.getAttribute('aria-label')).toContain('тёмную');
  });
});
