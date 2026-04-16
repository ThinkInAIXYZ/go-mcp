package protocol

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/ThinkInAIXYZ/go-mcp/pkg"
)

type DataType string

const (
	ObjectT DataType = "object"
	Number  DataType = "number"
	Integer DataType = "integer"
	String  DataType = "string"
	Array   DataType = "array"
	Null    DataType = "null"
	Boolean DataType = "boolean"
)

// PropertyType is a JSON Schema "type" keyword: either a single type string or a union (array of strings).
type PropertyType []DataType

func (pt PropertyType) MarshalJSON() ([]byte, error) {
	if len(pt) == 0 {
		return json.Marshal("")
	}
	if len(pt) == 1 {
		return json.Marshal(string(pt[0]))
	}
	ss := make([]string, len(pt))
	for i, t := range pt {
		ss[i] = string(t)
	}
	return json.Marshal(ss)
}

func (pt *PropertyType) UnmarshalJSON(data []byte) error {
	if len(data) == 0 {
		*pt = nil
		return nil
	}
	if string(data) == "null" {
		*pt = nil
		return nil
	}
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		if s == "" {
			*pt = nil
			return nil
		}
		*pt = PropertyType{DataType(s)}
		return nil
	}
	var arr []string
	if err := json.Unmarshal(data, &arr); err != nil {
		return err
	}
	*pt = make(PropertyType, len(arr))
	for i, x := range arr {
		(*pt)[i] = DataType(x)
	}
	return nil
}

type Property struct {
	Type PropertyType `json:"type"`
	// Description is the description of the schema.
	Description string `json:"description,omitempty"`
	// Items specifies which data type an array contains, if the schema type is Array.
	Items *Property `json:"items,omitempty"`
	// Properties describes the properties of an object, if the schema type is Object.
	Properties map[string]*Property `json:"properties,omitempty"`
	Required   []string             `json:"required,omitempty"`
	Enum       []string             `json:"enum,omitempty"`
}

var schemaCache = pkg.SyncMap[*InputSchema]{}

func generateSchemaFromReqStruct(v any) (*InputSchema, error) {
	t := reflect.TypeOf(v)
	for t.Kind() != reflect.Struct {
		if t.Kind() != reflect.Ptr {
			return nil, fmt.Errorf("invalid type %v", t)
		}
		t = t.Elem()
	}

	typeUID := getTypeUUID(t)
	if schema, ok := schemaCache.Load(typeUID); ok {
		return schema, nil
	}

	schema := &InputSchema{Type: Object}

	property, err := reflectSchemaByObject(t)
	if err != nil {
		return nil, err
	}

	schema.Properties = property.Properties
	schema.Required = property.Required

	schemaCache.Store(typeUID, schema)
	return schema, nil
}

func getTypeUUID(t reflect.Type) string {
	if t.PkgPath() != "" && t.Name() != "" {
		return t.PkgPath() + "." + t.Name()
	}
	// fallback for unnamed types (like anonymous struct)
	return t.String()
}

func reflectSchemaByObject(t reflect.Type) (*Property, error) {
	var (
		properties     = make(map[string]*Property)
		requiredFields = make([]string, 0)
		enumValues     = make([]string, 0)
	)

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if !field.IsExported() {
			continue
		}

		jsonTag := field.Tag.Get("json")
		if jsonTag == "-" {
			continue
		}
		required := true
		if jsonTag == "" {
			jsonTag = field.Name
		}
		if strings.HasSuffix(jsonTag, ",omitempty") {
			jsonTag = strings.TrimSuffix(jsonTag, ",omitempty")
			required = false
		}

		item, err := reflectSchemaByType(field.Type)
		if err != nil {
			return nil, err
		}

		if description := field.Tag.Get("description"); description != "" {
			item.Description = description
		}
		properties[jsonTag] = item

		if s := field.Tag.Get("required"); s != "" {
			required, err = strconv.ParseBool(s)
			if err != nil {
				return nil, fmt.Errorf("invalid required field %v: %v", jsonTag, err)
			}
		}
		if required {
			requiredFields = append(requiredFields, jsonTag)
		}

		if v := field.Tag.Get("enum"); v != "" {
			enumValues = strings.Split(v, ",")
			for i := range enumValues {
				enumValues[i] = strings.TrimSpace(enumValues[i])
			}

			// Check if enum values are consistent with the field type
			for _, value := range enumValues {
				switch field.Type.Kind() {
				case reflect.String:
					// No additional processing required for string type
				case reflect.Int, reflect.Int64:
					if _, err := strconv.Atoi(value); err != nil {
						return nil, fmt.Errorf("enum value %q is not compatible with type %v", value, field.Type)
					}
				case reflect.Float64:
					if _, err := strconv.ParseFloat(value, 64); err != nil {
						return nil, fmt.Errorf("enum value %q is not compatible with type %v", value, field.Type)
					}
				default:
					return nil, fmt.Errorf("unsupported type %v for enum validation", field.Type)
				}
			}
			item.Enum = enumValues
		}
	}

	property := &Property{
		Type:       PropertyType{ObjectT},
		Properties: properties,
		Required:   requiredFields,
		Enum:       enumValues,
	}
	return property, nil
}

func reflectSchemaByType(t reflect.Type) (*Property, error) {
	s := &Property{}

	switch t.Kind() {
	case reflect.String:
		s.Type = PropertyType{String}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		s.Type = PropertyType{Integer}
	case reflect.Float32, reflect.Float64:
		s.Type = PropertyType{Number}
	case reflect.Bool:
		s.Type = PropertyType{Boolean}
	case reflect.Slice, reflect.Array:
		s.Type = PropertyType{Array}
		items, err := reflectSchemaByType(t.Elem())
		if err != nil {
			return nil, err
		}
		s.Items = items
	case reflect.Struct:
		object, err := reflectSchemaByObject(t)
		if err != nil {
			return nil, err
		}
		object.Type = PropertyType{ObjectT}
		s = object
	case reflect.Map:
		if t.Key().Kind() != reflect.String {
			return nil, fmt.Errorf("map key type %s is not supported", t.Key().Kind())
		}
		object := &Property{
			Type: PropertyType{ObjectT},
		}
		s = object
	case reflect.Ptr:
		p, err := reflectSchemaByType(t.Elem())
		if err != nil {
			return nil, err
		}
		s = p
	case reflect.Invalid, reflect.Uintptr, reflect.Complex64, reflect.Complex128,
		reflect.Chan, reflect.Func, reflect.Interface,
		reflect.UnsafePointer:
		return nil, fmt.Errorf("unsupported type: %s", t.Kind().String())
	default:
	}
	return s, nil
}
