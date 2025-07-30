package client

import (
	"testing"

	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testValidationModel struct {
	Name        string            `validate:"min=1,max=64"`
	Age         int               `validate:"min=0,max=150"`
	Email       string            `validate:"omitempty,email"`
	Tags        []string          `validate:"dive,max=10"`
	Score       *int              `validate:"omitempty,min=0,max=100"`
	FloodVLANs  []int             `validate:"max=4096,dive,min=0,max=4095"`
	Protocols   []string          `validate:"dive,oneof='OpenFlow10' 'OpenFlow11' 'OpenFlow12' 'OpenFlow13' 'OpenFlow14' 'OpenFlow15'"`
	Mappings    map[int]int       `validate:"dive,keys,min=0,max=16777215,endkeys,min=0,max=4095"`
	Config      map[string]string `validate:"dive,keys,min=1,max=32,endkeys,min=1,max=256"`
	KeyConfig   map[int]int       `validate:"dive,keys,min=0,max=10"`
	ValueConfig map[string]int    `validate:"dive,min=0,max=10"`
	IsActive    bool
}

type testValidationNestedModel struct {
	ID     string              `validate:"uuid"`
	Detail testValidationModel `validate:"required"`
}

func intPtr(i int) *int {
	return &i
}

func TestValidateModel_Nil(t *testing.T) {
	err := validateModel(nil)
	require.Error(t, err)
	assert.ErrorContains(t, err, "model cannot be nil")
}

func TestValidateModel_Valid(t *testing.T) {
	model := testValidationModel{
		Name:        "Test Name",
		Age:         30,
		Email:       "test@example.com",
		Tags:        []string{"tag1", "tag2"},
		Score:       intPtr(85),
		FloodVLANs:  []int{100, 200, 4095},
		Protocols:   []string{"OpenFlow13", "OpenFlow15"},
		Mappings:    map[int]int{100: 500, 200: 1000, 16777215: 4095},
		Config:      map[string]string{"key1": "value1", "key2": "value2"},
		KeyConfig:   map[int]int{1: 1, 10: 10},
		ValueConfig: map[string]int{"key1": 1, "key2": 9},
		IsActive:    true,
	}

	err := validateModel(&model)
	require.NoError(t, err)
}

func TestValidateModel_Invalid(t *testing.T) {
	tests := []struct {
		name          string
		model         testValidationModel
		expectedError string
	}{
		{
			name: "invalid empty name",
			model: testValidationModel{
				Name: "",
				Age:  30,
			},
			expectedError: "field 'testValidationModel.Name' (value: '') failed on rule 'min' (param: 1)",
		},
		{
			name: "invalid age",
			model: testValidationModel{
				Name: "Test Name",
				Age:  -5,
			},
			expectedError: "field 'testValidationModel.Age' (value: '-5') failed on rule 'min' (param: 0)",
		},
		{
			name: "invalid email",
			model: testValidationModel{
				Name:  "Test Name",
				Age:   30,
				Email: "not-an-email",
			},
			expectedError: "field 'testValidationModel.Email' (value: 'not-an-email') failed on rule 'email'",
		},
		{
			name: "invalid score",
			model: testValidationModel{
				Name:  "Test Name",
				Age:   30,
				Score: intPtr(150),
			},
			expectedError: "field 'testValidationModel.Score' (value: '150') failed on rule 'max' (param: 100)",
		},
		{
			name: "invalid FloodVLAN value (too high)",
			model: testValidationModel{
				Name:       "Test Name",
				Age:        30,
				FloodVLANs: []int{100, 4096},
			},
			expectedError: "field 'testValidationModel.FloodVLANs[1]' (value: '4096') failed on rule 'max' (param: 4095)",
		},
		{
			name: "invalid FloodVLAN value (too low)",
			model: testValidationModel{
				Name:       "Test Name",
				Age:        30,
				FloodVLANs: []int{-1, 100},
			},
			expectedError: "field 'testValidationModel.FloodVLANs[0]' (value: '-1') failed on rule 'min' (param: 0)",
		},
		{
			name: "invalid Protocol value",
			model: testValidationModel{
				Name:      "Test Name",
				Age:       30,
				Protocols: []string{"OpenFlow13", "InvalidProtocol"},
			},
			expectedError: "field 'testValidationModel.Protocols[1]' (value: 'InvalidProtocol') failed on rule 'oneof' (param: 'OpenFlow10' 'OpenFlow11' 'OpenFlow12' 'OpenFlow13' 'OpenFlow14' 'OpenFlow15')",
		},
		{
			name: "invalid Mappings key (too high)",
			model: testValidationModel{
				Name:     "Test Name",
				Age:      30,
				Mappings: map[int]int{16777216: 1000},
			},
			expectedError: "field 'testValidationModel.Mappings[16777216]' (value: '16777216') failed on rule 'max' (param: 16777215)",
		},
		{
			name: "invalid Mappings key (too low)",
			model: testValidationModel{
				Name:     "Test Name",
				Age:      30,
				Mappings: map[int]int{-1: 1000},
			},
			expectedError: "field 'testValidationModel.Mappings[-1]' (value: '-1') failed on rule 'min' (param: 0)",
		},
		{
			name: "invalid Mappings value (too high)",
			model: testValidationModel{
				Name:     "Test Name",
				Age:      30,
				Mappings: map[int]int{100: 4096},
			},
			expectedError: "field 'testValidationModel.Mappings[100]' (value: '4096') failed on rule 'max' (param: 4095)",
		},
		{
			name: "invalid Mappings value (too low)",
			model: testValidationModel{
				Name:     "Test Name",
				Age:      30,
				Mappings: map[int]int{100: -1},
			},
			expectedError: "field 'testValidationModel.Mappings[100]' (value: '-1') failed on rule 'min' (param: 0)",
		},
		{
			name: "invalid Config key (too short)",
			model: testValidationModel{
				Name:   "Test Name",
				Age:    30,
				Config: map[string]string{"": "value1"},
			},
			expectedError: "field 'testValidationModel.Config[]' (value: '') failed on rule 'min' (param: 1)",
		},
		{
			name: "invalid Config key (too long)",
			model: testValidationModel{
				Name:   "Test Name",
				Age:    30,
				Config: map[string]string{"verylongkeythatexceedsthirtytwocharacters": "value1"},
			},
			expectedError: "field 'testValidationModel.Config[verylongkeythatexceedsthirtytwocharacters]' (value: 'verylongkeythatexceedsthirtytwocharacters') failed on rule 'max' (param: 32)",
		},
		{
			name: "invalid Config value (too short)",
			model: testValidationModel{
				Name:   "Test Name",
				Age:    30,
				Config: map[string]string{"key1": ""},
			},
			expectedError: "field 'testValidationModel.Config[key1]' (value: '') failed on rule 'min' (param: 1)",
		},
		{
			name: "invalid Config value (too long)",
			model: testValidationModel{
				Name:   "Test Name",
				Age:    30,
				Config: map[string]string{"key1": "verylongvaluethatexceedstwohundredandfiftysixcharacterslimitverylongvaluethatexceedstwohundredandfiftysixcharacterslimitverylongvaluethatexceedstwohundredandfiftysixcharacterslimitverylongvaluethatexceedstwohundredandfiftysixcharacterslimitverylongvaluethatexceedstwohundredandfiftysixcharacterslimit"},
			},
			expectedError: "field 'testValidationModel.Config[key1]' (value: 'verylongvaluethatexceedstwohundredandfiftysixcharacterslimitverylongvaluethatexceedstwohundredandfiftysixcharacterslimitverylongvaluethatexceedstwohundredandfiftysixcharacterslimitverylongvaluethatexceedstwohundredandfiftysixcharacterslimitverylongvaluethatexceedstwohundredandfiftysixcharacterslimit') failed on rule 'max' (param: 256)",
		},
		{
			name: "invalid KeyConfig key (too large)",
			model: testValidationModel{
				Name:      "Test Name",
				Age:       30,
				KeyConfig: map[int]int{11: 999},
			},
			expectedError: "field 'testValidationModel.KeyConfig[11]' (value: '11') failed on rule 'max' (param: 10)",
		},
		{
			name: "invalid ValueConfig value (too large)",
			model: testValidationModel{
				Name:        "Test Name",
				Age:         30,
				ValueConfig: map[string]int{"key1": 11, "key2": 10},
			},
			expectedError: "field 'testValidationModel.ValueConfig[key1]' (value: '11') failed on rule 'max' (param: 10)",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateModel(&tc.model)

			require.Error(t, err)
			require.ErrorContains(t, err, "model validation failed:")
			if tc.expectedError != "" {
				require.ErrorContains(t, err, tc.expectedError)
			}

			var valErrs validator.ValidationErrors
			require.ErrorAs(t, err, &valErrs)
			assert.NotEmpty(t, valErrs)
		})
	}
}

func TestValidateModel_NestedStruct(t *testing.T) {
	validNestedModel := testValidationNestedModel{
		ID: "550e8400-e29b-41d4-a716-446655440000",
		Detail: testValidationModel{
			Name:       "Test Name",
			Age:        30,
			FloodVLANs: []int{100, 200},
			Protocols:  []string{"OpenFlow13"},
			Mappings:   map[int]int{100: 500},
			Config:     map[string]string{"key1": "value1"},
		},
	}

	err := validateModel(&validNestedModel)
	require.NoError(t, err)

	invalidNestedModel := testValidationNestedModel{
		ID: "invalid-uuid",
		Detail: testValidationModel{
			Name: "Test Name",
			Age:  30,
		},
	}

	err = validateModel(&invalidNestedModel)
	require.Error(t, err)

	require.ErrorContains(t, err, "model validation failed:")
	require.ErrorContains(t, err, "field 'testValidationNestedModel.ID' (value: 'invalid-uuid') failed on rule 'uuid'")
	var valErrs validator.ValidationErrors
	require.ErrorAs(t, err, &valErrs)
	assert.NotEmpty(t, valErrs)
}

func TestValidateModel_SliceValidation(t *testing.T) {
	model := testValidationModel{
		Name: "Test Name",
		Age:  30,
		Tags: []string{"this-tag-is-more-than-ten-characters-long", "valid-tag"},
	}

	err := validateModel(&model)
	require.Error(t, err)
	require.ErrorContains(t, err, "model validation failed:")
	require.ErrorContains(t, err, "field 'testValidationModel.Tags[0]' (value: 'this-tag-is-more-than-ten-characters-long') failed on rule 'max' (param: 10)")
	var valErrs validator.ValidationErrors
	require.ErrorAs(t, err, &valErrs)
	assert.NotEmpty(t, valErrs)
}

func TestFormatValidationErrors(t *testing.T) {
	// Test the formatValidationErrors function directly
	model := testValidationModel{
		Name: "",
		Age:  -5,
	}

	err := validate.Struct(&model)
	require.Error(t, err)

	var valErrs validator.ValidationErrors
	require.ErrorAs(t, err, &valErrs)

	// Test without context
	formatted := formatValidationErrors("client.testValidationModel", "", valErrs)
	assert.Contains(t, formatted, "validation error for model client.testValidationModel")
	assert.Contains(t, formatted, "field 'testValidationModel.Name' (value: '') failed on rule 'min' (param: 1)")
	assert.Contains(t, formatted, "field 'testValidationModel.Age' (value: '-5') failed on rule 'min' (param: 0)")

	// Test with context
	formatted = formatValidationErrors("client.testValidationModel", "mutation on column name", valErrs)
	assert.Contains(t, formatted, "validation error for model client.testValidationModel: mutation on column name")
	assert.Contains(t, formatted, "field 'testValidationModel.Name' (value: '') failed on rule 'min' (param: 1)")
}
