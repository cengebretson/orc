package telemetry

import (
	"bufio"
	"fmt"
	"io"
)

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
