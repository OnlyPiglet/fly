package signtools

import (
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
)

type SignFormat string

const (
	Hex    SignFormat = "Hex"
	Base64 SignFormat = "Base64"
)

func MD5Sign(content []byte, format SignFormat) (string, error) {
	md5Hash := md5.New()
	md5Hash.Write(content)
	switch format {
	case Hex:
		return hex.EncodeToString(md5Hash.Sum(nil)), nil
	case Base64:
		return base64.StdEncoding.EncodeToString(md5Hash.Sum(nil)), nil
	default:
		return "", errors.New(fmt.Sprintf("unknown sign format %s", format))
	}
}
