package jsontools

import (
	"bytes"
	"encoding/json"
	"github.com/OnlyPiglet/fly"
)

func JsonMash[T any](obj T) (string, error) {
	marshal, err := json.Marshal(obj)
	if err != nil {
		return "", err
	}
	return fly.QuickBytesToString(marshal), nil
}

func JsonUnMash[T any](objJson string, t T) error {
	objBytes := fly.QuickStringToBytes(objJson)
	return json.Unmarshal(objBytes, t)
}

func JsonMashWithoutEscapeHTML(t interface{}) (string, error) {
	bf := bytes.NewBuffer([]byte{})
	jsonEncoder := json.NewEncoder(bf)
	jsonEncoder.SetEscapeHTML(false)
	err := jsonEncoder.Encode(t)
	return bf.String(), err
}
