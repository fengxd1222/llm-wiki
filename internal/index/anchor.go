package index

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// AnchorKind 是 read_raw_anchor 支持的三类 anchor。
type AnchorKind string

const (
	AnchorHeading AnchorKind = "heading"
	AnchorPara    AnchorKind = "para"
	AnchorChar    AnchorKind = "char"
)

// ErrAnchorMalformed 表示 anchor 字符串不符合 #heading / #para-N / #char[a:b]。
var ErrAnchorMalformed = errors.New("anchor malformed")

// ErrHeadingNotFound 表示 heading slug 没有命中。
var ErrHeadingNotFound = errors.New("heading anchor not found")

// ErrParaOutOfRange 表示 paragraph 序号越界。
var ErrParaOutOfRange = errors.New("paragraph anchor out of range")

// ErrCharSpanInvalid 表示 char span 非法或越界。
var ErrCharSpanInvalid = errors.New("char span invalid")

var charAnchorRe = regexp.MustCompile(`^char\[(\d+):(\d+)\]$`)

// ParseAnchor 解析 read_raw_anchor 的 anchor 字符串。
//
// 返回 kind 与 value：
//   - #heading-slug -> ("heading", "heading-slug")
//   - #para-N -> ("para", "N")
//   - #char[start:end] -> ("char", "start:end")
func ParseAnchor(s string) (AnchorKind, string, error) {
	a := strings.TrimSpace(s)
	if a == "" || !strings.HasPrefix(a, "#") || strings.HasPrefix(a, "##") {
		return "", "", fmt.Errorf("%w: %q", ErrAnchorMalformed, s)
	}
	body := strings.TrimPrefix(a, "#")
	if body == "" {
		return "", "", fmt.Errorf("%w: empty anchor", ErrAnchorMalformed)
	}
	if strings.HasPrefix(body, "para-") {
		n := strings.TrimPrefix(body, "para-")
		if n == "" {
			return "", "", fmt.Errorf("%w: %q", ErrAnchorMalformed, s)
		}
		v, err := strconv.Atoi(n)
		if err != nil || v <= 0 {
			return "", "", fmt.Errorf("%w: %q", ErrAnchorMalformed, s)
		}
		return AnchorPara, n, nil
	}
	if m := charAnchorRe.FindStringSubmatch(body); m != nil {
		return AnchorChar, m[1] + ":" + m[2], nil
	}
	if strings.HasPrefix(body, "char") {
		return "", "", fmt.Errorf("%w: %q", ErrAnchorMalformed, s)
	}
	if strings.Contains(body, "#") || strings.TrimSpace(body) == "" {
		return "", "", fmt.Errorf("%w: %q", ErrAnchorMalformed, s)
	}
	return AnchorHeading, body, nil
}

// ResolveAnchor 在 markdown raw 内容里解析 anchor，返回命中文本与原文 rune span。
func ResolveAnchor(content []byte, anchor string) (string, [2]int, error) {
	kind, value, err := ParseAnchor(anchor)
	if err != nil {
		return "", [2]int{}, err
	}
	text := string(content)
	switch kind {
	case AnchorHeading:
		return resolveHeading(text, value)
	case AnchorPara:
		n, _ := strconv.Atoi(value)
		return resolvePara(text, n)
	case AnchorChar:
		return resolveCharSpan(text, value)
	default:
		return "", [2]int{}, fmt.Errorf("%w: unknown anchor kind %q", ErrAnchorMalformed, kind)
	}
}

// QuoteHash 返回 normalized text 的 sha256 前 8 位 hex。
func QuoteHash(text string) string {
	normalized := normalizeQuoteText(text)
	sum := sha256.Sum256([]byte(normalized))
	return hex.EncodeToString(sum[:])[:8]
}

func resolveHeading(content, slug string) (string, [2]int, error) {
	bodyStart := bodyByteStart(content)
	lines := splitLinesWithOffsets(content[bodyStart:])
	want := strings.ToLower(strings.TrimSpace(slug))
	for i, line := range lines {
		level, title, ok := parseHeadingLine(line.text)
		if !ok || slugifyHeading(title) != want {
			continue
		}
		start := bodyStart + line.start
		end := len(content)
		for j := i + 1; j < len(lines); j++ {
			nextLevel, _, ok := parseHeadingLine(lines[j].text)
			if ok && nextLevel <= level {
				end = bodyStart + lines[j].start
				break
			}
		}
		return strings.TrimSpace(content[start:end]), runeSpan(content, start, end), nil
	}
	return "", [2]int{}, fmt.Errorf("%w: %s", ErrHeadingNotFound, slug)
}

func resolvePara(content string, n int) (string, [2]int, error) {
	bodyStart := bodyByteStart(content)
	body := content[bodyStart:]
	paras := splitParagraphs(body)
	if n <= 0 || n > len(paras) {
		return "", [2]int{}, fmt.Errorf("%w: para-%d", ErrParaOutOfRange, n)
	}
	p := paras[n-1]
	start := bodyStart + p.start
	end := bodyStart + p.end
	return strings.TrimSpace(content[start:end]), runeSpan(content, start, end), nil
}

func resolveCharSpan(content, value string) (string, [2]int, error) {
	parts := strings.Split(value, ":")
	if len(parts) != 2 {
		return "", [2]int{}, fmt.Errorf("%w: %s", ErrCharSpanInvalid, value)
	}
	start, err1 := strconv.Atoi(parts[0])
	end, err2 := strconv.Atoi(parts[1])
	runeCount := utf8.RuneCountInString(content)
	if err1 != nil || err2 != nil || start < 0 || end < 0 || start > end || end > runeCount {
		return "", [2]int{}, fmt.Errorf("%w: %s", ErrCharSpanInvalid, value)
	}
	startByte := byteOffsetForRune(content, start)
	endByte := byteOffsetForRune(content, end)
	return content[startByte:endByte], [2]int{start, end}, nil
}

type lineSpan struct {
	text       string
	start, end int
}

func splitLinesWithOffsets(s string) []lineSpan {
	var lines []lineSpan
	start := 0
	for start < len(s) {
		end := strings.IndexByte(s[start:], '\n')
		if end < 0 {
			lines = append(lines, lineSpan{text: s[start:], start: start, end: len(s)})
			break
		}
		end += start
		lineEnd := end + 1
		lines = append(lines, lineSpan{text: s[start:lineEnd], start: start, end: lineEnd})
		start = lineEnd
	}
	if len(s) == 0 {
		return []lineSpan{{}}
	}
	return lines
}

func parseHeadingLine(line string) (int, string, bool) {
	trimmed := strings.TrimRight(line, "\r\n")
	if !strings.HasPrefix(trimmed, "#") {
		return 0, "", false
	}
	level := 0
	for level < len(trimmed) && trimmed[level] == '#' {
		level++
	}
	if level == 0 || level > 6 || level >= len(trimmed) || trimmed[level] != ' ' {
		return 0, "", false
	}
	title := strings.TrimSpace(trimmed[level+1:])
	title = strings.TrimSpace(strings.TrimRight(title, "#"))
	if title == "" {
		return 0, "", false
	}
	return level, title, true
}

func slugifyHeading(title string) string {
	var b strings.Builder
	lastDash := false
	for _, r := range strings.ToLower(title) {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
			lastDash = false
		case unicode.IsSpace(r) || r == '-' || r == '_':
			if b.Len() > 0 && !lastDash {
				b.WriteByte('-')
				lastDash = true
			}
		}
	}
	return strings.Trim(b.String(), "-")
}

func bodyByteStart(s string) int {
	if !strings.HasPrefix(s, "---") {
		return 0
	}
	lines := splitLinesWithOffsets(s)
	if len(lines) == 0 || strings.TrimSpace(lines[0].text) != "---" {
		return 0
	}
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i].text) == "---" {
			return lines[i].end
		}
	}
	return 0
}

type paragraphSpan struct {
	start, end int
}

func splitParagraphs(body string) []paragraphSpan {
	var paras []paragraphSpan
	start := -1
	lastNonBlankEnd := 0
	for _, line := range splitLinesWithOffsets(body) {
		if strings.TrimSpace(line.text) == "" {
			if start >= 0 {
				paras = append(paras, paragraphSpan{start: start, end: lastNonBlankEnd})
				start = -1
			}
			continue
		}
		if start < 0 {
			start = line.start
		}
		lastNonBlankEnd = line.end
	}
	if start >= 0 {
		paras = append(paras, paragraphSpan{start: start, end: lastNonBlankEnd})
	}
	return paras
}

func runeSpan(s string, startByte, endByte int) [2]int {
	return [2]int{
		utf8.RuneCountInString(s[:startByte]),
		utf8.RuneCountInString(s[:endByte]),
	}
}

func byteOffsetForRune(s string, target int) int {
	if target <= 0 {
		return 0
	}
	count := 0
	for i := range s {
		if count == target {
			return i
		}
		count++
	}
	return len(s)
}

func normalizeQuoteText(text string) string {
	s := strings.ReplaceAll(text, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	s = strings.TrimSpace(s)
	for strings.Contains(s, "\n\n") {
		s = strings.ReplaceAll(s, "\n\n", "\n")
	}
	return s
}
