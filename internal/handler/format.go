package handler

import "strconv"

func uintString(value uint) string {
	return strconv.FormatUint(uint64(value), 10)
}
