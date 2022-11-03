package jsonHelpers

import "encoding/json"

func DesserializarJson[Para any](bytes []byte) (Para, error) {
	para := new(Para)
	err := json.Unmarshal(bytes, para)
	return *para, err
}
