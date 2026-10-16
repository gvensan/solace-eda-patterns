package solace

import "strings"

// FieldInt pulls a numeric field out of a flat JSON payload without a full
// parse, which keeps a consumer's hot path cheap during a burst or with a
// large payload. It returns false when the field is missing or not a number.
func FieldInt(payload, field string) (int, bool) {
	key := `"` + field + `":`
	i := strings.Index(payload, key)
	if i < 0 {
		return 0, false
	}
	n, found := 0, false
	for _, c := range payload[i+len(key):] {
		if c < '0' || c > '9' {
			break
		}
		n = n*10 + int(c-'0')
		found = true
	}
	return n, found
}

// FieldString pulls a string field out of a flat JSON payload, or "".
func FieldString(payload, field string) string {
	key := `"` + field + `":"`
	i := strings.Index(payload, key)
	if i < 0 {
		return ""
	}
	rest := payload[i+len(key):]
	if j := strings.IndexByte(rest, '"'); j >= 0 {
		return rest[:j]
	}
	return rest
}
