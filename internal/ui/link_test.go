package ui

import (
	"strings"
	"testing"

	"github.com/jedipunkz/bsky/internal/api"
)

// hrefs returns the URLs of every OSC 8 hyperlink opened in s.
func hrefs(s string) []string {
	var out []string
	for _, part := range strings.Split(s, "\033]8;;")[1:] {
		if uri, _, ok := strings.Cut(part, "\a"); ok && uri != "" {
			out = append(out, uri)
		}
	}
	return out
}

func TestRenderTextWithLinks_WrappedURLKeepsFullHref(t *testing.T) {
	const url = "https://example.com/a/very/long/path/that/will/wrap"
	rec := api.PostRecord{Text: "see " + url + " here"}

	got := hrefs(renderTextWithLinks(rec, 20, textStyle))
	if len(got) == 0 {
		t.Fatal("no hyperlink emitted")
	}
	for _, h := range got {
		if h != url {
			t.Errorf("href = %q, want the full URL %q", h, url)
		}
	}
}

func TestRenderTextWithLinks_FacetURLBeatsShortenedText(t *testing.T) {
	// Bluesky shortens long links for display and keeps the real target in a
	// facet, so the visible text is not a usable URL on its own.
	const uri = "https://example.com/full/path?utm_source=bsky"
	text := "check example.com/full/... out"
	start := strings.Index(text, "example.com")
	rec := api.PostRecord{
		Text: text,
		Facets: []api.Facet{{
			Index:    api.FacetIndex{ByteStart: start, ByteEnd: start + len("example.com/full/...")},
			Features: []api.FacetFeature{{Type: "app.bsky.richtext.facet#link", URI: uri}},
		}},
	}

	got := hrefs(renderTextWithLinks(rec, 60, textStyle))
	if len(got) != 1 || got[0] != uri {
		t.Errorf("hrefs = %v, want [%q]", got, uri)
	}
}

func TestRenderTextWithLinks_IgnoresNonLinkAndOutOfRangeFacets(t *testing.T) {
	rec := api.PostRecord{
		Text: "hi @bob.bsky.social",
		Facets: []api.Facet{
			{
				Index:    api.FacetIndex{ByteStart: 3, ByteEnd: 19},
				Features: []api.FacetFeature{{Type: "app.bsky.richtext.facet#mention"}},
			},
			{
				Index:    api.FacetIndex{ByteStart: 0, ByteEnd: 9999},
				Features: []api.FacetFeature{{Type: "app.bsky.richtext.facet#link", URI: "https://evil.example"}},
			},
		},
	}

	if got := hrefs(renderTextWithLinks(rec, 40, textStyle)); len(got) != 0 {
		t.Errorf("hrefs = %v, want none", got)
	}
}

func TestRenderTextWithLinks_PlainTextIsUnchanged(t *testing.T) {
	rec := api.PostRecord{Text: "no links here at all"}
	got := renderTextWithLinks(rec, 40, textStyle)
	if strings.Contains(got, "\033]8;;") {
		t.Errorf("unexpected hyperlink in %q", got)
	}
	if !strings.Contains(got, "no links here at all") {
		t.Errorf("text lost: %q", got)
	}
}

func TestMapWrapped_StaysAlignedThroughWrapping(t *testing.T) {
	cases := []string{
		"see https://example.com/a/very/long/path/that/wraps here",
		"日本語のテキスト https://example.com/foo とても長い文章がここに続きます",
		"multiple   spaces   here and more words to force a wrap",
		"line one\nline two is quite long and will wrap somewhere",
		"tab\there and a lot of words to make this wrap at some point",
	}
	for _, orig := range cases {
		wrapped := wrapText(orig, 20)
		idx, ok := mapWrapped(orig, wrapped)
		if !ok {
			t.Errorf("mapWrapped(%q) fell out of step", orig)
			continue
		}
		for w, o := range idx {
			if o == -1 {
				if wrapped[w] != '\n' {
					t.Errorf("%q: byte %d unmapped but is %q", orig, w, wrapped[w])
				}
				continue
			}
			if orig[o] != wrapped[w] {
				t.Errorf("%q: wrapped[%d]=%q maps to orig[%d]=%q", orig, w, wrapped[w], o, orig[o])
			}
		}
	}
}
