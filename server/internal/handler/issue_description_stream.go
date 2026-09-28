package handler

import (
	"encoding/json"
	"errors"
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

var errDescriptionStreamOutput = errors.New("invalid description stream output")

// descriptionTextStream decodes only the top-level JSON text string. This
// incremental scanner never treats provisional output as a validated result;
// the complete document still passes the ordinary JSON/Markdown validators.
type descriptionTextStream struct {
	rawBytes   int
	depth      int
	expectKey  bool
	key        string
	inString   bool
	stringKey  bool
	stringText bool
	escaped    bool
	keyBytes   []byte
	textSeen   bool
	textRunes  int
	utf8Tail   string
	decoder    descriptionJSONString
}

func (s *descriptionTextStream) Push(fragment string) (string, error) {
	s.rawBytes += len(fragment)
	if s.rawBytes > 180000 {
		return "", errDescriptionStreamOutput
	}
	var output strings.Builder
	for i := 0; i < len(fragment); i++ {
		c := fragment[i]
		if s.inString {
			if s.stringText {
				closed, err := s.decoder.push(c, &output)
				if err != nil {
					return "", err
				}
				if closed {
					s.inString = false
					s.stringText = false
				}
				continue
			}
			if s.stringKey {
				s.keyBytes = append(s.keyBytes, c)
			}
			if s.escaped {
				s.escaped = false
				continue
			}
			if c == '\\' {
				s.escaped = true
				continue
			}
			if c == '"' {
				s.inString = false
				if s.stringKey {
					if err := json.Unmarshal(s.keyBytes, &s.key); err != nil {
						return "", errDescriptionStreamOutput
					}
					s.stringKey = false
					s.expectKey = false
				}
			}
			continue
		}
		switch c {
		case '{', '[':
			s.depth++
			if s.depth == 1 && c == '{' {
				s.expectKey = true
			}
		case '}', ']':
			s.depth--
			if s.depth < 0 {
				return "", errDescriptionStreamOutput
			}
		case ',':
			if s.depth == 1 {
				s.expectKey = true
				s.key = ""
			}
		case '"':
			s.inString = true
			if s.depth == 1 && s.expectKey {
				s.stringKey = true
				s.keyBytes = []byte{'"'}
			} else if s.depth == 1 && s.key == "text" {
				if s.textSeen {
					return "", errDescriptionStreamOutput
				}
				s.stringText = true
				s.textSeen = true
			}
		}
	}
	// Upstream deltas usually contain whole runes, but preserving a partial UTF-8
	// tail makes the extractor independent of transport/chunk boundaries.
	decoded := s.utf8Tail + output.String()
	end := 0
	for end < len(decoded) && utf8.FullRuneInString(decoded[end:]) {
		r, size := utf8.DecodeRuneInString(decoded[end:])
		if r == utf8.RuneError && size == 1 {
			return "", errDescriptionStreamOutput
		}
		end += size
		s.textRunes++
	}
	s.utf8Tail = decoded[end:]
	if s.textRunes > 30000 || (!s.stringText && s.utf8Tail != "") {
		return "", errDescriptionStreamOutput
	}
	return decoded[:end], nil
}

// JSON escapes can straddle arbitrary deltas, including the two Unicode
// escapes that encode one supplementary-plane rune (for example an emoji).
type descriptionJSONString struct {
	escaped       bool
	hexRemaining  int
	code          rune
	highSurrogate rune
}

func (s *descriptionJSONString) push(c byte, out *strings.Builder) (bool, error) {
	if s.hexRemaining > 0 {
		var digit byte
		switch {
		case c >= '0' && c <= '9':
			digit = c - '0'
		case c >= 'a' && c <= 'f':
			digit = c - 'a' + 10
		case c >= 'A' && c <= 'F':
			digit = c - 'A' + 10
		default:
			return false, errDescriptionStreamOutput
		}
		s.code = s.code*16 + rune(digit)
		s.hexRemaining--
		if s.hexRemaining == 0 {
			s.emitRune(s.code, out)
		}
		return false, nil
	}
	if s.escaped {
		s.escaped = false
		switch c {
		case 'u':
			s.hexRemaining = 4
			s.code = 0
		case '"', '\\', '/':
			s.emitRune(rune(c), out)
		case 'n':
			s.emitRune('\n', out)
		case 'r':
			s.emitRune('\r', out)
		case 't':
			s.emitRune('\t', out)
		case 'b':
			s.emitRune('\b', out)
		case 'f':
			s.emitRune('\f', out)
		default:
			return false, errDescriptionStreamOutput
		}
		return false, nil
	}
	if c == '\\' {
		s.escaped = true
		return false, nil
	}
	if s.highSurrogate != 0 {
		out.WriteRune(utf8.RuneError)
		s.highSurrogate = 0
	}
	if c == '"' {
		return true, nil
	}
	if c < 0x20 {
		return false, errDescriptionStreamOutput
	}
	out.WriteByte(c)
	return false, nil
}

func (s *descriptionJSONString) emitRune(r rune, out *strings.Builder) {
	if s.highSurrogate != 0 {
		if r >= 0xdc00 && r <= 0xdfff {
			out.WriteRune(utf16.DecodeRune(s.highSurrogate, r))
			s.highSurrogate = 0
			return
		}
		out.WriteRune(utf8.RuneError)
		s.highSurrogate = 0
	}
	if r >= 0xd800 && r <= 0xdbff {
		s.highSurrogate = r
		return
	}
	if r >= 0xdc00 && r <= 0xdfff {
		r = utf8.RuneError
	}
	out.WriteRune(r)
}
