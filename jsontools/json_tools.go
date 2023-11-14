package jsontools

import (
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
