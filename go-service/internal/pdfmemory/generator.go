// Package pdfmemory creates the searchable PDF used to transport the
// already-selected long-term-memory lane. It does not select memories or
// decide which provider route receives the document.
package pdfmemory

import (
	"crypto/sha256"
	"embed"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/signintech/gopdf"
)

const (
	fontFamily  = "NotoSansKR"
	fontSize    = 2.0
	lineHeight  = 2.3
	pageWidth   = 595.28
	pageHeight  = 841.89
	pageMargin  = 10.0
	columnGap   = 5.0
	columnCount = 4
)

var (
	ErrEmptyText       = errors.New("pdfmemory: empty text")
	ErrInvalidUTF8     = errors.New("pdfmemory: invalid UTF-8")
	ErrUnsupportedText = errors.New("pdfmemory: unsupported text")
	ErrFontLoad        = errors.New("pdfmemory: font load failed")
	ErrRender          = errors.New("pdfmemory: PDF render failed")
)

// assets contains the unmodified Google Fonts Noto Sans KR variable font and
// its OFL license.
//
//go:embed assets/NotoSansKR-VF.ttf assets/NotoSansKR-OFL.txt
var assets embed.FS

// Document contains the generated PDF and measurements derived without
// interpreting provider behavior.
type Document struct {
	Bytes             []byte
	PageCount         int
	PageWidthPoints   float64
	PageHeightPoints  float64
	LogicalChars      int
	LogicalTextSHA256 string
	PDFBytes          int
	Base64Chars       int
	FontAssetBytes    int
}

// Generate creates a searchable A4 PDF containing exactly text. It lays the
// text out at 2pt in four columns and adds pages as needed. It does not trim,
// normalize, summarize, label, or otherwise add to the input.
func Generate(text string) (Document, error) {
	fontData, err := assets.ReadFile("assets/NotoSansKR-VF.ttf")
	if err != nil {
		return Document{}, fmt.Errorf("%w: embedded Noto Sans KR asset: %v", ErrFontLoad, err)
	}
	return generateWithFont(text, fontData)
}

func generateWithFont(text string, fontData []byte) (Document, error) {
	if text == "" {
		return Document{}, ErrEmptyText
	}
	if !utf8.ValidString(text) {
		return Document{}, ErrInvalidUTF8
	}
	if err := validateTextControls(text); err != nil {
		return Document{}, err
	}

	missingGlyphs := map[rune]struct{}{}
	pdf := &gopdf.GoPdf{}
	pdf.Start(gopdf.Config{
		Unit:     gopdf.UnitPT,
		PageSize: gopdf.Rect{W: pageWidth, H: pageHeight},
	})
	if err := pdf.AddTTFFontDataWithOption(fontFamily, fontData, gopdf.TtfOption{
		OnGlyphNotFound: func(r rune) {
			missingGlyphs[r] = struct{}{}
		},
	}); err != nil {
		return Document{}, fmt.Errorf("%w: %v", ErrFontLoad, err)
	}
	if err := pdf.SetFont(fontFamily, "", fontSize); err != nil {
		return Document{}, fmt.Errorf("%w: select Noto Sans KR: %v", ErrFontLoad, err)
	}

	columnWidth := (pageWidth - 2*pageMargin - columnGap*float64(columnCount-1)) / columnCount
	visualLines, err := splitVisualLines(pdf, text, columnWidth)
	if err != nil {
		return Document{}, fmt.Errorf("%w: layout: %v", ErrRender, err)
	}
	if len(missingGlyphs) != 0 {
		return Document{}, fmt.Errorf("%w: missing glyphs %s", ErrUnsupportedText, formatRunes(missingGlyphs))
	}

	rowsPerColumn := int(math.Floor((pageHeight - 2*pageMargin) / lineHeight))
	if rowsPerColumn < 1 {
		return Document{}, fmt.Errorf("%w: invalid page layout", ErrRender)
	}
	linesPerPage := rowsPerColumn * columnCount
	pageCount := (len(visualLines) + linesPerPage - 1) / linesPerPage
	pageSize := &gopdf.Rect{W: pageWidth, H: pageHeight}
	for pageIndex := 0; pageIndex < pageCount; pageIndex++ {
		pdf.AddPageWithOption(gopdf.PageOption{PageSize: pageSize})
		pageStart := pageIndex * linesPerPage
		pageEnd := pageStart + linesPerPage
		if pageEnd > len(visualLines) {
			pageEnd = len(visualLines)
		}
		for index := pageStart; index < pageEnd; index++ {
			line := visualLines[index]
			position := index - pageStart
			column := position / rowsPerColumn
			row := position % rowsPerColumn
			x := pageMargin + float64(column)*(columnWidth+columnGap)
			y := pageMargin + fontSize + float64(row)*lineHeight
			if line != "" {
				pdf.SetXY(x, y)
				if err := pdf.Text(line); err != nil {
					return Document{}, fmt.Errorf("%w: text: %v", ErrRender, err)
				}
			}
		}
	}
	if len(missingGlyphs) != 0 {
		return Document{}, fmt.Errorf("%w: missing glyphs %s", ErrUnsupportedText, formatRunes(missingGlyphs))
	}

	pdfBytes, err := pdf.GetBytesPdfReturnErr()
	if err != nil {
		return Document{}, fmt.Errorf("%w: compile: %v", ErrRender, err)
	}
	if len(pdfBytes) == 0 || !strings.HasPrefix(string(pdfBytes), "%PDF-") {
		return Document{}, fmt.Errorf("%w: invalid PDF output", ErrRender)
	}

	sum := sha256.Sum256([]byte(text))
	return Document{
		Bytes:             pdfBytes,
		PageCount:         pageCount,
		PageWidthPoints:   pageWidth,
		PageHeightPoints:  pageHeight,
		LogicalChars:      utf8.RuneCountInString(text),
		LogicalTextSHA256: hex.EncodeToString(sum[:]),
		PDFBytes:          len(pdfBytes),
		Base64Chars:       base64.StdEncoding.EncodedLen(len(pdfBytes)),
		FontAssetBytes:    len(fontData),
	}, nil
}

func splitVisualLines(pdf *gopdf.GoPdf, text string, width float64) ([]string, error) {
	logicalLines := strings.Split(text, "\n")
	visualLines := make([]string, 0, len(logicalLines))
	for _, logicalLine := range logicalLines {
		if logicalLine == "" {
			visualLines = append(visualLines, "")
			continue
		}
		wrapped, err := pdf.SplitText(logicalLine, width)
		if err != nil {
			return nil, err
		}
		visualLines = append(visualLines, wrapped...)
	}
	return visualLines, nil
}

func validateTextControls(text string) error {
	for _, r := range text {
		if r == '\n' {
			continue
		}
		if r < 0x20 || r == 0x7f {
			return fmt.Errorf("%w: control rune U+%04X", ErrUnsupportedText, r)
		}
	}
	return nil
}

func formatRunes(values map[rune]struct{}) string {
	formatted := make([]string, 0, len(values))
	for r := range values {
		formatted = append(formatted, fmt.Sprintf("U+%04X", r))
	}
	sort.Strings(formatted)
	return strings.Join(formatted, ",")
}
