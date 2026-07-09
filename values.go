package variables

import "strconv"

func inferScalar(src string) any {
	if src == "null" {
		return nil
	}
	if b, err := strconv.ParseBool(src); err == nil {
		return b
	}
	if i, err := strconv.ParseInt(src, 10, 64); err == nil {
		return i
	}
	if f, err := strconv.ParseFloat(src, 64); err == nil {
		return f
	}
	return src
}
