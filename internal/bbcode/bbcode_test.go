package bbcode

import (
	"strings"
	"testing"
)

func TestRenderBold(t *testing.T) {
	got := Render("[b]Hello[/b]")
	want := "<strong>Hello</strong>"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestRenderImage(t *testing.T) {
	got := Render("[img]http://example.com/pic.png[/img]")
	want := `<img src="http://example.com/pic.png" loading="lazy" class="bb-img" style="max-width:100%; height:auto;">`
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestRenderURL(t *testing.T) {
	got := Render("[url=http://example.com]Click[/url]")
	want := `<a href="http://example.com" target="_blank" rel="noopener">Click</a>`
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestRenderSpoiler(t *testing.T) {
	got := Render(`[spoiler="Secret"]hidden text[/spoiler]`)
	if !containsAll(got, "<details", `class="spoiler"`, "<summary>Secret</summary>", "hidden text") {
		t.Errorf("got %q", got)
	}
}

func TestRenderSpoilerNoTitle(t *testing.T) {
	got := Render("[spoiler]hidden[/spoiler]")
	if !containsAll(got, "<summary>Spoiler</summary>", "hidden") {
		t.Errorf("got %q", got)
	}
}

func TestRenderNested(t *testing.T) {
	got := Render("[b][i]nested[/i][/b]")
	if !containsAll(got, "<strong>", "<em>nested</em>", "</strong>") {
		t.Errorf("got %q", got)
	}
}

func TestRenderSize(t *testing.T) {
	got := Render("[size=24]Big[/size]")
	want := `<span style="font-size:24px">Big</span>`
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestRenderList(t *testing.T) {
	got := Render("[list][*]one[*]two[/list]")
	if !containsAll(got, `<ul class="bb-list">`, "<li>one</li>", "<li>two</li>") {
		t.Errorf("got %q", got)
	}
}

func TestRenderHr(t *testing.T) {
	got := Render("before[hr]after")
	if !containsAll(got, "before", `<hr class="bb-hr">`, "after") {
		t.Errorf("got %q", got)
	}
}

func TestRenderAlign(t *testing.T) {
	got := Render("[align=center]centered[/align]")
	if !containsAll(got, `text-align:center`, "centered") {
		t.Errorf("got %q", got)
	}
}

func TestRenderNestedSpoilers(t *testing.T) {
	got := Render("[spoiler=Outer][spoiler=Inner]deep[/spoiler]outer[/spoiler]")
	// Should have 2 details elements
	count := 0
	for i := 0; i < len(got)-8; i++ {
		if got[i:i+8] == "<details" {
			count++
		}
	}
	if count != 2 {
		t.Errorf("expected 2 details tags, got %d in: %s", count, got)
	}
}

func TestRenderSmiley(t *testing.T) {
	got := Render("hello :in_love:")
	if got != "hello 😍" {
		t.Errorf("got %q", got)
	}
}

func TestCaseInsensitiveTags(t *testing.T) {
	// Uppercase tags
	got := Render("[B]bold[/B] [I]italic[/I] [URL=http://x.com]link[/URL]")
	if !containsAll(got, "<strong>bold</strong>", "<em>italic</em>", `<a href="http://x.com"`) {
		t.Errorf("got %q", got)
	}
}

func TestURLWithIMG(t *testing.T) {
	// [URL=...][IMG]...[/IMG][/URL] — clickable image
	got := Render(`[URL=https://example.com][IMG]https://example.com/pic.jpg[/IMG][/URL]`)
	if !containsAll(got, `<a href="https://example.com"`, `<img src="https://example.com/pic.jpg"`) {
		t.Errorf("got %q", got)
	}
}

func TestSizeInAlign(t *testing.T) {
	// [align=center][size=20]text[/size][/align] — size inside align
	got := Render(`[align=center][size=20]big[/size][/align]`)
	if !containsAll(got, `text-align:center`, `font-size:20px`, "big") {
		t.Errorf("got %q", got)
	}
}

func TestNestedSpoilerWithSize(t *testing.T) {
	got := Render(`[spoiler=Test][size=18]text[/size][/spoiler]`)
	if !containsAll(got, "<details", "Test", "font-size:18px", "text") {
		t.Errorf("got %q", got)
	}
}

func TestRenderOneline(t *testing.T) {
	got := Render("[oneline]line1\nline2[/oneline]")
	if !containsAll(got, `<span class="bb-oneline">`, "line1 line2") {
		t.Errorf("got %q", got)
	}
}

func TestRenderColor(t *testing.T) {
	got := Render("[color=olive]text[/color]")
	want := `<span style="color:olive">text</span>`
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestMultilineURLWithIMG(t *testing.T) {
	// Real-world case: URL + IMG on separate lines with newlines
	got := Render(`[URL=https://fastpic.ru/view/xxx.jpg.html]
[IMG]https://i.fastpic.ru/thumb/xxx.jpeg[/IMG]
[/URL]`)
	if !containsAll(got, `<a href="https://fastpic.ru/view/xxx.jpg.html"`, `<img src="https://i.fastpic.ru/thumb/xxx.jpeg"`) {
		t.Errorf("got %q", got)
	}
}

func TestMultilineAlignSize(t *testing.T) {
	// [align=center] with newline and [size] inside
	got := Render(`[align=center]
[size=20]big text[/size]
[/align]`)
	if !containsAll(got, `text-align:center`, `font-size:20px`, "big text") {
		t.Errorf("got %q", got)
	}
}

func containsAll(s string, substrs ...string) bool {
	for _, sub := range substrs {
		if !strings.Contains(s, sub) {
			return false
		}
	}
	return true
}
