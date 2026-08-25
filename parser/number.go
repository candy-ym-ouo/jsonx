package parser
import (
	"math"
	"strconv"
)
// ParseIntFast parses the common integer path without allocating. It falls
// back to strconv for precise boundary errors.
func ParseIntFast(data []byte) (int64, error) {
	if len(data) == 0 {
		return 0, strconv.ErrSyntax
	}
	negative, i := false, 0
	if data[0] == '-' {
		negative, i = true, 1
	}
	var limit uint64 = math.MaxInt64
	if negative {
		limit++
	}
	var n uint64
	for ; i < len(data); i++ {
		c := data[i]
		if c < '0' || c > '9' || n > (limit-uint64(c-'0'))/10 {
			return strconv.ParseInt(string(data), 10, 64)
		}
		n = n*10 + uint64(c-'0')
	}
	if negative {
		if n == uint64(math.MaxInt64)+1 {
			return math.MinInt64, nil
		}
		return -int64(n), nil
	}
	return int64(n), nil
}
func ParseUintFast(data []byte) (uint64, error) {
	if len(data) == 0 {
		return 0, strconv.ErrSyntax
	}
	var n uint64
	for _, c := range data {
		if c < '0' || c > '9' || n > (math.MaxUint64-uint64(c-'0'))/10 {
			return strconv.ParseUint(string(data), 10, 64)
		}
		n = n*10 + uint64(c-'0')
	}
	return n, nil
}
func ParseFloat(data []byte) (float64, error) {
	// strconv uses an exact, heavily optimized Eisel-Lemire implementation and
	// is the correctness fallback prescribed by the project design.
	return strconv.ParseFloat(string(data), 64)
}
func IsInteger(text string) bool {
	for _, c := range text {
		if c == '.' || c == 'e' || c == 'E' {
			return false
		}
	}
	return true
}
