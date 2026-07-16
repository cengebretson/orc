package telemetry

import (
	"bufio"
	"strings"
	"testing"
)

func TestReadLineCappedDiscardsOversizedLineAndContinues(t *testing.T) {
	reader := bufio.NewReaderSize(strings.NewReader("123456789\nnext\n"), 3)

	line, over, err := readLineCapped(reader, 5)
	if err != nil || !over || len(line) != 0 {
		t.Fatalf("oversized line = %q, over=%v, err=%v", line, over, err)
	}
	line, over, err = readLineCapped(reader, 5)
	if err != nil || over || string(line) != "next\n" {
		t.Fatalf("next line = %q, over=%v, err=%v", line, over, err)
	}
}
