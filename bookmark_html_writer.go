package crb

import (
	"bufio"
	"html"
	"io"
	"strconv"
	"strings"
)

// FaviconFunc gets the data URL of the favicon for url, returning an empty
// string if one isn't available.
type FaviconFunc func(url string) (dataURL string)

// Export writes a HTML bookmark export of b to w, getting favicons from f.
func Export(w io.Writer, b *Bookmarks, f FaviconFunc) error {
	wr := bufio.NewWriter(w)
	wr.WriteString("<!DOCTYPE NETSCAPE-Bookmark-file-1>\r\n" +
		"<!-- This is an automatically generated file.\r\n" +
		"     It will be read and overwritten.\r\n" +
		"     DO NOT EDIT! -->\r\n" +
		"<META HTTP-EQUIV=\"Content-Type\" CONTENT=\"text/html; charset=UTF-8\">\r\n" +
		"<TITLE>Bookmarks</TITLE>\r\n" +
		"<H1>Bookmarks</H1>\r\n" +
		"<DL><p>\r\n")
	exportNode(wr, b.Roots.BookmarkBar, f, 1, "bookmark_bar")
	exportNode(wr, b.Roots.Other, f, 1, "other")
	exportNode(wr, b.Roots.MobileBookmark, f, 1, "synced")
	wr.WriteString("</DL><p>\r\n")
	return wr.Flush()
}

func exportNode(wr *bufio.Writer, n BookmarkNode, f FaviconFunc, indent int, specialType string) {
	switch n.Type {
	case NodeTypeURL:
		for i := 0; i < indent; i++ {
			wr.WriteString("    ")
		}
		wr.WriteString("<DT><A")
		if n.URL != "" {
			wr.WriteString(" HREF=\"")
			wr.WriteString(escapeHTML(n.URL, true))
			wr.WriteString("\"")
		}
		if !n.DateAdded.IsZero() {
			wr.WriteString(" ADD_DATE=\"")
			wr.WriteString(strconv.FormatInt(n.DateAdded.Unix(), 10))
			wr.WriteString("\"")
		}
		if f != nil {
			if v := f(n.URL); v != "" {
				wr.WriteString(" ICON=\"")
				wr.WriteString(escapeHTML(v, true))
				wr.WriteString("\"")
			}
		}
		wr.WriteString(">")
		wr.WriteString(escapeHTML(n.Name, false))
		wr.WriteString("</A>\r\n")
	case NodeTypeFolder:
		switch specialType {
		case "other", "synced":
			if n.Children != nil {
				for _, c := range *n.Children {
					exportNode(wr, c, f, indent, "")
				}
			}
		default:
			for i := 0; i < indent; i++ {
				wr.WriteString("    ")
			}
			wr.WriteString("<DT><H3")
			if !n.DateAdded.IsZero() {
				wr.WriteString(" ADD_DATE=\"")
				wr.WriteString(strconv.FormatInt(n.DateAdded.Unix(), 10))
				wr.WriteString("\"")
			}
			if !n.DateModified.IsZero() {
				wr.WriteString(" LAST_MODIFIED=\"")
				wr.WriteString(strconv.FormatInt(n.DateModified.Unix(), 10))
				wr.WriteString("\"")
			}
			if specialType == "bookmark_bar" {
				wr.WriteString(" PERSONAL_TOOLBAR_FOLDER=\"true\"")
			}
			wr.WriteString(">")
			wr.WriteString(escapeHTML(n.Name, false))
			wr.WriteString("</H3>\r\n")
			for i := 0; i < indent; i++ {
				wr.WriteString("    ")
			}
			wr.WriteString("<DL><p>\r\n")
			if n.Children != nil {
				for _, c := range *n.Children {
					exportNode(wr, c, f, indent+1, "")
				}
			}
			for i := 0; i < indent; i++ {
				wr.WriteString("    ")
			}
			wr.WriteString("</DL><p>\r\n")
		}
	}
}

func escapeHTML(s string, attr bool) string {
	if attr {
		return strings.ReplaceAll(s, "\"", "&quot;")
	} else {
		return html.EscapeString(s)
	}
}
