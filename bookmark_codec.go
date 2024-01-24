// Package crb manipulates Chrome bookmarks.
//
// Written against Chromium aabc28688acc0ba19b42ac3795febddc11a43ede (Chrome
// 106, 2022-08-22). Should work at least as far back as 2014 (Chrome 40).
//
// See:
//   - https://source.chromium.org/chromium/chromium/src/+/main:components/bookmarks/browser/bookmark_codec.cc;drc=aabc28688acc0ba19b42ac3795febddc11a43ede
//   - https://source.chromium.org/chromium/chromium/src/+/main:chrome/browser/bookmarks/bookmark_html_writer.cc;drc=aabc28688acc0ba19b42ac3795febddc11a43ede
package crb

import (
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"time"
	"unicode/utf16"
)

type Bookmarks struct {
	Checksum string `json:"checksum"`
	Roots    struct {
		BookmarkBar    BookmarkNode `json:"bookmark_bar"`
		Other          BookmarkNode `json:"other"`
		MobileBookmark BookmarkNode `json:"synced"`
	} `json:"roots"`
	SyncMetadata     Bytes             `json:"sync_metadata,omitempty"`
	Version          Version           `json:"version"`
	MetaInfo         map[string]string `json:"meta_info,omitempty"`
	UnsyncedMetaInfo map[string]string `json:"unsynced_meta_info,omitempty"`
}

type BookmarkNode struct {
	Children         *[]BookmarkNode   `json:"children,omitempty"` // Type == NodeTypeFolder
	DateAdded        Time              `json:"date_added"`
	DateLastUsed     Time              `json:"date_last_used,omitempty"`
	DateModified     Time              `json:"date_modified,omitempty"`
	GUID             GUID              `json:"guid"`
	ID               int               `json:"id,string"`
	Name             string            `json:"name"`
	ShowIcon         bool              `json:"show_icon,omitempty"` // used by MSEdge
	Source           Source            `json:"source,omitempty"`    // used by MSEdge
	Type             NodeType          `json:"type"`
	URL              string            `json:"url,omitempty"`
	MetaInfo         map[string]string `json:"meta_info,omitempty"`
	UnsyncedMetaInfo map[string]string `json:"unsynced_meta_info,omitempty"`
}

// Decode strictly decodes a Chrome bookmarks file.
func Decode(r io.Reader) (*Bookmarks, bool, error) {
	var b Bookmarks
	d := json.NewDecoder(r)
	d.DisallowUnknownFields()
	if err := d.Decode(&b); err != nil {
		return nil, false, err
	}
	return &b, b.Checksum == b.CalculateChecksum(), nil
}

// Encode re-encodes a Chrome bookmarks file. It should be more or less the same
// as Chrome itself, but the formatting is slightly different.
func Encode(w io.Writer, b *Bookmarks) error {
	e := json.NewEncoder(w)
	e.SetIndent("", "   ")
	e.SetEscapeHTML(false)
	return e.Encode(b)
}

// Calculate calculates the expected checksum for b.
func (b Bookmarks) CalculateChecksum() string {
	h := md5.New()
	b.Walk(func(n BookmarkNode, parents ...string) error {
		switch n.Type {
		case "url":
			h.Write([]byte(strconv.Itoa(n.ID)))
			h.Write(u16string(n.Name))
			h.Write([]byte(n.Type))
			h.Write([]byte(n.URL))
		case "folder":
			h.Write([]byte(strconv.Itoa(n.ID)))
			h.Write(u16string(n.Name))
			h.Write([]byte(n.Type))
		}
		return nil
	})
	return hex.EncodeToString(h.Sum(nil))
}

var ErrBreak = errors.New("break")

type WalkFunc func(n BookmarkNode, parents ...string) error

// Walk iterates over folders and bookmarks in b depth-first, stopping if
// ErrBreak or another error is returned.
func (b Bookmarks) Walk(fn WalkFunc) error {
	if fn == nil {
		return nil
	}
	if err := b.Roots.BookmarkBar.walk(fn); err != nil {
		if err == ErrBreak {
			err = nil
		}
		return err
	}
	if err := b.Roots.Other.walk(fn); err != nil {
		if err == ErrBreak {
			err = nil
		}
		return err
	}
	if err := b.Roots.MobileBookmark.walk(fn); err != nil {
		if err == ErrBreak {
			err = nil
		}
		return err
	}
	return nil
}

// Walk iterates over folders and bookmarks in b depth-first, stopping if
// ErrBreak or another error is returned.
func (n BookmarkNode) Walk(fn WalkFunc) error {
	if fn == nil {
		return nil
	}
	if err := n.walk(fn); err != nil && err != ErrBreak {
		return err
	}
	return nil
}

func (n BookmarkNode) walk(fn WalkFunc, parents ...string) error {
	if err := fn(n, parents...); err != nil {
		return err
	}
	if n.Type == NodeTypeFolder && n.Children != nil {
		for _, c := range *n.Children {
			if err := c.walk(fn, append(parents, c.Name)...); err != nil {
				return err
			}
		}
	}
	return nil
}

type Version int

var (
	_ json.Marshaler   = Version(0)
	_ json.Unmarshaler = (*Version)(nil)
)

const CurrentVersion Version = 1

func (v Version) Valid() error {
	if v != 1 {
		return fmt.Errorf("unsupported bookmarks version %d", v)
	}
	return nil
}

func (v Version) MarshalJSON() ([]byte, error) {
	if err := v.Valid(); err != nil {
		return nil, err
	}
	return json.Marshal(int(v))
}

func (v *Version) UnmarshalJSON(b []byte) error {
	if err := json.Unmarshal(b, (*int)(v)); err != nil {
		return err
	}
	if err := v.Valid(); err != nil {
		return err
	}
	return nil
}

type NodeType string

var (
	_ json.Marshaler   = NodeType("")
	_ json.Unmarshaler = (*NodeType)(nil)
)

const (
	NodeTypeURL    NodeType = "url"
	NodeTypeFolder NodeType = "folder"
)

func (t NodeType) Valid() error {
	switch t {
	case NodeTypeURL, NodeTypeFolder:
		return nil
	default:
		return fmt.Errorf("unrecognized node type %q", string(t))
	}
}

func (t NodeType) MarshalJSON() ([]byte, error) {
	if err := t.Valid(); err != nil {
		return nil, err
	}
	return json.Marshal(string(t))
}

func (t *NodeType) UnmarshalJSON(b []byte) error {
	if err := json.Unmarshal(b, (*string)(t)); err != nil {
		return err
	}
	if err := t.Valid(); err != nil {
		return err
	}
	return nil
}

type Source string

var (
	_ json.Marshaler   = Source("")
	_ json.Unmarshaler = (*Source)(nil)
)

const (
	SourceUserAdd   Source = "user_add"
	SourceImportFre Source = "import_fre"
	SourceUnknown   Source = "unknown"
)

func (s Source) Valid() error {
	switch s {
	case SourceUserAdd, SourceImportFre, SourceUnknown:
		return nil
	default:
		return fmt.Errorf("unrecognized source %q", string(s))
	}
}

func (s Source) MarshalJSON() ([]byte, error) {
	if err := s.Valid(); err != nil {
		return nil, err
	}
	return json.Marshal(string(s))
}

func (s *Source) UnmarshalJSON(b []byte) error {
	if err := json.Unmarshal(b, (*string)(s)); err != nil {
		return err
	}
	if err := s.Valid(); err != nil {
		return err
	}
	return nil
}

type Bytes []byte

var (
	_ json.Marshaler   = Bytes{}
	_ json.Unmarshaler = (*Bytes)(nil)
)

func (c Bytes) MarshalJSON() ([]byte, error) {
	switch {
	case c == nil:
		return []byte(`null`), nil
	case len(c) == 0:
		return []byte(`""`), nil
	default:
		return json.Marshal(base64.StdEncoding.EncodeToString(c))
	}
}

func (c *Bytes) UnmarshalJSON(b []byte) error {
	var s *string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	switch {
	case s == nil:
		*c = nil
	case *s == "":
		*c = []byte{}
	default:
		if b, err := base64.StdEncoding.DecodeString(*s); err != nil {
			return err
		} else {
			*c = b
		}
	}
	return nil
}

type Time int64

var (
	_ json.Marshaler   = Time(0)
	_ json.Unmarshaler = (*Time)(nil)
)

const crTimeEpochDelta int64 = 11644473600 // seconds

func (t Time) IsZero() bool {
	return t == 0
}

func (t Time) MarshalJSON() ([]byte, error) {
	if t.IsZero() {
		return nil, nil
	}
	return json.Marshal(strconv.FormatInt(int64(t), 10))
}

func (t *Time) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	if v, err := strconv.ParseInt(s, 10, 64); err != nil {
		return err
	} else {
		*t = Time(v)
	}
	return nil
}

func (t *Time) SetTime(x time.Time) {
	*t = Time(x.UnixMicro() + (crTimeEpochDelta * 1000 * 1000))
}

func (t Time) Time() time.Time {
	if t.IsZero() {
		return time.Time{}
	}
	// microsecond time with an epoch of 1601-01-01 UTC
	return time.UnixMicro(t.UnixMicro())
}

func (t Time) Unix() int64 {
	return t.UnixMicro() / 1000 / 1000
}

func (t Time) UnixMicro() int64 {
	return int64(t) - (crTimeEpochDelta * 1000 * 1000)
}

func (t Time) String() string {
	return t.Time().String()
}

type GUID string

var (
	_ json.Marshaler   = GUID("")
	_ json.Unmarshaler = (*GUID)(nil)
)

func (s GUID) MarshalJSON() ([]byte, error) {
	if err := s.Valid(); err != nil {
		return nil, err
	}
	return json.Marshal(string(s))
}

func (s *GUID) UnmarshalJSON(b []byte) error {
	if err := json.Unmarshal(b, (*string)(s)); err != nil {
		return err
	}
	if err := s.Valid(); err != nil {
		return err
	}
	return nil
}

func (s GUID) String() string {
	v, _ := s.Canonical()
	return v
}

func (s GUID) Valid() error {
	_, err := s.Bytes()
	return err
}

func (s GUID) Canonical() (string, error) {
	u, err := s.Bytes()
	return fmt.Sprintf("%x-%x-%x-%x-%x", u[0:4], u[4:6], u[6:8], u[8:10], u[10:]), err
}

func (s GUID) Bytes() ([16]byte, error) {
	var u [16]byte
	if len(s) != 36 || s[8] != '-' || s[13] != '-' || s[18] != '-' || s[23] != '-' {
		return u, fmt.Errorf("invalid guid format")
	}
	for i, x := range [16]int{0, 2, 4, 6, 9, 11, 14, 16, 19, 21, 24, 26, 28, 30, 32, 34} {
		v, ok := xtob(s[x], s[x+1])
		if !ok {
			return u, fmt.Errorf("invalid guid hex char")
		}
		u[i] = v
	}
	return u, nil
}

func u16string(s string) []byte {
	codes := utf16.Encode([]rune(s))
	b := make([]byte, len(codes)*2)
	for i, r := range codes {
		b[i*2] = byte(r)
		b[i*2+1] = byte(r >> 8)
	}
	return b
}

var xv = [256]byte{
	255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255,
	255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255,
	255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255,
	0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 255, 255, 255, 255, 255, 255,
	255, 10, 11, 12, 13, 14, 15, 255, 255, 255, 255, 255, 255, 255, 255, 255,
	255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255,
	255, 10, 11, 12, 13, 14, 15, 255, 255, 255, 255, 255, 255, 255, 255, 255,
	255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255,
	255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255,
	255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255,
	255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255,
	255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255,
	255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255,
	255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255,
	255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255,
	255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255,
}

func xtob(x1, x2 byte) (byte, bool) {
	b1 := xv[x1]
	b2 := xv[x2]
	return (b1 << 4) | b2, b1 != 255 && b2 != 255
}
