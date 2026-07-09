package variables

import (
	"strings"

	"github.com/clbanning/mxj/v2"
)

func decodeXML(data []byte) (any, error) {
	mv, err := mxj.NewMapXml(data)
	if err != nil {
		return nil, err
	}
	return normalizeXML(map[string]any(mv)), nil
}

func normalizeXML(src any) any {
	switch value := src.(type) {
	case map[string]any:
		out := make(map[string]any, len(value))
		for key, child := range value {
			out[xmlKey(key)] = normalizeXML(child)
		}
		return out
	case []any:
		out := make([]any, len(value))
		for i, child := range value {
			out[i] = normalizeXML(child)
		}
		return out
	default:
		return value
	}
}

func xmlKey(key string) string {
	if strings.HasPrefix(key, "-") {
		return "@" + strings.TrimPrefix(key, "-")
	}
	return key
}
