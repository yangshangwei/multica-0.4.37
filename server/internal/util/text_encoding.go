package util

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

var (
	bomUTF8    = []byte{0xEF, 0xBB, 0xBF}
	bomUTF16LE = []byte{0xFF, 0xFE}
	bomUTF16BE = []byte{0xFE, 0xFF}
)

// encodingFixHint is appended to every decode failure. The caller is usually an
// agent reading stderr inside its own tool loop, so the recovery command has to
// be in the error itself — a description of the problem alone costs a whole
// retry turn, or wedges the task when the model doesn't infer the fix.
const encodingFixHint = ". Rewrite it as UTF-8 before retrying — in PowerShell: " +
	`$utf8 = New-Object System.Text.UTF8Encoding($false); ` +
	`[IO.File]::WriteAllText((Join-Path $PWD 'body.md'), $body, $utf8)`

// DecodeTextFileBytes converts the raw bytes of an agent- or human-authored
// text file (or a piped stdin body) into a string, decoding the encodings a
// Windows caller produces by accident and refusing the ones that cannot be
// identified without guessing.
//
// Windows PowerShell 5.1 picks a non-UTF-8 encoding unless it is told
// otherwise, and which one depends on the cmdlet rather than on anything the
// author chose:
//
//   - `Out-File` and `>` write UTF-16LE with a BOM.
//   - `Set-Content` writes the machine ANSI code page (CP936 on a Chinese
//     install, CP1251 on a Russian one, ...).
//   - Only an explicit UTF8Encoding writer produces UTF-8, and PowerShell
//     5.1's `-Encoding utf8` still prefixes a BOM.
//
// The split between decoding and refusing is deliberate, and it is a
// correctness line rather than a convenience one:
//
//   - A BOM is an unambiguous, in-band declaration of the encoding. Decoding
//     it is lossless and cannot invent content, so a mis-encoded file becomes
//     a non-event instead of a failed task.
//   - BOM-less ANSI carries no such declaration. Decoding it means guessing a
//     code page, and a wrong guess yields plausible-looking wrong characters —
//     mojibake that reads as text and so survives review. That is strictly
//     worse than an error, because the failure becomes invisible. Such input
//     is refused even though the bytes could be force-decoded.
//
// NUL is refused for the same reason it is stripped on the persistence side
// (see SanitizeTextForPostgres): PostgreSQL rejects it in TEXT with SQLSTATE
// 22021. Catching it here, where the encoding boundary actually is, turns an
// opaque database error into an actionable one naming the file and the fix.
func DecodeTextFileBytes(data []byte, label string) (string, error) {
	switch {
	case bytes.HasPrefix(data, bomUTF8):
		data = data[len(bomUTF8):]
	case bytes.HasPrefix(data, bomUTF16LE):
		return decodeUTF16(data[len(bomUTF16LE):], binary.LittleEndian, label)
	case bytes.HasPrefix(data, bomUTF16BE):
		return decodeUTF16(data[len(bomUTF16BE):], binary.BigEndian, label)
	}

	// Overwhelmingly the common case: a well-formed UTF-8 body, checked before
	// any sniffing so the normal path stays a single pass.
	if utf8.Valid(data) && bytes.IndexByte(data, 0) < 0 {
		return string(data), nil
	}

	// A BOM-less UTF-16 body whose text is mostly ASCII is still identifiable
	// from its NUL pattern, and it has to be caught here: every one of its
	// bytes is individually valid UTF-8, so utf8.Valid alone waves it through
	// and the embedded NULs travel all the way to the database.
	if order, ok := sniffBOMlessUTF16(data); ok {
		return decodeUTF16(data, order, label)
	}

	if bytes.IndexByte(data, 0) >= 0 {
		return "", fmt.Errorf(
			"%s contains NUL bytes, so it is not usable text (binary content, or UTF-16 too short or too non-ASCII to identify without a BOM)%s",
			label, encodingFixHint)
	}
	return "", fmt.Errorf(
		"%s is not valid UTF-8 and carries no BOM to decode from; Windows PowerShell 5.1 writes UTF-16LE for `Out-File`/`>` "+
			"and the machine ANSI code page (CP936, CP1251, ...) for `Set-Content`. UTF-16 with a BOM is decoded automatically, "+
			"but BOM-less ANSI is indistinguishable from corrupt bytes and is refused rather than guessed at%s",
		label, encodingFixHint)
}

// decodeUTF16 decodes a BOM-stripped UTF-16 body of known endianness, refusing
// the malformed shapes instead of letting utf16.Decode replace them with U+FFFD.
// A silent U+FFFD here would be indistinguishable from content the author
// actually wrote, which is the specific outcome this whole path exists to avoid.
func decodeUTF16(body []byte, order binary.ByteOrder, label string) (string, error) {
	if len(body) == 0 {
		return "", nil
	}
	if len(body)%2 != 0 {
		return "", fmt.Errorf("%s looks like UTF-16 but has an odd trailing byte, so the file is truncated%s", label, encodingFixHint)
	}

	units := make([]uint16, len(body)/2)
	for i := range units {
		units[i] = order.Uint16(body[i*2:])
	}
	for i, u := range units {
		switch {
		case u >= 0xD800 && u <= 0xDBFF:
			if i+1 >= len(units) || units[i+1] < 0xDC00 || units[i+1] > 0xDFFF {
				return "", fmt.Errorf("%s is UTF-16 with an unpaired high surrogate at unit %d, so the file is corrupt%s", label, i, encodingFixHint)
			}
		case u >= 0xDC00 && u <= 0xDFFF:
			// A low surrogate whose predecessor was a high surrogate was
			// already validated as a pair on the previous iteration.
			if i == 0 || units[i-1] < 0xD800 || units[i-1] > 0xDBFF {
				return "", fmt.Errorf("%s is UTF-16 with an unpaired low surrogate at unit %d, so the file is corrupt%s", label, i, encodingFixHint)
			}
		}
	}

	decoded := string(utf16.Decode(units))
	if strings.ContainsRune(decoded, 0) {
		return "", fmt.Errorf("%s decodes from UTF-16 but contains a NUL character, so it is not usable text%s", label, encodingFixHint)
	}
	return decoded, nil
}

// sniffBOMlessUTF16 reports the endianness of a UTF-16 body that lost its BOM,
// and does so only for the one shape that is provable rather than likely: text
// whose every 16-bit unit is ASCII-range. Such a unit always has a NUL half, so
// UTF-16LE puts a NUL at every odd offset and none at any even one, and
// UTF-16BE the reverse. Requiring the NUL count to equal the unit count exactly
// — not merely to be a majority — is what makes this a proof.
//
// That exact-match bar is load-bearing, and a majority test is not a safe
// relaxation of it. BOM-less UTF-16LE "😀" is 3D D8 00 DE: one NUL, sitting on
// an even offset because U+1F600's low surrogate happens to be 0xDE00. Any
// threshold loose enough to accept it reads it as big-endian and yields "㷘Þ" —
// a clean decode into the wrong characters, which is the precise failure this
// file exists to prevent. Under exact matching it is refused instead.
//
// So this deliberately identifies less than it could. Mixed-script and CJK
// UTF-16 fall through to an error, and that is the intended trade: those files
// come from PowerShell's `Out-File`, which always writes the BOM that makes
// them decodable for real. What is caught here is the case that has no other
// defence — pure-ASCII UTF-16, whose every byte is individually valid UTF-8, so
// utf8.Valid alone waves it through and its NULs reach PostgreSQL.
func sniffBOMlessUTF16(data []byte) (binary.ByteOrder, bool) {
	if len(data) < 2 || len(data)%2 != 0 {
		return nil, false
	}

	var evenNULs, oddNULs int
	for i, b := range data {
		if b != 0 {
			continue
		}
		if i%2 == 0 {
			evenNULs++
		} else {
			oddNULs++
		}
	}

	units := len(data) / 2
	switch {
	case oddNULs == units && evenNULs == 0:
		return binary.LittleEndian, true
	case evenNULs == units && oddNULs == 0:
		return binary.BigEndian, true
	}
	return nil, false
}
