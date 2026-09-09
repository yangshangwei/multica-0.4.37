// This file is the canonical boundary matrix for DecodeTextFileBytes. The CLI
// suites that call it (cmd/multica/cmd_issue_test.go and friends) keep only
// their wiring case and point here rather than re-running this table through a
// command.
package util

import (
	"encoding/binary"
	"strings"
	"testing"
	"unicode/utf16"
)

// encodeUTF16 builds the byte shape PowerShell writes, so the test data is
// derived the same way the real files are rather than hand-transcribed.
func encodeUTF16(t *testing.T, s string, order binary.ByteOrder, bom bool) []byte {
	t.Helper()
	units := utf16.Encode([]rune(s))
	out := make([]byte, 0, len(units)*2+2)
	if bom {
		out = append(out, 0xFE, 0xFF)
		if order == binary.LittleEndian {
			out[0], out[1] = 0xFF, 0xFE
		}
	}
	for _, u := range units {
		var b [2]byte
		order.PutUint16(b[:], u)
		out = append(out, b[0], b[1])
	}
	return out
}

func TestDecodeTextFileBytes(t *testing.T) {
	const mixed = "标题 / Заголовок / 中文 / 😀"

	t.Run("decodes", func(t *testing.T) {
		cases := []struct {
			name string
			data []byte
			want string
		}{
			{"empty stays empty", nil, ""},
			{"plain ASCII", []byte("hello\nworld"), "hello\nworld"},
			{"UTF-8 multi-script", []byte(mixed), mixed},
			{
				// PowerShell 5.1's `-Encoding utf8` emits this BOM. It is an
				// encoding marker, not content, so it must not reach the API.
				"UTF-8 with BOM strips the marker",
				append([]byte{0xEF, 0xBB, 0xBF}, []byte(mixed)...),
				mixed,
			},
			{"UTF-8 BOM alone decodes to empty", []byte{0xEF, 0xBB, 0xBF}, ""},
			{"UTF-16LE BOM (PowerShell Out-File)", encodeUTF16(t, mixed, binary.LittleEndian, true), mixed},
			{"UTF-16BE BOM", encodeUTF16(t, mixed, binary.BigEndian, true), mixed},
			{"UTF-16LE BOM, surrogate pair only", encodeUTF16(t, "😀", binary.LittleEndian, true), "😀"},
			{"UTF-16LE BOM alone decodes to empty", []byte{0xFF, 0xFE}, ""},
			{"UTF-16BE BOM alone decodes to empty", []byte{0xFE, 0xFF}, ""},
			{"UTF-16LE BOM keeps CRLF", encodeUTF16(t, "a\r\nb", binary.LittleEndian, true), "a\r\nb"},
			{
				// No BOM, but every unit is ASCII-range, so the endianness is
				// provable. This is the shape utf8.Valid alone lets through.
				"BOM-less UTF-16LE, all-ASCII",
				encodeUTF16(t, "hi there", binary.LittleEndian, false),
				"hi there",
			},
			{
				"BOM-less UTF-16BE, all-ASCII",
				encodeUTF16(t, "hi there", binary.BigEndian, false),
				"hi there",
			},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				got, err := DecodeTextFileBytes(tc.data, "test input")
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if got != tc.want {
					t.Errorf("got %q, want %q", got, tc.want)
				}
			})
		}
	})

	t.Run("refuses", func(t *testing.T) {
		cases := []struct {
			name     string
			data     []byte
			wantText string
		}{
			{
				// `Set-Content` on a Chinese install. Decoding needs a code-page
				// guess, and a wrong guess reads as plausible text, so refuse.
				"CP936 ANSI without BOM",
				[]byte{0xD6, 0xD0, 0xCE, 0xC4},
				"no BOM to decode from",
			},
			{
				"CP1251 ANSI without BOM",
				[]byte{0xD0, 0xF3, 0xF1, 0xF1, 0xEA, 0xE8, 0xE9},
				"no BOM to decode from",
			},
			{"truncated UTF-8 rune", []byte{0xE4, 0xB8}, "no BOM to decode from"},
			{
				"UTF-16 with an odd trailing byte",
				append(encodeUTF16(t, "hi", binary.LittleEndian, true), 0x41),
				"odd trailing byte",
			},
			{
				"unpaired high surrogate",
				[]byte{0xFF, 0xFE, 0x3D, 0xD8, 0x41, 0x00},
				"unpaired high surrogate",
			},
			{
				"unpaired low surrogate",
				[]byte{0xFF, 0xFE, 0x00, 0xDE, 0x41, 0x00},
				"unpaired low surrogate",
			},
			{
				// Valid UTF-8 byte-wise, but NUL aborts the INSERT in PostgreSQL
				// (SQLSTATE 22021), so it is caught at the encoding boundary
				// where the error can still name a file and a fix.
				"NUL inside otherwise-valid UTF-8",
				[]byte("ab\x00cd"),
				"contains NUL bytes",
			},
			{
				// The documented cost of proving endianness instead of guessing
				// it: BOM-less UTF-16LE "😀" carries its lone NUL on an even
				// offset, which a majority-based sniff would read as big-endian
				// and decode to "㷘Þ". Refused rather than silently mangled.
				"BOM-less UTF-16LE surrogate pair is refused, not guessed",
				encodeUTF16(t, "😀", binary.LittleEndian, false),
				"contains NUL bytes",
			},
			{
				"BOM-less UTF-16LE CJK is refused, not guessed",
				encodeUTF16(t, "中文标题", binary.LittleEndian, false),
				"no BOM to decode from",
			},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				_, err := DecodeTextFileBytes(tc.data, "file content for --description-file")
				if err == nil {
					t.Fatal("expected an error")
				}
				if !strings.Contains(err.Error(), tc.wantText) {
					t.Errorf("error %q does not mention %q", err, tc.wantText)
				}
				// Every refusal has to name its flag and carry the recovery
				// command: the reader is an agent in a tool loop, and an error
				// it cannot act on costs a retry turn or wedges the task.
				if !strings.Contains(err.Error(), "--description-file") {
					t.Errorf("error %q does not name the flag", err)
				}
				if !strings.Contains(err.Error(), "UTF8Encoding") {
					t.Errorf("error %q does not carry the PowerShell fix command", err)
				}
			})
		}
	})

	t.Run("round-trips every PowerShell writer for the same body", func(t *testing.T) {
		// The point of the helper: which cmdlet the agent reached for stops
		// being observable in the stored content.
		for name, data := range map[string][]byte{
			"Out-File / >":           encodeUTF16(t, mixed, binary.LittleEndian, true),
			"-Encoding unicode":      encodeUTF16(t, mixed, binary.LittleEndian, true),
			"-Encoding bigendianuni": encodeUTF16(t, mixed, binary.BigEndian, true),
			"-Encoding utf8 (5.1)":   append([]byte{0xEF, 0xBB, 0xBF}, []byte(mixed)...),
			"UTF8Encoding($false)":   []byte(mixed),
		} {
			got, err := DecodeTextFileBytes(data, "test input")
			if err != nil {
				t.Errorf("%s: unexpected error: %v", name, err)
				continue
			}
			if got != mixed {
				t.Errorf("%s: got %q, want %q", name, got, mixed)
			}
		}
	})
}
