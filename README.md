# crb

[![PkgGoDev](https://pkg.go.dev/badge/github.com/pgaskin/crb)](https://pkg.go.dev/github.com/pgaskin/crb)

Library and tools for parsing and exporting Chrome bookmarks.

Written against Chromium [main@{2022-08-28}](https://source.chromium.org/chromium/chromium/src/+/aabc28688acc0ba19b42ac3795febddc11a43ede:) (Chrome 106). Should work at least as far back as 2014 (Chrome 40).

```
Usage: crb [options] bookmarks_file

Options:
  -E, --export stringArray   export bookmarks HTML to the specified file (- for stdout)
  -h, --help                 show this help text
  -q, --quiet                don't write info about the bookmarks file to stderr
  -t, --tree                 write the bookmarks tree to stdout (use --verbose to show dates)
  -v, --verbose              show additional information
```

```
Usage: crb-carve [options] file[:[start_offset][:[end_offset]|+length]]...

Options:
  -h, --help                   show this help text
  -j, --json                   show information about the recovered files as JSON
  -o, --output string          write the recovered files to the specified directory
  -O, --output-format string   output file format (default "bookmarks.{input.basename}-{match.offset}.{bookmarks.checksum}.json")
  -q, --quiet                  don't show information about the recovered files

Output Fields (--output-format, --json):
  input.path                 input file path
  input.basename             input file basename
  match.offset               match offset
  match.length               match length
  bookmarks.barguid          chrome bookmarks bar folder guid
  bookmarks.checksum         chrome bookmarks checksum
  bookmarks.date.unix        most recent date (unix timestamp)
  bookmarks.date.unixmicro   most recent date (unix microscond timestamp)
  bookmarks.date.yyyymmdd    most recent data (yyyymmdd)
  bookmarks.count.folders    number of folders
  bookmarks.count.urls       number of bookmarks
  output                     output file basename (not for --output-format)
```
