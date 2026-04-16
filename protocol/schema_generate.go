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
	Enum       []any                `json:"enum,omitempty"`
	// Default specifies the default value for the property.
	Default any `json:"default,omitempty"`
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
		properties      = make(map[string]*Property)
		requiredFields  = make([]string, 0)
		anonymousFields = make([]reflect.StructField, 0)
	)

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)

		if field.Anonymous {
			anonymousFields = append(anonymousFields, field)
			continue
		}

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
			enumStrings := strings.Split(v, ",")
			enumValues := make([]any, len(enumStrings))

			for j, value := range enumStrings {
				value = strings.TrimSpace(value)

				// Convert string values to appropriate types based on field type
				switch field.Type.Kind() {
				case reflect.String:
					enumValues[j] = value
				case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
					intVal, err := strconv.Atoi(value)
					if err != nil {
						return nil, fmt.Errorf("enum value %q is not compatible with integer type %v", value, field.Type)
					}
					enumValues[j] = intVal
				case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
					uintVal, err := strconv.ParseUint(value, 10, 64)
					if err != nil {
						return nil, fmt.Errorf("enum value %q is not compatible with unsigned integer type %v", value, field.Type)
					}
					enumValues[j] = uintVal
				case reflect.Float32, reflect.Float64:
					floatVal, err := strconv.ParseFloat(value, 64)
					if err != nil {
						return nil, fmt.Errorf("enum value %q is not compatible with float type %v", value, field.Type)
					}
					enumValues[j] = floatVal
				case reflect.Bool:
					boolVal, err := strconv.ParseBool(value)
					if err != nil {
						return nil, fmt.Errorf("enum value %q is not compatible with boolean type %v", value, field.Type)
					}
					enumValues[j] = boolVal
				default:
					return nil, fmt.Errorf("unsupported type %v for enum validation", field.Type)
				}
			}
			item.Enum = enumValues
		}

		// Handle default value
		if defaultValue := field.Tag.Get("default"); defaultValue != "" {
			// Convert string value to appropriate type based on field type
			switch field.Type.Kind() {
			case reflect.String:
				item.Default = defaultValue
			case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
				intVal, err := strconv.Atoi(defaultValue)
				if err != nil {
					return nil, fmt.Errorf("default value %q is not compatible with integer type %v", defaultValue, field.Type)
				}
				item.Default = intVal
			case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
				uintVal, err := strconv.ParseUint(defaultValue, 10, 64)
				if err != nil {
					return nil, fmt.Errorf("default value %q is not compatible with unsigned integer type %v", defaultValue, field.Type)
				}
				item.Default = uintVal
			case reflect.Float32, reflect.Float64:
				floatVal, err := strconv.ParseFloat(defaultValue, 64)
				if err != nil {
					return nil, fmt.Errorf("default value %q is not compatible with float type %v", defaultValue, field.Type)
				}
				item.Default = floatVal
			case reflect.Bool:
				boolVal, err := strconv.ParseBool(defaultValue)
				if err != nil {
					return nil, fmt.Errorf("default value %q is not compatible with boolean type %v", defaultValue, field.Type)
				}
				item.Default = boolVal
			default:
				// For complex types (arrays, objects), keep as string
				// The consumer can parse it as needed
				item.Default = defaultValue
			}
		}
	}

	for _, field := range anonymousFields {
		object, err := reflectSchemaByObject(field.Type)
		if err != nil {
			return nil, err
		}
		for propName, propValue := range object.Properties {
			if _, ok := properties[propName]; ok {
				return nil, fmt.Errorf("duplicate property name %s in anonymous struct", propName)
			}
			properties[propName] = propValue
		}
		requiredFields = append(requiredFields, object.Required...)
	}

	property := &Property{
		Type:       PropertyType{ObjectT},
		Properties: properties,
		Required:   requiredFields,
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
