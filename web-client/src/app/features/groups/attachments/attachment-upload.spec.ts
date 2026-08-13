import { describe, expect, it } from 'vitest';
import { MAX_FILES, validateSelection } from './attachment-upload';

const file = (name: string, size: number) => new File([new Uint8Array(size)], name);

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
