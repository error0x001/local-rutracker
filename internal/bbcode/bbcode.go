package bbcode

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	reURL    = regexp.MustCompile(`(?is)\[url=(.*?)\](.*?)\[/url\]`)
	reURLRaw = regexp.MustCompile(`(?is)\[url\](.*?)\[/url\]`)
	reImg    = regexp.MustCompile(`(?is)\[img=(left|right)\](.*?)\[/img\]`)
	reImgRaw = regexp.MustCompile(`(?is)\[img\](.*?)\[/img\]`)
	reSize   = regexp.MustCompile(`(?is)\[size=(\d+)\](.*?)\[/size\]`)
	reColor  = regexp.MustCompile(`(?is)\[color=([^]]+)\](.*?)\[/color\]`)
	reFont   = regexp.MustCompile(`(?is)\[font=([^]]+)\](.*?)\[/font\]`)
	reBold   = regexp.MustCompile(`(?is)\[b\](.*?)\[/b\]`)
	reItalic = regexp.MustCompile(`(?is)\[i\](.*?)\[/i\]`)
	reUnder  = regexp.MustCompile(`(?is)\[u\](.*?)\[/u\]`)
	reStrike = regexp.MustCompile(`(?is)\[s\](.*?)\[/s\]`)
	reSmiley = regexp.MustCompile(`:(\w+):`)
)

var smileyMap = map[string]string{
	"in_love":     "😍",
	"heart":       "❤️",
	"cool":        "😎",
	"smile":       "😊",
	"sad":         "😢",
	"cry":         "😭",
	"angry":       "😡",
	"laugh":       "😂",
	"wink":        "😉",
	"surprised":   "😮",
	"confused":    "🤔",
	"thinking":    "🤔",
	"biggrin":     "😁",
	"rolleyes":    "🙄",
	"mad":         "😠",
	"eek":         "😱",
	"shock":       "😱",
	"oops":        "😳",
	"blush":       "😊",
	"embarrassed": "😳",
	"sleep":       "😴",
	"tongue":      "😛",
	"devil":       "😈",
	"angel":       "😇",
	"geek":        "🤓",
	"nerd":        "🤓",
	"rofl":        "🤣",
	"yikes":       "😬",
}

func Render(bbcode string) string {
	html := parseBlocks(bbcode)
	html = processInline(html)
	html = processSmilies(html)
	html = convertNewlines(html)
	html = strings.ReplaceAll(html, "%%", "%")
	return html
}

// parseBlocks iterates through the string and handles block-level tags
func parseBlocks(s string) string {
	var result strings.Builder
	i := 0
	for i < len(s) {
		tag, attr, content, end := findTag(s, i)
		if tag != "" {
			switch tag {
			case "spoiler":
				result.WriteString(renderSpoiler(attr, content))
			case "quote":
				result.WriteString(renderQuote(attr, content))
			case "code":
				result.WriteString("<pre class=\"code-block\"><code>" + escapeHTML(content) + "</code></pre>")
			case "pre":
				result.WriteString("<pre class=\"pre-block\">" + escapeHTML(content) + "</pre>")
			case "list":
				result.WriteString(renderList(content))
			case "align":
				result.WriteString(renderAlign(attr, content))
			case "hr":
				result.WriteString("<hr class=\"bb-hr\">")
			case "indent":
				result.WriteString("<div class=\"bb-indent\">" + content + "</div>")
			case "box":
				result.WriteString(renderBox(attr, content))
			case "oneline":
				inner := strings.ReplaceAll(content, "\n", " ")
				inner = processInline(inner)
				inner = processSmilies(inner)
				result.WriteString("<span class=\"bb-oneline\">" + inner + "</span>")
			}
			i = end
		} else {
			result.WriteByte(s[i])
			i++
		}
	}
	return result.String()
}

// findTag tries to match any known block tag at position i
func findTag(s string, i int) (tag, attr, content string, end int) {
	tags := []string{"spoiler", "quote", "code", "pre", "list", "align", "indent", "box", "oneline"}
	for _, t := range tags {
		if tag, a, c, e := tryMatch(s, i, t); tag != "" {
			return tag, a, c, e
		}
	}
	// [hr]
	if i+4 <= len(s) && strings.EqualFold(s[i:i+4], "[hr]") {
		return "hr", "", "", i + 4
	}
	return "", "", "", 0
}

// tryMatch tries to match [tag...]content[/tag] at position i
func tryMatch(s string, i int, tag string) (matchedTag, attr, content string, end int) {
	prefix := "[" + tag
	if i+len(prefix) > len(s) {
		return "", "", "", 0
	}
	if !strings.EqualFold(s[i:i+len(prefix)], prefix) {
		return "", "", "", 0
	}

	// Find the closing ] of the opening tag
	closeBracket := strings.Index(s[i+len(prefix):], "]")
	if closeBracket == -1 {
		return "", "", "", 0
	}

	closeBracket += i + len(prefix)

	// Extract attribute (everything between [tag and ])
	rawAttr := strings.TrimSpace(s[i+len(prefix) : closeBracket])
	// Remove leading = if present (e.g., [spoiler="X"] -> attr is "X")
	if strings.HasPrefix(rawAttr, "=") {
		rawAttr = rawAttr[1:]
	}
	attr = strings.TrimSpace(rawAttr)

	// Find the matching closing tag
	contentStart := closeBracket + 1
	contentEnd := findMatchingClose(s, contentStart, tag)
	if contentEnd == -1 {
		return "", "", "", 0
	}

	content = s[contentStart:contentEnd]
	end = contentEnd + len("[/"+tag+"]")
	return tag, attr, content, end
}

// findMatchingClose finds [/tag] handling nesting using a stack approach
func findMatchingClose(s string, start int, tag string) int {
	openTag := "[" + tag
	closeTag := "[/" + tag + "]"
	depth := 1
	pos := start

	for pos < len(s) {
		// Find next opening or closing tag
		openIdx := findCaseInsensitive(s[pos:], openTag)
		closeIdx := findCaseInsensitive(s[pos:], closeTag)

		if closeIdx == -1 {
			return -1
		}

		// If opening tag comes first, increase depth
		if openIdx != -1 && openIdx < closeIdx {
			// Make sure it's not a closing tag
			if pos+openIdx+1 >= len(s) || s[pos+openIdx+1] != '/' {
				depth++
			}
			pos = pos + openIdx + len(openTag)
		} else {
			// Closing tag found
			depth--
			if depth == 0 {
				return pos + closeIdx
			}
			pos = pos + closeIdx + len(closeTag)
		}
	}
	return -1
}

func findCaseInsensitive(s, sub string) int {
	lower := strings.ToLower(s)
	return strings.Index(lower, strings.ToLower(sub))
}

func countTags(s, tag string) int {
	count := 0
	openTag := "[" + tag
	pos := 0
	for {
		idx := findCaseInsensitive(s[pos:], openTag)
		if idx == -1 {
			break
		}
		// Make sure it's not a closing tag
		if idx+1 < len(s[pos:]) && s[pos+idx+1] == '/' {
			pos += idx + 1
			continue
		}
		count++
		pos += idx + len(openTag)
	}
	return count
}

func renderSpoiler(attr, content string) string {
	title := "Spoiler"
	if attr != "" {
		title = strings.Trim(attr, "\" ")
	}
	inner := parseBlocks(content)
	inner = processInline(inner)
	inner = processSmilies(inner)
	return fmt.Sprintf("<details class=\"spoiler\"><summary>%s</summary><div class=\"spoiler-content\">%s</div></details>",
		escapeHTML(title), inner)
}

func renderQuote(attr, content string) string {
	author := ""
	if attr != "" {
		author = strings.Trim(attr, "\" ")
	}
	inner := parseBlocks(content)
	inner = processInline(inner)
	inner = processSmilies(inner)
	header := ""
	if author != "" {
		header = fmt.Sprintf("<div class=\"quote-author\">%s wrote:</div>", escapeHTML(author))
	}
	return fmt.Sprintf("<blockquote class=\"quote\">%s<div class=\"quote-content\">%s</div></blockquote>",
		header, inner)
}

func renderList(content string) string {
	items := strings.Split(content, "[*]")
	var listItems string
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		item = parseBlocks(item)
		item = processInline(item)
		item = processSmilies(item)
		listItems += fmt.Sprintf("<li>%s</li>", item)
	}
	return fmt.Sprintf("<ul class=\"bb-list\">%s</ul>", listItems)
}

func renderAlign(attr, content string) string {
	align := strings.ToLower(strings.TrimSpace(attr))
	if align == "" {
		align = "center"
	}
	switch align {
	case "left", "right", "center", "justify":
	default:
		align = "center"
	}
	inner := parseBlocks(content)
	inner = processInline(inner)
	inner = processSmilies(inner)
	return fmt.Sprintf("<div style=\"text-align:%s\">%s</div>", align, inner)
}

func renderBox(attr, content string) string {
	bgColor := "#f0f0f0"
	borderColor := "#ccc"
	// Parse attribute: [box=bgColor,borderColor] or [box=bgColor]
	if attr != "" {
		parts := strings.SplitN(attr, ",", 2)
		if len(parts) >= 1 {
			bgColor = strings.TrimSpace(parts[0])
		}
		if len(parts) >= 2 {
			borderColor = strings.TrimSpace(parts[1])
		}
	}
	inner := parseBlocks(content)
	inner = processInline(inner)
	inner = processSmilies(inner)
	return fmt.Sprintf("<div class=\"bb-box\" style=\"background:%s; border:1px solid %s; padding:12px; border-radius:6px; margin:8px 0;\">%s</div>",
		bgColor, borderColor, inner)
}

func processInline(html string) string {
	html = reImg.ReplaceAllStringFunc(html, func(match string) string {
		parts := reImg.FindStringSubmatch(match)
		src := parts[2]
		style := ""
		switch parts[1] {
		case "right":
			style = " style=\"float:right; margin:0 0 10px 10px; max-width:100%%;\""
		case "left":
			style = " style=\"float:left; margin:0 10px 10px 0; max-width:100%%;\""
		}
		return fmt.Sprintf("<img src=\"%s\" loading=\"lazy\" class=\"bb-img\"%s>",
			escapeAttr(src), style)
	})

	html = reImgRaw.ReplaceAllStringFunc(html, func(match string) string {
		src := reImgRaw.FindStringSubmatch(match)[1]
		return fmt.Sprintf("<img src=\"%s\" loading=\"lazy\" class=\"bb-img\" style=\"max-width:100%%; height:auto;\">",
			escapeAttr(src))
	})

	html = reURL.ReplaceAllStringFunc(html, func(match string) string {
		parts := reURL.FindStringSubmatch(match)
		href := parts[1]
		text := parts[2]
		if !strings.HasPrefix(href, "http://") && !strings.HasPrefix(href, "https://") && !strings.HasPrefix(href, "/") {
			href = "https://" + href
		}
		return fmt.Sprintf("<a href=\"%s\" target=\"_blank\" rel=\"noopener\">%s</a>",
			escapeAttr(href), text)
	})

	html = reURLRaw.ReplaceAllStringFunc(html, func(match string) string {
		href := reURLRaw.FindStringSubmatch(match)[1]
		if !strings.HasPrefix(href, "http://") && !strings.HasPrefix(href, "https://") && !strings.HasPrefix(href, "/") {
			href = "https://" + href
		}
		return fmt.Sprintf("<a href=\"%s\" target=\"_blank\" rel=\"noopener\">%s</a>",
			escapeAttr(href), href)
	})

	html = reSize.ReplaceAllStringFunc(html, func(match string) string {
		parts := reSize.FindStringSubmatch(match)
		return fmt.Sprintf("<span style=\"font-size:%spx\">%s</span>", parts[1], parts[2])
	})

	html = reColor.ReplaceAllStringFunc(html, func(match string) string {
		parts := reColor.FindStringSubmatch(match)
		return fmt.Sprintf("<span style=\"color:%s\">%s</span>", escapeAttr(parts[1]), parts[2])
	})

	html = reFont.ReplaceAllStringFunc(html, func(match string) string {
		parts := reFont.FindStringSubmatch(match)
		font := strings.TrimSpace(parts[1])
		// Map common RuTracker font names to web-safe fonts
		switch strings.ToLower(font) {
		case "serif1", "serif":
			font = "Georgia, 'Times New Roman', serif"
		case "tahoma", "arial", "verdana":
			font = font + ", sans-serif"
		case "courier", "courier new", "monospace":
			font = "'Courier New', monospace"
		default:
			font = font + ", sans-serif"
		}
		return fmt.Sprintf("<span style=\"font-family:%s\">%s</span>", font, parts[2])
	})

	html = reBold.ReplaceAllString(html, "<strong>$1</strong>")
	html = reItalic.ReplaceAllString(html, "<em>$1</em>")
	html = reUnder.ReplaceAllString(html, "<u>$1</u>")
	html = reStrike.ReplaceAllString(html, "<s>$1</s>")

	return html
}

func processSmilies(html string) string {
	return reSmiley.ReplaceAllStringFunc(html, func(match string) string {
		parts := reSmiley.FindStringSubmatch(match)
		if emoji, ok := smileyMap[parts[1]]; ok {
			return emoji
		}
		return match
	})
}

func convertNewlines(html string) string {
	var result strings.Builder
	inPre := false
	runes := []rune(html)
	for i := 0; i < len(runes); i++ {
		if runes[i] == '<' && i+4 < len(runes) {
			if strings.ToLower(string(runes[i:i+5])) == "<pre>" {
				inPre = true
			}
		}
		if !inPre && runes[i] == '\n' {
			result.WriteString("<br>")
		} else {
			result.WriteRune(runes[i])
		}
		if inPre && i >= 5 && strings.ToLower(string(runes[i-5:i+1])) == "</pre>" {
			inPre = false
		}
	}
	return result.String()
}

func escapeHTML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	return s
}

func escapeAttr(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}
