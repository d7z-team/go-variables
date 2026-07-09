package variables

import "encoding/json"

func (v *Variables) LoadStruct(src any, opts ...LoadOption) error {
	data, err := json.Marshal(src)
	if err != nil {
		return err
	}
	value, err := decodeJSON(data)
	if err != nil {
		return err
	}

	return v.loadValue(value, newLoadOptions(opts))
}
