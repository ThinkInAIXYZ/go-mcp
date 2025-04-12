//go:build amd64

package pkg

import (
	"fmt"

	"github.com/bytedance/sonic"
)

var sonicAPI = sonic.Config{
	UseInt64:                true, // Effectively prevents integer overflow
	NoQuoteTextMarshaler:    true,
	NoValidateJSONMarshaler: true,
	NoValidateJSONSkip:      true,
}.Froze()

func JSONMarshal(v interface{}) ([]byte, error) {
	data, err := sonicAPI.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("%w:, error: %+v", ErrJSONMarshal, err)
	}
	return data, nil
}

func JSONUnmarshal(data []byte, v interface{}) error {
	if err := sonicAPI.Unmarshal(data, v); err != nil {
		return fmt.Errorf("%w: data=%s, error: %+v", ErrJSONUnmarshal, data, err)
	}
	return nil
}
