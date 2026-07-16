package telemetry

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
)

func scanJSONL(path string, visit func([]byte)) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close() //nolint:errcheck

	reader := bufio.NewReader(f)
	for {
		line, over, err := readLineCapped(reader, maxJSONLLineBytes)
		if !over && len(bytes.TrimSpace(line)) > 0 {
			visit(line)
		}
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
	}
}

func readLineCapped(reader *bufio.Reader, limit int) ([]byte, bool, error) {
	var line []byte
	over := false
	for {
		part, err := reader.ReadSlice('\n')
		if !over {
			if len(line)+len(part) <= limit {
				line = append(line, part...)
			} else {
				over = true
				line = nil
			}
		}
		if err == nil {
			return line, over, nil
		}
		if err == bufio.ErrBufferFull {
			continue
		}
		if err == io.EOF {
			return line, over, io.EOF
		}
		return nil, over, fmt.Errorf("read JSONL: %w", err)
	}
}
