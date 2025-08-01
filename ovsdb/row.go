package ovsdb

import "encoding/json"

// Row is a table Row according to RFC7047
type Row map[string]any

// UnmarshalJSON unmarshalls a byte array to an OVSDB Row
func (r *Row) UnmarshalJSON(b []byte) (err error) {
	*r = make(map[string]any)
	var raw map[string]any
	err = json.Unmarshal(b, &raw)
	for key, val := range raw {
		val, err = ovsSliceToGoNotation(val)
		if err != nil {
			return err
		}
		(*r)[key] = val
	}
	return err
}

// NewRow returns a new empty row
func NewRow() Row {
	return Row(make(map[string]any))
}
