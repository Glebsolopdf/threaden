import { PreferencesService } from './preferences.service';

describe('PreferencesService', () => {
  beforeEach(() => {
    localStorage.clear();
    document.documentElement.removeAttribute('data-theme');
  });

  it('defaults to dark and persists a changed theme', () => {
    const service = new PreferencesService();

    expect(service.theme()).toBe('dark');
    service.setTheme('light');

    expect(service.theme()).toBe('light');
    expect(document.documentElement.dataset['theme']).toBe('light');
    expect(JSON.parse(localStorage.getItem('voice_rooms_theme') ?? 'null')).toBe('light');
  });

  it('restores a stored light theme', () => {
    localStorage.setItem('voice_rooms_theme', JSON.stringify('light'));

    const service = new PreferencesService();

    expect(service.theme()).toBe('light');
    expect(document.documentElement.dataset['theme']).toBe('light');
  });
});
