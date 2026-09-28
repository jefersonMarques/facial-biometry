package cnh

import (
	"regexp"
	"testing"
)

func TestClosestContentsToByteRangeSupportsContentsBeforeByteRange(t *testing.T) {
	pdf := []byte("<< /Contents <AABBCCDD> /ByteRange [0 10 20 30] >>")

	contents := nativeContentsPattern.FindAllSubmatchIndex(pdf, -1)
	byteRanges := nativeByteRangePattern.FindAllSubmatchIndex(pdf, -1)
	if len(contents) != 1 || len(byteRanges) != 1 {
		t.Fatal("expected one Contents and one ByteRange")
	}

	match := closestContentsToByteRange(contents, byteRanges[0][0], byteRanges[0][1])
	if match == nil {
		t.Fatal("expected /Contents before /ByteRange to be associated")
	}
	if got := string(regexp.MustCompile(`\s+`).ReplaceAll(pdf[match[2]:match[3]], nil)); got != "AABBCCDD" {
		t.Fatalf("unexpected contents: %q", got)
	}
}

func TestByteRangeCoversWholeFile(t *testing.T) {
	if !byteRangeCoversWholeFile([4]int64{0, 100, 200, 50}, 250) {
		t.Fatal("expected whole-file coverage")
	}
	if byteRangeCoversWholeFile([4]int64{0, 100, 200, 49}, 250) {
		t.Fatal("expected partial coverage to be rejected")
	}
}
