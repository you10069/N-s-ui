package util

import "fmt"

// StripJSONComments removes // and /* */ comments while preserving string
// contents, byte offsets and line breaks. The output can be decoded by the
// standard encoding/json package and error line numbers remain useful.
func StripJSONComments(input []byte) ([]byte, error) {
	output := append([]byte(nil), input...)
	inString := false
	escaped := false

	for i := 0; i < len(output); i++ {
		ch := output[i]
		if inString {
			if escaped {
				escaped = false
				continue
			}
			if ch == '\\' {
				escaped = true
				continue
			}
			if ch == '"' {
				inString = false
			}
			continue
		}

		if ch == '"' {
			inString = true
			continue
		}
		if ch != '/' || i+1 >= len(output) {
			continue
		}

		switch output[i+1] {
		case '/':
			output[i], output[i+1] = ' ', ' '
			i += 2
			for ; i < len(output) && output[i] != '\n' && output[i] != '\r'; i++ {
				output[i] = ' '
			}
			i--
		case '*':
			start := i
			output[i], output[i+1] = ' ', ' '
			i += 2
			closed := false
			for ; i < len(output); i++ {
				if i+1 < len(output) && output[i] == '*' && output[i+1] == '/' {
					output[i], output[i+1] = ' ', ' '
					i++
					closed = true
					break
				}
				if output[i] != '\n' && output[i] != '\r' {
					output[i] = ' '
				}
			}
			if !closed {
				return nil, fmt.Errorf("unterminated block comment at byte %d", start)
			}
		}
	}

	if inString {
		return nil, fmt.Errorf("unterminated JSON string")
	}
	return output, nil
}
