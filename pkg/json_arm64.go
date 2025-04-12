//go:build arm64

package pkg

import (
	"fmt"

	"github.com/go-json-experiment/json"
	"github.com/go-json-experiment/json/jsontext"
)

func JSONMarshal(v interface{}) ([]byte, error) {
	data, err := json.Marshal(v, jsontext.CanonicalizeRawInts(true))
	if err != nil {
		return nil, fmt.Errorf("%w:, error: %+v", ErrJSONMarshal, err)
	}
	return data, nil
}

func JSONUnmarshal(data []byte, v interface{}) error {
	if err := json.Unmarshal(data, v); err != nil {
		return fmt.Errorf("%w: data=%s, error: %+v", ErrJSONUnmarshal, data, err)
	}
	return nil
}
