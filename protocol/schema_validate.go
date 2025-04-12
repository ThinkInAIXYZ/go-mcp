package protocol

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"

	"github.com/ThinkInAIXYZ/go-mcp/pkg"
)

func VerifyAndUnmarshal(content json.RawMessage, v any) error {
	t := reflect.TypeOf(v)
	for t.Kind() != reflect.Struct {
		if t.Kind() != reflect.Ptr {
			return fmt.Errorf("invalid type %v, plz use func `pkg.JSONUnmarshal` instead", t)
		}
		t = t.Elem()
	}

	typeUID := getTypeUUID(t)
	schema, ok := schemaCache.Load(typeUID)
	if !ok {
		return fmt.Errorf("schema has not been generated，unable to verify: plz use func `pkg.JSONUnmarshal` instead")
	}

	return VerifySchemaAndUnmarshal(Property{
		Type:       ObjectT,
		Properties: schema.Properties,
		Required:   schema.Required,
	}, content, v)
}

func VerifySchemaAndUnmarshal(schema Property, content []byte, v any) error {
	var data any
	err := pkg.JSONUnmarshal(content, &data)
	if err != nil {
		return err
	}
	if !Validate(schema, data) {
		return errors.New("data validation failed against the provided schema")
	}
	return pkg.JSONUnmarshal(content, &v)
}

func Validate(schema Property, data any) bool {
	switch schema.Type {
	case ObjectT:
		return validateObject(schema, data)
	case Array:
		return validateArray(schema, data)
	case String:
		_, ok := data.(string)
		return ok
	case Number: // float64 and int
		_, ok := data.(float64)
		if !ok {
			_, ok = data.(int)
		}
		return ok
	case Boolean:
		_, ok := data.(bool)
		return ok
	case Integer:
		// Golang unmarshals all numbers as float64, so we need to check if the float64 is an integer
		if num, ok := data.(float64); ok {
			return num == float64(int64(num))
		}
		_, ok := data.(int)
		if ok {
			return true
		}
		_, ok = data.(int64)
		return ok
	case Null:
		return data == nil
	default:
		return false
	}
}

func validateObject(schema Property, data any) bool {
	dataMap, ok := data.(map[string]any)
	if !ok {
		return false
	}
	for _, field := range schema.Required {
		if _, exists := dataMap[field]; !exists {
			return false
		}
	}
	for key, valueSchema := range schema.Properties {
		value, exists := dataMap[key]
		if exists && !Validate(*valueSchema, value) {
			return false
		}
	}
	return true
}

func validateArray(schema Property, data any) bool {
	dataArray, ok := data.([]any)
	if !ok {
		return false
	}
	for _, item := range dataArray {
		if !Validate(*schema.Items, item) {
			return false
		}
	}
	return true
}
