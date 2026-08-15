import { describe, expect, it } from 'vitest';
import { attachmentKind, canSendMessage, MAX_FILES, validateSelection } from './attachment-upload';

const file = (name: string, size: number, type = '') => new File([new Uint8Array(size)], name, { type });

describe('validateSelection', () => {
  it('rejects more than three files', () => {
    expect(validateSelection(Array.from({ length: MAX_FILES + 1 }, (_, i) => file(`${i}.jpg`, 1)))).toContain('3');
  });

  it('uses the archive candidate limit for archive extensions', () => {
    expect(validateSelection([file('backup.any.zip', 5 * 1024 * 1024 + 1)])).toContain('превышает');
  });

  it('accepts a caption-free selection', () => {
    expect(validateSelection([file('photo.jpg', 1024)])).toBeNull();
  });
});

describe('attachmentKind', () => {
  it('classifies previews without trusting only the filename', () => {
    expect(attachmentKind(file('photo.unknown', 1, 'image/jpeg'))).toBe('image');
    expect(attachmentKind(file('clip.bin', 1, 'video/mp4'))).toBe('video');
    expect(attachmentKind(file('voice.bin', 1, 'audio/webm'))).toBe('audio');
    expect(attachmentKind(file('voice.m4a', 1))).toBe('audio');
    expect(attachmentKind(file('backup.data', 1, 'application/zip'))).toBe('archive');
    expect(attachmentKind(file('notes.txt', 1, 'text/plain'))).toBe('file');
  });
});

describe('canSendMessage', () => {
  it.each([
    ['text only', 'hello', 0, true],
    ['text and attachment', 'hello', 1, true],
    ['attachment only', '', 1, true],
    ['whitespace and attachment', '  ', 1, true],
    ['empty', '', 0, false],
  ])('%s', (_name, body, attachments, expected) => {
    expect(canSendMessage(body, attachments)).toBe(expected);
  });
});
