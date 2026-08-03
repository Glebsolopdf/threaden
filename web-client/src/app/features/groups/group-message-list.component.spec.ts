import { TestBed } from '@angular/core/testing';
import type { MessageView } from '../../core/events/groups.store';
import { GroupMessageListComponent } from './group-message-list.component';

const authorA = { id: 'user-1', email: 'a@example.com', display_name: 'Аня', created_at: '' };
const authorB = { id: 'user-2', email: 'b@example.com', display_name: 'Боря', created_at: '' };

function chat(body: string, extra: Partial<{ author: typeof authorA; read: boolean; edited_at: string; status: 'sending' | 'sent' | 'error' }> = {}): MessageView {
  return {
    message: {
      id: 'msg_' + body.length,
      group_id: 'group-1',
      author: extra.author ?? authorA,
      body,
      created_at: '2026-01-01T10:00:00Z',
      read: extra.read,
      edited_at: extra.edited_at,
    },
    status: extra.status ?? 'sent',
  };
}

async function mount(messages: MessageView[], currentUserId = authorA.id) {
  TestBed.configureTestingModule({ imports: [GroupMessageListComponent] });
  const fixture = TestBed.createComponent(GroupMessageListComponent);
  fixture.componentRef.setInput('messages', messages);
  fixture.componentRef.setInput('loading', false);
  fixture.componentRef.setInput('currentUserId', currentUserId);
  fixture.detectChanges();
  await fixture.whenStable();
  return fixture.nativeElement as HTMLElement;
}

describe('GroupMessageListComponent', () => {
  it('shows the empty copy when there are no messages', async () => {
    const el = await mount([]);
    expect(el.textContent).toContain('Сообщений пока нет');
  });

  it('marks own and other messages and hides the avatar for own', async () => {
    const el = await mount([chat('привет'), chat('как дела', { author: authorB })]);
    const articles = el.querySelectorAll('article.chat-message');
    expect(articles.length).toBe(2);
    expect(articles[0].classList.contains('chat-message--own')).toBe(true);
    expect(articles[1].classList.contains('chat-message--other')).toBe(true);
    expect(el.querySelectorAll('app-avatar').length).toBe(1);
  });

  it('shows read ticks only for own sent messages', async () => {
    const el = await mount([chat('отправлено', { read: true }), chat('не прочитано'), chat('другое', { author: authorB, read: true })]);
    const ticks = el.querySelectorAll('.message-status');
    expect(ticks.length).toBe(2);
    expect(ticks[0].getAttribute('aria-label')).toBe('Прочитано');
    expect(ticks[1].getAttribute('aria-label')).toBe('Отправлено');
  });

  it('appends status suffixes for sending, error and edited', async () => {
    const el = await mount([
      chat('a', { status: 'sending' }),
      chat('b', { status: 'error' }),
      chat('c', { edited_at: '2026-01-01T10:05:00Z' }),
    ]);
    const footer = el.querySelectorAll('.chat-message__bubble footer');
    expect(footer[0].textContent).toContain('· отправка');
    expect(footer[1].textContent).toContain('· ошибка');
    expect(footer[2].textContent).toContain('· изменено');
  });

  it('renders system messages as separate articles', async () => {
    const el = await mount([
      chat('привет'),
      { kind: 'system', id: 'system-1', body: 'Из чата исключён участник: Глеб', animate: 'incoming' },
    ]);
    const system = el.querySelectorAll('article.system-message');
    expect(system.length).toBe(1);
    expect(system[0].textContent).toContain('Глеб');
  });
});
