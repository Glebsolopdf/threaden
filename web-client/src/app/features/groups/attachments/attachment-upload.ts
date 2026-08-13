export const MAX_FILES = 3;
export const MAX_MEDIA_BYTES = 10 * 1024 * 1024;
export const MAX_ARCHIVE_BYTES = 5 * 1024 * 1024;

const archiveExtensions = /\.(zip|7z|rar|tar|gz|bz2|xz|tgz|tbz2|txz)$/i;

export function validateSelection(files: File[]): string | null {
  if (files.length > MAX_FILES) return 'Можно выбрать не более 3 файлов';
  for (const file of files) {
    const limit = archiveExtensions.test(file.name) ? MAX_ARCHIVE_BYTES : MAX_MEDIA_BYTES;
    if (file.size <= 0) return `Файл «${file.name}» пустой`;
    if (file.size > limit) return `Файл «${file.name}» превышает допустимый размер`;
  }
  return null;
}

export function formatBytes(size: number): string {
  if (size < 1024) return `${size} Б`;
  if (size < 1024 * 1024) return `${Math.ceil(size / 1024)} КБ`;
  return `${(size / (1024 * 1024)).toFixed(1)} МБ`;
}
