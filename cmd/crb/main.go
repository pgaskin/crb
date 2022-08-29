// Command crb parses, validates, and exports Chrome bookmark files.
package main

import (
	"fmt"
	"io"
	"os"
	"time"

	"github.com/pgaskin/crb"
	"github.com/spf13/pflag"
)

var (
	Export  = pflag.StringArrayP("export", "E", nil, "export bookmarks HTML to the specified file (- for stdout)")
	Tree    = pflag.BoolP("tree", "t", false, "write the bookmarks tree to stdout (use --verbose to show dates)")
	Verbose = pflag.BoolP("verbose", "v", false, "show additional information")
	Quiet   = pflag.BoolP("quiet", "q", false, "don't write info about the bookmarks file to stderr")
	Help    = pflag.BoolP("help", "h", false, "show this help text")
)

func main() {
	pflag.Parse()

	if pflag.NArg() != 1 || *Help {
		fmt.Printf("Usage: %s [options] bookmarks_file\n\nOptions:\n%s", os.Args[0], pflag.CommandLine.FlagUsages())
		if !*Help {
			os.Exit(2)
		}
		return
	}

	b, err := parse()
	if err != nil {
		fmt.Fprintf(os.Stderr, "fatal: %v\n", err)
		os.Exit(1)
	}

	if !*Quiet {
		info(os.Stderr, b)
	}

	if *Tree {
		tree(os.Stderr, b)
	}

	var fail bool
	for _, fn := range *Export {
		if err := export(fn, b); err != nil {
			fmt.Fprintf(os.Stderr, "error: export to %q: %v\n", fn, err)
			fail = true
		}
	}

	if fail {
		os.Exit(1)
	}
}

func parse() (*crb.Bookmarks, error) {
	var r io.Reader
	switch input := pflag.Arg(0); input {
	case "-":
		r = os.Stdin
	default:
		if f, err := os.Open(input); err == nil {
			defer f.Close()
			r = f
		} else {
			return nil, err
		}
	}

	b, valid, err := crb.Decode(r)
	if err != nil {
		return nil, fmt.Errorf("parse bookmarks: %w", err)
	}
	if !valid {
		return nil, fmt.Errorf("parse bookmarks: invalid checksum")
	}
	return b, nil
}

func info(w io.Writer, b *crb.Bookmarks) {
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
	fmt.Fprintf(w, "Version: %d\n", b.Version)
	fmt.Fprintf(w, "Folders: %d\n", cf)
	fmt.Fprintf(w, "Bookmarks: %d\n", cb)
	fmt.Fprintf(w, "Modified: %s\n", t.Time().Format(time.ANSIC))
	fmt.Fprintf(w, "Checksum: %s\n", b.Checksum)
	fmt.Fprintf(w, "Bookmarks bar GUID: %s\n", b.Roots.BookmarkBar.GUID.String())
}

func tree(w io.Writer, b *crb.Bookmarks) {
	fmt.Fprintf(w, "\n")
	b.Walk(func(n crb.BookmarkNode, parents ...string) error {
		for range parents {
			fmt.Fprintf(w, "  ")
		}
		if n.Type == crb.NodeTypeFolder {
			if *Verbose && !n.DateAdded.IsZero() {
				fmt.Fprintf(w, "\x1b[1m+ %s \x1b[90m[%s -> %s]\x1b[0m\n", n.Name, n.DateAdded.Time().Format("Jan 02 2006"), n.DateModified.Time().Format("Jan 02 2006"))
			} else {
				fmt.Fprintf(w, "\x1b[1m+ %s\x1b[0m\n", n.Name)
			}
		} else {
			if *Verbose && !n.DateAdded.IsZero() {
				fmt.Fprintf(w, "\x1b[1m-\x1b[0m %s \x1b[90m[%s]\x1b[0m\n", n.Name, n.DateAdded.Time().Format("Jan 02 2006"))
			} else {
				fmt.Fprintf(w, "\x1b[1m-\x1b[0m %s\n", n.Name)
			}
			for range parents {
				fmt.Fprintf(w, "  ")
			}
			fmt.Fprintf(w, "\x1b[90m  %s\x1b[0m\n", n.URL)
		}
		return nil
	})
	fmt.Fprintf(w, "\n")
}

func export(fn string, b *crb.Bookmarks) (rerr error) {
	var w interface {
		io.Writer
		Sync() error
	}
	switch fn {
	case "-":
		w = os.Stdout
	default:
		if f, err := os.OpenFile(fn, os.O_TRUNC|os.O_CREATE|os.O_WRONLY, 0666); err == nil {
			defer func() {
				if rerr == nil {
					if err := f.Sync(); err != nil {
						rerr = err
					}
					if err := f.Close(); err != nil {
						rerr = err
					}
				}
			}()
			w = f
		} else {
			return err
		}
	}
	if err := crb.Export(w, b, nil); err != nil {
		return err
	}
	return nil
}
