package protocol

import "fmt"

// Hexa32ToString32 converts a number to Scouter's Hexa32 base-32 string format.
// Matches Java scouter.util.Hexa32.toString32().
// Negative numbers are prefixed with 'z', numbers >= 10 are prefixed with 'x',
// and numbers 0-9 are returned as plain decimal strings.
func Hexa32ToString32(num int64) string {
	if num < 0 {
		if num == -9223372036854775808 { // Long.MIN_VALUE
			return "z8000000000000"
		}
		return fmt.Sprintf("z%s", formatBase32(-num))
	}
	if num < 10 {
		return fmt.Sprintf("%d", num)
	}
	return fmt.Sprintf("x%s", formatBase32(num))
}

// formatBase32 formats a non-negative int64 as base-32 string
// using Java's Long.toString(num, 32) character set: 0-9, a-v
func formatBase32(num int64) string {
	const digits = "0123456789abcdefghijklmnopqrstuv"
	if num == 0 {
		return "0"
	}
	buf := make([]byte, 0, 13)
	for num > 0 {
		buf = append([]byte{digits[num%32]}, buf...)
		num /= 32
	}
	return string(buf)
}
