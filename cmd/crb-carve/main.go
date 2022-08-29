// Command crb-carve attempts to recover Chrome bookmark files from a file.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/pgaskin/crb"
	"github.com/spf13/pflag"
)

var (
	Arg          = regexp.MustCompile(`^(.+?)(?:[:]([0-9]*)(?:[:]([0-9]*)|[+]([0-9]*))?)?$`) // path, start_offset, end_offset | length
	Output       = pflag.StringP("output", "o", "", "write the recovered files to the specified directory")
	OutputFormat = pflag.StringP("output-format", "O", "bookmarks.{input.basename}-{match.offset}.{bookmarks.checksum}.json", "output file format")
	Quiet        = pflag.BoolP("quiet", "q", false, "don't show information about the recovered files")
	JSON         = pflag.BoolP("json", "j", false, "show information about the recovered files as JSON")
	Help         = pflag.BoolP("help", "h", false, "show this help text")
)

var fnCharRe = regexp.MustCompile(`[^a-zA-Z0-9._ {}-]+|^ | $`)

func main() {
	pflag.Parse()

	if pflag.NArg() < 1 || *Help {
		fmt.Printf("Usage: %s [options] file[:[start_offset][:[end_offset]|+length]]...\n\nOptions:\n%s", os.Args[0], pflag.CommandLine.FlagUsages())
		fmt.Printf("\nOutput Fields (--output-format, --json):\n")
		fmt.Printf("  %-24s   %s\n", "input.path", "input file path")
		fmt.Printf("  %-24s   %s\n", "input.basename", "input file basename")
		fmt.Printf("  %-24s   %s\n", "match.offset", "match offset")
		fmt.Printf("  %-24s   %s\n", "match.length", "match length")
		fmt.Printf("  %-24s   %s\n", "bookmarks.barguid", "chrome bookmarks bar folder guid")
		fmt.Printf("  %-24s   %s\n", "bookmarks.checksum", "chrome bookmarks checksum")
		fmt.Printf("  %-24s   %s\n", "bookmarks.date.unix", "most recent date (unix timestamp)")
		fmt.Printf("  %-24s   %s\n", "bookmarks.date.unixmicro", "most recent date (unix microscond timestamp)")
		fmt.Printf("  %-24s   %s\n", "bookmarks.date.yyyymmdd", "most recent data (yyyymmdd)")
		fmt.Printf("  %-24s   %s\n", "bookmarks.count.folders", "number of folders")
		fmt.Printf("  %-24s   %s\n", "bookmarks.count.urls", "number of bookmarks")
		fmt.Printf("  %-24s   %s\n", "output", "output file basename (not for --output-format)")
		if !*Help {
			os.Exit(2)
		}
		return
	}

	if *OutputFormat == "" {
		fmt.Fprintf(os.Stderr, "fatal: output format is empty\n")
		os.Exit(2)
	}
	if fnCharRe.MatchString(*OutputFormat) {
		fmt.Fprintf(os.Stderr, "fatal: output format contains invalid characters\n")
		os.Exit(2)
	}

	if *Output != "" {
		if err := os.MkdirAll(*Output, 0777); err != nil {
			fmt.Fprintf(os.Stderr, "fatal: failed to create output dir: %v\n", err)
			os.Exit(1)
		}
	}

	var iPath []string
	var iOff []int64
	var iLen []int64
	for _, path := range pflag.Args() {
		var offset, length int64
		if m := Arg.FindStringSubmatch(path); m != nil {
			path = m[1]
			if v := m[2]; v != "" {
				offset, _ = strconv.ParseInt(v, 10, 64)
			}
			if v := m[3]; v != "" {
				length, _ = strconv.ParseInt(v, 10, 64)
				length -= offset
			}
			if v := m[4]; v != "" {
				length, _ = strconv.ParseInt(v, 10, 64)
			}
			if length < 0 {
				fmt.Fprintf(os.Stderr, "fatal: invalid slice for %q: length <= 0 (did you mean to use '+' instead of ':'?)\n", path)
				os.Exit(2)
			}
			if length == 0 {
				length = 1<<63 - 1
			}
		}
		iPath = append(iPath, path)
		iOff = append(iOff, offset)
		iLen = append(iLen, length)
	}

	var fail bool
	for i := range iPath {
		if err := carve(iPath[i], iOff[i], iLen[i]); err != nil {
			fmt.Fprintf(os.Stderr, "error: failed to carve %q: %v\n", iPath[i], err)
			fail = true
		}
	}
	if fail {
		os.Exit(1)
	}
}

func carve(path string, offset, length int64) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	r := io.NewSectionReader(f, offset, length)

	return crb.Carve(r, func(off int64, buf []byte, b *crb.Bookmarks) error {
		var t crb.Time
		var cf, cb int
		b.Walk(func(n crb.BookmarkNode, parents ...string) error {
			switch n.Type {
			case crb.NodeTypeFolder:
				cf++
			case crb.NodeTypeURL:
				cb++
			}
			if v := n.DateAdded; v > t {
				t = v
			}
			if v := n.DateLastUsed; v > t {
				t = v
			}
			if v := n.DateModified; v > t {
				t = v
			}
			return nil
		})

		var m struct {
			Input struct {
				Path     string `json:"path"`
				Basename string `json:"basename"`
			} `json:"input"`
			Match struct {
				Offset int64 `json:"offset"`
				Length int64 `json:"length"`
			} `json:"match"`
			Bookmarks struct {
				BarGUID  string `json:"barguid"`
				Checksum string `json:"checksum"`
				Date     struct {
					Unix      int64  `json:"unix"`
					UnixMicro int64  `json:"unixmicro"`
					YYYYMMDD  string `json:"yyyymmdd"`
				} `json:"date"`
				Count struct {
					Folder int `json:"folders"`
					URL    int `json:"urls"`
				} `json:"count"`
			} `json:"bookmarks"`
			Output string `json:"output,omitempty"`
		}

		m.Input.Path = path
		m.Input.Basename = filepath.Base(path)
		m.Match.Offset = off + offset
		m.Match.Length = int64(len(buf))
		m.Bookmarks.BarGUID = b.Roots.BookmarkBar.GUID.String()
		m.Bookmarks.Checksum = b.Checksum
		m.Bookmarks.Date.Unix = t.Unix()
		m.Bookmarks.Date.UnixMicro = t.UnixMicro()
		m.Bookmarks.Date.YYYYMMDD = t.Time().Format("20060102")
		m.Bookmarks.Count.Folder = cf
		m.Bookmarks.Count.URL = cb

		if *Output != "" {
			m.Output = strings.NewReplacer(
				"{input.path}", fnCharRe.ReplaceAllLiteralString(m.Input.Path, "_"),
				"{input.basename}", fnCharRe.ReplaceAllLiteralString(m.Input.Basename, "_"),
				"{match.offset}", strconv.FormatInt(m.Match.Offset, 10),
				"{match.length}", strconv.FormatInt(m.Match.Length, 10),
				"{bookmarks.barguid}", m.Bookmarks.BarGUID,
				"{bookmarks.checksum}", m.Bookmarks.Checksum,
				"{bookmarks.date.unix}", strconv.FormatInt(m.Bookmarks.Date.Unix, 10),
				"{bookmarks.date.unixmicro}", strconv.FormatInt(m.Bookmarks.Date.UnixMicro, 10),
				"{bookmarks.date.yyyymmdd}", m.Bookmarks.Date.YYYYMMDD,
				"{bookmarks.count.folders}", strconv.Itoa(m.Bookmarks.Count.Folder),
				"{bookmarks.count.urls}", strconv.Itoa(m.Bookmarks.Count.URL),
			).Replace(*OutputFormat)
		}

		if !*Quiet {
			if *JSON {
				enc := json.NewEncoder(os.Stdout)
				enc.SetEscapeHTML(false)
				enc.Encode(m)
			} else {
				var o string
				if m.Output != "" {
					o = " -> " + m.Output
				}
				fmt.Fprintf(os.Stdout, "%s:%d+%d [%s @ %s] %s (%d,%d)%s\n", m.Input.Path, m.Match.Offset, m.Match.Length, m.Bookmarks.BarGUID, t.Time().Format("02 Jan 06 15:04 MST"), m.Bookmarks.Checksum, m.Bookmarks.Count.Folder, m.Bookmarks.Count.URL, o)
			}
		}

		if *Output != "" {
			if err := os.WriteFile(filepath.Join(*Output, m.Output), buf, 0666); err != nil {
				return fmt.Errorf("write output: %w", err)
			}
		}

		return nil
	})
}
