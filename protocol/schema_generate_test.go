package protocol

import (
	"reflect"
	"slices"
	"sort"
	"testing"
)

func TestGenerateSchemaFromReqStruct(t *testing.T) {
	type testData struct {
		String  string  `json:"string" description:"string"` // required
		Number  float64 `json:"number,omitempty"`            // optional
		Integer int     `json:"-"`                           // ignore

		String4Enum  string  `json:"string4enum,omitempty" enum:"a,b,c"`       // enum
		Integer4Enum int     `json:"integer4enum,omitempty" enum:"1,2,3"`      // enum
		Number4Enum  float64 `json:"number4enum,omitempty" enum:"1.1,2.2,3.3"` // enum
		Number4Enum2 int     `json:"number4enum2,omitempty" enum:"1,2,3"`      // enum
	}

	type anonymousTestDataWrapper struct {
		testData
		ExtraField string `json:"extraField,omitempty" description:"extra string enum" enum:"a,b,c"`
	}

	type testData4InvalidInteger4Enum struct {
		Integer4Enum int `json:"integer4enum,omitempty" enum:"a,b,c"`
	}

	type testData4InvalidNumber4Enum struct {
		Number4Enum float64 `json:"number4enum,omitempty" enum:"a,b,c"`
	}

	type testData4InvalidNumber4Enum2 struct {
		Number4Enum2 float64 `json:"number4enum2,omitempty" enum:"a,b,c"`
	}

	type testData4InvalidEnum struct {
		Enum byte `json:"enum,omitempty" enum:"a,b,c"`
	}

	type args struct {
		v any
	}
	tests := []struct {
		name    string
		args    args
		want    *InputSchema
		wantErr bool
	}{
		{
			name: "invalid type",
			args: args{
				v: 1,
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "struct type",
			args: args{
				v: testData{},
			},
			want: &InputSchema{
				Type: Object,
				Properties: map[string]*Property{
					"string": {
						Type:        PropertyType{String},
						Description: "string",
					},
					"number": {
						Type: PropertyType{Number},
					},
					"string4enum": {
						Type: PropertyType{String},
						Enum: []any{"a", "b", "c"},
					},
					"integer4enum": {
						Type: PropertyType{Integer},
						Enum: []any{1, 2, 3},
					},
					"number4enum": {
						Type: PropertyType{Number},
						Enum: []any{1.1, 2.2, 3.3},
					},
					"number4enum2": {
						Type: PropertyType{Integer},
						Enum: []any{1, 2, 3},
					},
				},
				Required: []string{"string"},
			},
		},
		{
			name: "struct point type",
			args: args{
				v: &testData{},
			},
			want: &InputSchema{
				Type: Object,
				Properties: map[string]*Property{
					"string": {
						Type:        PropertyType{String},
						Description: "string",
					},
					"number": {
						Type: PropertyType{Number},
					},
					"string4enum": {
						Type: PropertyType{String},
						Enum: []any{"a", "b", "c"},
					},
					"integer4enum": {
						Type: PropertyType{Integer},
						Enum: []any{1, 2, 3},
					},
					"number4enum": {
						Type: PropertyType{Number},
						Enum: []any{1.1, 2.2, 3.3},
					},
					"number4enum2": {
						Type: PropertyType{Integer},
						Enum: []any{1, 2, 3},
					},
				},
				Required: []string{"string"},
			},
		},
		{
			name: "anonymous nested struct type",
			args: args{
				v: anonymousTestDataWrapper{},
			},
			want: &InputSchema{
				Type: Object,
				Properties: map[string]*Property{
					"string": {
						Type:        PropertyType{String},
						Description: "string",
					},
					"extraField": {
						Type:        PropertyType{String},
						Description: "extra string enum",
						Enum:        []any{"a", "b", "c"},
					},
					"number": {
						Type: PropertyType{Number},
					},
					"string4enum": {
						Type: PropertyType{String},
						Enum: []any{"a", "b", "c"},
					},
					"integer4enum": {
						Type: PropertyType{Integer},
						Enum: []any{1, 2, 3},
					},
					"number4enum": {
						Type: PropertyType{Number},
						Enum: []any{1.1, 2.2, 3.3},
					},
					"number4enum2": {
						Type: PropertyType{Integer},
						Enum: []any{1, 2, 3},
					},
				},
				Required: []string{"string"},
			},
		},
		{
			name: "anonymous member collision",
			args: args{
				v: struct {
					testData
					String string `json:"string" description:"string"` // conflict with testData.string
				}{},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "invalid type for integer4Enum",
			args: args{
				v: &testData4InvalidInteger4Enum{},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "invalid type for number4Enum",
			args: args{
				v: &testData4InvalidNumber4Enum{},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "invalid type for number4Enum2",
			args: args{
				v: &testData4InvalidNumber4Enum2{},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "invalid type for enum",
			args: args{
				v: &testData4InvalidEnum{},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "anonymous struct type",
			args: args{
				v: struct {
					Name    string `json:"name" description:"user name"`
					Age     int    `json:"age,omitempty"`
					Address struct {
						City   string `json:"city"`
						Street string `json:"street,omitempty"`
					} `json:"address"`
				}{},
			},
			want: &InputSchema{
				Type: Object,
				Properties: map[string]*Property{
					"name": {
						Type:        PropertyType{String},
						Description: "user name",
					},
					"age": {
						Type: PropertyType{Integer},
					},
					"address": {
						Type: PropertyType{ObjectT},
						Properties: map[string]*Property{
							"city": {
								Type: PropertyType{String},
							},
							"street": {
								Type: PropertyType{String},
							},
						},
						Required: []string{"city"},
					},
				},
				Required: []string{"name", "address"},
			},
		},
		{
			name: "pointer to anonymous struct type",
			args: args{
				v: &struct {
					ID     int    `json:"id"`
					Email  string `json:"email,omitempty"`
					Active bool   `json:"active"`
				}{},
			},
			want: &InputSchema{
				Type: Object,
				Properties: map[string]*Property{
					"id": {
						Type: PropertyType{Integer},
					},
					"email": {
						Type: PropertyType{String},
					},
					"active": {
						Type: PropertyType{Boolean},
					},
				},
				Required: []string{"id", "active"},
			},
		},
		{
			name: "nested anonymous struct",
			args: args{
				v: struct {
					User struct {
						Name string `json:"name"`
						Info struct {
							Age    int  `json:"age,omitempty"`
							Active bool `json:"active"`
						} `json:"info"`
					} `json:"user"`
				}{},
			},
			want: &InputSchema{
				Type: Object,
				Properties: map[string]*Property{
					"user": {
						Type: PropertyType{ObjectT},
						Properties: map[string]*Property{
							"name": {
								Type: PropertyType{String},
							},
							"info": {
								Type: PropertyType{ObjectT},
								Properties: map[string]*Property{
									"age": {
										Type: PropertyType{Integer},
									},
									"active": {
										Type: PropertyType{Boolean},
									},
								},
								Required: []string{"active"},
							},
						},
						Required: []string{"name", "info"},
					},
				},
				Required: []string{"user"},
			},
		},
		{
			name: "map struct",
			args: args{
				v: struct {
					User struct {
						Name string            `json:"name"`
						Info map[string]string `json:"info"`
					} `json:"user"`
				}{},
			},
			want: &InputSchema{
				Type: Object,
				Properties: map[string]*Property{
					"user": {
						Type: PropertyType{ObjectT},
						Properties: map[string]*Property{
							"name": {
								Type: PropertyType{String},
							},
							"info": {
								Type: PropertyType{ObjectT},
							},
						},
						Required: []string{"name", "info"},
					},
				},
				Required: []string{"user"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := generateSchemaFromReqStruct(tt.args.v)
			if (err != nil) != tt.wantErr {
				t.Errorf("GenerateSchemaFromReqStruct() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !compareInputSchema(got, tt.want) {
				t.Errorf("GenerateSchemaFromReqStruct() got = %v, want %v", got, tt.want)
			}
		})
	}
}

// compareInputSchema Compare the contents of two InputSchema structures instead of comparing pointer addresses
func compareInputSchema(a, b *InputSchema) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	if a.Type != b.Type {
		return false
	}

	// compare required field
	if len(a.Required) != len(b.Required) {
		return false
	}
	aRequiredCopy := make([]string, len(a.Required))
	bRequiredCopy := make([]string, len(b.Required))
	copy(aRequiredCopy, a.Required)
	copy(bRequiredCopy, b.Required)
	sort.Strings(aRequiredCopy)
	sort.Strings(bRequiredCopy)
	for i := range aRequiredCopy {
		if aRequiredCopy[i] != bRequiredCopy[i] {
			return false
		}
	}

	// 比较Properties字段
	if len(a.Properties) != len(b.Properties) {
		return false
	}
	for k, aProp := range a.Properties {
		bProp, ok := b.Properties[k]
		if !ok {
			return false
		}
		if !compareProperty(aProp, bProp) {
			return false
		}
	}

	return true
}

// compareProperty Compare the contents of two Property structures
func compareProperty(a, b *Property) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	if !slices.Equal(a.Type, b.Type) {
		return false
	}
	if a.Description != b.Description {
		return false
	}

	// compare Items field
	if !compareProperty(a.Items, b.Items) {
		return false
	}
	// compare Properties field
	if len(a.Properties) != len(b.Properties) {
		return false
	}
	for k, aProp := range a.Properties {
		bProp, ok := b.Properties[k]
		if !ok {
			return false
		}
		if !compareProperty(aProp, bProp) {
			return false
		}
	}

	// compare Required field比
	if len(a.Required) != len(b.Required) {
		return false
	}
	aRequiredCopy := make([]string, len(a.Required))
	bRequiredCopy := make([]string, len(b.Required))
	copy(aRequiredCopy, a.Required)
	copy(bRequiredCopy, b.Required)
	sort.Strings(aRequiredCopy)
	sort.Strings(bRequiredCopy)
	for i := range aRequiredCopy {
		if aRequiredCopy[i] != bRequiredCopy[i] {
			return false
		}
	}

	// 比较Enum字段
	if !compareAnySlice(a.Enum, b.Enum) {
		return false
	}

	// 比较Default字段
	if !reflect.DeepEqual(a.Default, b.Default) {
		return false
	}

	return true
}

// compareAnySlice compares two []any slices for equality
func compareAnySlice(a, b []any) bool {
	if len(a) != len(b) {
		return false
	}

	// Create copies for sorting
	aCopy := make([]any, len(a))
	bCopy := make([]any, len(b))
	copy(aCopy, a)
	copy(bCopy, b)

	// Convert to string representations for sorting
	aStrings := make([]string, len(aCopy))
	bStrings := make([]string, len(bCopy))

	for i, v := range aCopy {
		aStrings[i] = reflect.ValueOf(v).String()
	}
	for i, v := range bCopy {
		bStrings[i] = reflect.ValueOf(v).String()
	}

	sort.Strings(aStrings)
	sort.Strings(bStrings)

	for i := range aStrings {
		if aStrings[i] != bStrings[i] {
			return false
		}
	}

	return true
}

func TestGenerateSchemaWithDefaultValues(t *testing.T) {
	type testDataWithDefaults struct {
		StringWithDefault string   `json:"string_with_default,omitempty" default:"hello"`
		IntWithDefault    int      `json:"int_with_default,omitempty" default:"42"`
		FloatWithDefault  float64  `json:"float_with_default,omitempty" default:"3.14"`
		BoolWithDefault   bool     `json:"bool_with_default,omitempty" default:"true"`
		RequiredString    string   `json:"required_string" description:"required field"`
		ArrayWithDefault  []string `json:"array_with_default,omitempty" default:"[\"item1\",\"item2\"]"`
	}

	type testDataInvalidDefaults struct {
		InvalidIntDefault   int     `json:"invalid_int,omitempty" default:"not_a_number"`
		InvalidFloatDefault float64 `json:"invalid_float,omitempty" default:"not_a_float"`
		InvalidBoolDefault  bool    `json:"invalid_bool,omitempty" default:"not_a_bool"`
	}

	tests := []struct {
		name    string
		input   any
		want    *InputSchema
		wantErr bool
	}{
		{
			name:  "struct with valid default values",
			input: testDataWithDefaults{},
			want: &InputSchema{
				Type: Object,
				Properties: map[string]*Property{
					"string_with_default": {
						Type:    PropertyType{String},
						Default: "hello",
					},
					"int_with_default": {
						Type:    PropertyType{Integer},
						Default: 42,
					},
					"float_with_default": {
						Type:    PropertyType{Number},
						Default: 3.14,
					},
					"bool_with_default": {
						Type:    PropertyType{Boolean},
						Default: true,
					},
					"required_string": {
						Type:        PropertyType{String},
						Description: "required field",
					},
					"array_with_default": {
						Type: PropertyType{Array},
						Items: &Property{
							Type: PropertyType{String},
						},
						Default: "[\"item1\",\"item2\"]",
					},
				},
				Required: []string{"required_string"},
			},
			wantErr: false,
		},
		{
			name:    "struct with invalid int default",
			input:   testDataInvalidDefaults{},
			want:    nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := generateSchemaFromReqStruct(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("generateSchemaFromReqStruct() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !compareInputSchema(got, tt.want) {
				t.Errorf("generateSchemaFromReqStruct() got = %v, want %v", got, tt.want)
			}
		})
	}
}
