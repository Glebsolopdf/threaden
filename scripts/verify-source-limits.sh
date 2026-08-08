#!/usr/bin/env bash
set -euo pipefail

root_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
source_extensions='go|ts|tsx|js|jsx|css|scss|html|sh|sql|py'
violations=0

source_files() {
  find "$root_dir" \
    \( -type d \( -name .git -o -name node_modules -o -name dist -o -name .angular -o -name output \) \) -prune -o \
    -type f -regextype posix-extended -regex ".*\.(${source_extensions})" -print | sort
}

source_directories() {
  find "$root_dir" \
    \( -type d \( -name .git -o -name node_modules -o -name dist -o -name .angular -o -name output \) \) -prune -o \
    -type d -print | sort
}

while IFS= read -r file; do
  line_count=$(wc -l <"$file")
  if ((line_count > 300)); then
    printf 'file exceeds 300 lines: %s (%d)\n' "${file#"$root_dir/"}" "$line_count"
    violations=1
  fi
done < <(source_files)

while IFS= read -r directory; do
  count=$(find "$directory" -maxdepth 1 -type f -regextype posix-extended -regex ".*\.(${source_extensions})" | wc -l)
  if ((count > 5)); then
    printf 'directory exceeds 5 source files: %s (%d)\n' "${directory#"$root_dir/"}" "$count"
    violations=1
  fi
done < <(source_directories)

if ((violations)); then
  exit 1
fi

printf '%s\n' 'source limits verified: all files <= 300 lines and directories <= 5 source files'
