package pdfmemory

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"io"
	"math"
	"os"
	"strings"
	"testing"

	pdfreader "github.com/ledongthuc/pdf"
)

func TestGenerateKoreanLongMemoryRoundTrip(t *testing.T) {
	text := koreanFixture(t)
	doc, err := Generate(text)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	extracted, pages := extractPDFText(t, doc.Bytes)
	extracted = normalizeExtractedText(extracted)
	if pages != doc.PageCount || pages < 1 {
		t.Fatalf("page count: extracted=%d metadata=%d", pages, doc.PageCount)
	}
	if doc.PageWidthPoints != pageWidth || doc.PageHeightPoints != pageHeight {
		t.Fatalf("page size=(%.2f, %.2f), want A4=(%.2f, %.2f)", doc.PageWidthPoints, doc.PageHeightPoints, pageWidth, pageHeight)
	}
	if strings.ReplaceAll(extracted, "\n", "") != strings.ReplaceAll(text, "\n", "") {
		t.Fatal("extracted Unicode text differs from the source")
	}
	for _, term := range []string{"관계", "상태", "대화 사건", "서윤", "민호", "하린", "도윤"} {
		if !strings.Contains(extracted, term) {
			t.Fatalf("searchable PDF text is missing %q", term)
		}
	}
}

func TestGeneratePreservesLogicalInputMetadata(t *testing.T) {
	text := "앞 문장  이중 공백\n가운데 문장: 유채(油菜) 종자(種子)\n마지막 문장"
	doc, err := Generate(text)
	if err != nil {
		t.Fatal(err)
	}
	extracted, _ := extractPDFText(t, doc.Bytes)
	if normalizeExtractedText(extracted) != text {
		t.Fatalf("input was rewritten: got %q want %q", normalizeExtractedText(extracted), text)
	}
	for _, term := range []string{"油", "菜", "種"} {
		if !strings.Contains(extracted, term) {
			t.Fatalf("searchable PDF text is missing Hanja regression glyph %q", term)
		}
	}
	sum := sha256.Sum256([]byte(text))
	if doc.LogicalTextSHA256 != hex.EncodeToString(sum[:]) || doc.LogicalChars != len([]rune(text)) {
		t.Fatalf("logical metadata mismatch: %#v", doc)
	}
	if doc.PDFBytes != len(doc.Bytes) || doc.Base64Chars != base64.StdEncoding.EncodedLen(len(doc.Bytes)) {
		t.Fatalf("size metadata mismatch: %#v", doc)
	}
}

func TestGenerateFailuresReturnNoPDF(t *testing.T) {
	tests := []struct {
		name string
		text string
		want error
	}{
		{name: "empty", text: "", want: ErrEmptyText},
		{name: "invalid UTF-8", text: string([]byte{0xff, 0xfe}), want: ErrInvalidUTF8},
		{name: "control", text: "첫 줄\r\n둘째 줄", want: ErrUnsupportedText},
		{name: "missing glyph", text: "한글 기억 😀", want: ErrUnsupportedText},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			doc, err := Generate(test.text)
			if !errors.Is(err, test.want) {
				t.Fatalf("error=%v, want %v", err, test.want)
			}
			if len(doc.Bytes) != 0 {
				t.Fatalf("failure produced %d PDF bytes", len(doc.Bytes))
			}
		})
	}
	doc, err := generateWithFont("한글 기억", []byte("not a TrueType font"))
	if !errors.Is(err, ErrFontLoad) || len(doc.Bytes) != 0 {
		t.Fatalf("font failure: doc=%#v err=%v", doc, err)
	}
}

func TestGenerateAddsPagesAndIsDeterministic(t *testing.T) {
	rowsPerColumn := int(math.Floor((pageHeight - 2*pageMargin) / lineHeight))
	lines := make([]string, rowsPerColumn*columnCount+25)
	for index := range lines {
		lines[index] = "기억 행"
	}
	text := strings.Join(lines, "\n")
	first, err := Generate(text)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Generate(text)
	if err != nil {
		t.Fatal(err)
	}
	if first.PageCount != 2 || !bytes.Equal(first.Bytes, second.Bytes) {
		t.Fatalf("page count/determinism mismatch: pages=%d", first.PageCount)
	}
}

func koreanFixture(t *testing.T) string {
	t.Helper()
	raw, err := os.ReadFile("testdata/korean_long_memory.txt")
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	if strings.Contains(text, "\r") {
		t.Fatal("fixture must use LF line endings")
	}
	return strings.TrimSuffix(text, "\n")
}

func extractPDFText(t *testing.T, data []byte) (string, int) {
	t.Helper()
	reader, err := pdfreader.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("open generated PDF: %v", err)
	}
	plain, err := reader.GetPlainText()
	if err != nil {
		t.Fatalf("extract generated PDF: %v", err)
	}
	extracted, err := io.ReadAll(plain)
	if err != nil {
		t.Fatalf("read extracted text: %v", err)
	}
	return string(extracted), reader.NumPage()
}

func normalizeExtractedText(text string) string {
	return strings.TrimPrefix(text, "\n")
}
