package index

import (
	"errors"
	"strings"
	"testing"
)

func TestParseAnchorCases(t *testing.T) {
	cases := []struct {
		name      string
		input     string
		wantKind  AnchorKind
		wantValue string
		wantErr   error
	}{
		{"heading", "#intro", AnchorHeading, "intro", nil},
		{"heading cjk", "#中文标题", AnchorHeading, "中文标题", nil},
		{"heading spaces trimmed", " #intro ", AnchorHeading, "intro", nil},
		{"para", "#para-1", AnchorPara, "1", nil},
		{"para two digits", "#para-12", AnchorPara, "12", nil},
		{"char", "#char[0:5]", AnchorChar, "0:5", nil},
		{"char zero span", "#char[3:3]", AnchorChar, "3:3", nil},
		{"empty", "", "", "", ErrAnchorMalformed},
		{"missing hash", "intro", "", "", ErrAnchorMalformed},
		{"hash only", "#", "", "", ErrAnchorMalformed},
		{"double hash", "##intro", "", "", ErrAnchorMalformed},
		{"para missing number", "#para-", "", "", ErrAnchorMalformed},
		{"para zero", "#para-0", "", "", ErrAnchorMalformed},
		{"para negative", "#para--1", "", "", ErrAnchorMalformed},
		{"char missing end", "#char[0:]", "", "", ErrAnchorMalformed},
		{"char negative", "#char[-1:2]", "", "", ErrAnchorMalformed},
		{"char alpha", "#char[a:b]", "", "", ErrAnchorMalformed},
		{"embedded hash", "#intro#extra", "", "", ErrAnchorMalformed},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			gotKind, gotValue, err := ParseAnchor(tc.input)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("err = %v, want %v", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseAnchor: %v", err)
			}
			if gotKind != tc.wantKind || gotValue != tc.wantValue {
				t.Fatalf("ParseAnchor = %q/%q, want %q/%q",
					gotKind, gotValue, tc.wantKind, tc.wantValue)
			}
		})
	}
}

func TestSlugifyHeadingCases(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{"english lower", "Hello World", "hello-world"},
		{"case folded", "HELLO World", "hello-world"},
		{"numbers", "Section 2 Basics", "section-2-basics"},
		{"cjk kept", "中文 标题", "中文-标题"},
		{"mixed cjk english", "LLM Wiki 是 Compounding", "llm-wiki-是-compounding"},
		{"punctuation removed", "Hello, world!", "hello-world"},
		{"slashes removed", "A/B Testing", "ab-testing"},
		{"underscore dash", "a_b--c", "a-b-c"},
		{"trim dash", " --- Hello --- ", "hello"},
		{"emoji removed", "Idea 🚀 Launch", "idea-launch"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			if got := slugifyHeading(tc.input); got != tc.want {
				t.Fatalf("slugifyHeading(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestResolveAnchorHeadingCases(t *testing.T) {
	content := `---
id: raw-test
---

# Intro
First intro paragraph.

## Details
Detail text.

## Details
Second detail.

# 中文 标题 2!
中文段落。

### Deep Section
deep text.
`
	cases := []struct {
		name     string
		anchor   string
		contains []string
		excludes []string
		wantErr  error
	}{
		{"h1 intro includes child", "#intro", []string{"# Intro", "## Details"}, []string{"# 中文"}, nil},
		{"duplicate first", "#details", []string{"Detail text."}, []string{"Second detail."}, nil},
		{"cjk slug", "#中文-标题-2", []string{"# 中文 标题 2!", "中文段落。"}, nil, nil},
		{"deep section", "#deep-section", []string{"### Deep Section", "deep text."}, nil, nil},
		{"case insensitive", "#INTRO", []string{"First intro paragraph."}, nil, nil},
		{"special removed lookup", "#中文-标题-2", []string{"中文段落。"}, nil, nil},
		{"missing", "#missing", nil, nil, ErrHeadingNotFound},
		{"malformed", "intro", nil, nil, ErrAnchorMalformed},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got, span, err := ResolveAnchor([]byte(content), tc.anchor)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("err = %v, want %v", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("ResolveAnchor: %v", err)
			}
			if span[0] < 0 || span[1] <= span[0] {
				t.Fatalf("span = %v, want positive range", span)
			}
			for _, want := range tc.contains {
				if !strings.Contains(got, want) {
					t.Fatalf("heading content missing %q:\n%s", want, got)
				}
			}
			for _, bad := range tc.excludes {
				if strings.Contains(got, bad) {
					t.Fatalf("heading content unexpectedly contains %q:\n%s", bad, got)
				}
			}
		})
	}
}

func TestResolveAnchorParagraphCases(t *testing.T) {
	content := "---\ntitle: x\n---\n\nFirst paragraph.\nwrapped line.\n\n\nSecond paragraph 中文。\n\n- list item\n- still same paragraph\n"
	cases := []struct {
		name    string
		anchor  string
		want    string
		wantErr error
	}{
		{"first skips frontmatter", "#para-1", "First paragraph.\nwrapped line.", nil},
		{"second after blank run", "#para-2", "Second paragraph 中文。", nil},
		{"third list block", "#para-3", "- list item\n- still same paragraph", nil},
		{"zero invalid parse", "#para-0", "", ErrAnchorMalformed},
		{"out of range", "#para-4", "", ErrParaOutOfRange},
		{"malformed", "#para-x", "", ErrAnchorMalformed},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got, span, err := ResolveAnchor([]byte(content), tc.anchor)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("err = %v, want %v", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("ResolveAnchor: %v", err)
			}
			if got != tc.want {
				t.Fatalf("paragraph = %q, want %q", got, tc.want)
			}
			if span[0] <= 0 || span[1] <= span[0] {
				t.Fatalf("span = %v, want body span after frontmatter", span)
			}
		})
	}
}

func TestResolveAnchorCharSpanCases(t *testing.T) {
	content := "ab中文cd"
	cases := []struct {
		name    string
		anchor  string
		want    string
		span    [2]int
		wantErr error
	}{
		{"ascii", "#char[0:2]", "ab", [2]int{0, 2}, nil},
		{"cjk exact", "#char[2:4]", "中文", [2]int{2, 4}, nil},
		{"mixed", "#char[1:5]", "b中文c", [2]int{1, 5}, nil},
		{"empty", "#char[3:3]", "", [2]int{3, 3}, nil},
		{"full", "#char[0:6]", content, [2]int{0, 6}, nil},
		{"start greater end", "#char[4:2]", "", [2]int{}, ErrCharSpanInvalid},
		{"end overflow", "#char[0:7]", "", [2]int{}, ErrCharSpanInvalid},
		{"negative malformed", "#char[-1:2]", "", [2]int{}, ErrAnchorMalformed},
		{"missing bracket malformed", "#char[1:2", "", [2]int{}, ErrAnchorMalformed},
		{"non numeric malformed", "#char[a:b]", "", [2]int{}, ErrAnchorMalformed},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got, span, err := ResolveAnchor([]byte(content), tc.anchor)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("err = %v, want %v", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("ResolveAnchor: %v", err)
			}
			if got != tc.want || span != tc.span {
				t.Fatalf("ResolveAnchor = %q/%v, want %q/%v", got, span, tc.want, tc.span)
			}
		})
	}
}

func TestQuoteHashNormalizesText(t *testing.T) {
	cases := []struct {
		name string
		a    string
		b    string
		same bool
	}{
		{"trim spaces", " quote ", "quote", true},
		{"trim newlines", "\nquote\n", "quote", true},
		{"collapse blank lines", "a\n\n\nb", "a\nb", true},
		{"crlf", "a\r\n\r\nb", "a\nb", true},
		{"content differs", "quote one", "quote two", false},
		{"single newline significant", "a\nb", "a b", false},
		{"stable length", "abc", "abc", true},
		{"unicode stable", " 中文\n\n文本 ", "中文\n文本", true},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			gotA := QuoteHash(tc.a)
			gotB := QuoteHash(tc.b)
			if len(gotA) != 8 || len(gotB) != 8 {
				t.Fatalf("hash lengths = %d/%d, want 8/8", len(gotA), len(gotB))
			}
			if (gotA == gotB) != tc.same {
				t.Fatalf("hash equality = %v, want %v (%s/%s)", gotA == gotB, tc.same, gotA, gotB)
			}
		})
	}
}
