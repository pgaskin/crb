package crb

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
)

type CarveMatchFunc func(off int64, buf []byte, obj *Bookmarks) error

// Carve attempts to recover valid Chrome bookmarks from r, which could be a
// disk image or something similar. It stops if ErrBreak or another error is
// returned.
func Carve(f io.ReaderAt, fn CarveMatchFunc) error {
	const (
		BufferSize = 8192
		MaxSize    = 20 * 1024 * 1024
	)

	var (
		s1 = []byte("{\n   \"checksum\": \"")
		s2 = []byte("   \"roots\": {\n      \"bookmark_bar\": {")
	)

	// note: we can do it this way without backtracking or additional
	// buffering since the s1[0] isn't in s1[1:]

	r := bufio.NewReaderSize(io.NewSectionReader(f, 0, 1<<63-1), BufferSize)
	jb := make(json.RawMessage, MaxSize)

	var off int64
	for {
		var nm bool
		for _, x := range s1 {
			c, err := r.ReadByte()
			if err != nil {
				if err == io.EOF {
					err = nil
				}
				return err
			}
			off++

			if c != x {
				nm = true
				break
			}
		}
		if nm {
			continue
		}

		sr := io.NewSectionReader(f, off, MaxSize)

		sb := make([]byte, 1024)
		if n, err := sr.Read(sb); err != nil {
			return err
		} else {
			sb = sb[:n]
		}
		if !bytes.Contains(sb, s2) {
			continue
		}

		// attempt to read the json bytes and ensure it's actually json at the same time
		if err := json.NewDecoder(io.MultiReader(
			bytes.NewReader(s1),
			bytes.NewReader(sb),
			sr,
		)).Decode(&jb); err != nil {
			continue
		}

		obj, valid, err := Decode(bytes.NewReader(jb))
		if err != nil || !valid {
			continue
		}

		if fn != nil {
			if err := fn(off-int64(len(s1)), []byte(jb), obj); err != nil {
				if err == ErrBreak {
					err = nil
				}
				return err
			}
		}
	}
}
