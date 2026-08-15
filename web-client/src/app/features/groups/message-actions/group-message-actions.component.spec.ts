import { Component } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { describe, expect, it } from 'vitest';
import type { GroupMessage } from '../../../core/api/models';
import { GroupMessageActionsComponent } from './group-message-actions.component';

const message: GroupMessage = {
  id: 'message-1',
  group_id: 'group-1',
  author: { id: 'user-1', email: 'user@example.com', display_name: 'User', created_at: '' },
  body: 'hello',
  created_at: '2026-01-01T10:00:00Z',
};

@Component({
  standalone: true,
  imports: [GroupMessageActionsComponent],
  template: '<div #bubble class="bubble"><app-group-message-actions [message]="message" [own]="true" /></div>',
})
class HostComponent {
  readonly message = message;
}

describe('GroupMessageActionsComponent', () => {
  it('positions the menu from the message bubble instead of pointer coordinates', async () => {
    TestBed.configureTestingModule({ imports: [HostComponent] });
    const fixture = TestBed.createComponent(HostComponent);
    fixture.detectChanges();
    const bubble = fixture.nativeElement.querySelector('.bubble') as HTMLElement;
    Object.defineProperty(bubble, 'getBoundingClientRect', { value: () => ({ top: 200, bottom: 260, left: 120, right: 420, width: 300, height: 60 }) });
    const actions = fixture.nativeElement.querySelector('app-group-message-actions') as HTMLElement;
    actions.dispatchEvent(new MouseEvent('contextmenu', { bubbles: true, clientX: 3, clientY: 580 }));
    fixture.detectChanges();
    const menu = fixture.nativeElement.querySelector('.message-actions__menu') as HTMLElement;
    Object.defineProperty(menu, 'getBoundingClientRect', { value: () => ({ top: 0, bottom: 80, left: 0, right: 140, width: 140, height: 80 }) });
    await new Promise((resolve) => setTimeout(resolve, 100));
    fixture.detectChanges();
    expect(menu.style.top).toBe('112px');
    expect(menu.style.left).toBe('200px');
  });
});
