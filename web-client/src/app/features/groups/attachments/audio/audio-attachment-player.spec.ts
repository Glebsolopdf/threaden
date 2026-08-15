import { TestBed } from '@angular/core/testing';
import { describe, expect, it, vi } from 'vitest';
import type { MessageAttachment } from '../../../../core/api/models';
import { AudioAttachmentPlayerComponent, formatAudioTime } from './audio-attachment-player.component';

describe('formatAudioTime', () => {
  it('formats a voice message duration for the track label', () => {
    expect(formatAudioTime(0)).toBe('0:00');
    expect(formatAudioTime(74.9)).toBe('1:14');
  });
});

describe('AudioAttachmentPlayerComponent', () => {
  it('starts the native audio element from the play button', async () => {
    TestBed.configureTestingModule({ imports: [AudioAttachmentPlayerComponent] });
    const fixture = TestBed.createComponent(AudioAttachmentPlayerComponent);
    fixture.componentRef.setInput('attachment', { id: 'a1', kind: 'audio', mime: 'audio/webm', name: 'voice.webm', size: 1, url: '/voice.webm', created_at: '', expires_at: '' } satisfies MessageAttachment);
    fixture.detectChanges();
    const audio = fixture.nativeElement.querySelector('audio') as HTMLAudioElement;
    Object.defineProperty(audio, 'paused', { configurable: true, value: true });
    const play = vi.spyOn(audio, 'play').mockResolvedValue(undefined);

    (fixture.nativeElement.querySelector('.message-audio__play') as HTMLButtonElement).click();
    await fixture.whenStable();

    expect(play).toHaveBeenCalledOnce();
  });
});
