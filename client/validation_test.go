package client

import (
	"fmt"
	"testing"

	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testValidationModel struct {
	Name       string   `validate:"min=1,max=64"`
	Age        int      `validate:"min=0,max=150"`
	Email      string   `validate:"omitempty,email"`
	Tags       []string `validate:"dive,max=10"`
	Score      *int     `validate:"omitempty,min=0,max=100"`
	FloodVLANs []int    `validate:"max=4096,dive,min=0,max=4095"`
	Protocols  []string `validate:"dive,oneof='OpenFlow10' 'OpenFlow11' 'OpenFlow12' 'OpenFlow13' 'OpenFlow14' 'OpenFlow15'"`
	IsActive   bool
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
	assert.Contains(t, err.Error(), "model cannot be nil")
}

func TestValidateModel_NilPointer(t *testing.T) {
	var model *testValidationModel
	err := validateModel(model)

	require.Error(t, err)
	var validationErr *ValidationError
	if assert.ErrorAs(t, err, &validationErr) {
		assert.Contains(t, validationErr.Error(), "validation attempt on nil model")
		assert.Equal(t, "client.testValidationModel", validationErr.ModelName)
		assert.Error(t, validationErr.GeneralError)
		assert.Nil(t, validationErr.FieldValidationErrors)

		assert.Equal(t, validationErr.GeneralError, validationErr.Unwrap())
	}
}

func TestValidateModel_NonStruct(t *testing.T) {
	err := validateModel("not a struct")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "model must be a struct or a pointer to a struct")
}

func TestValidateModel_Valid(t *testing.T) {
	model := testValidationModel{
		Name:       "Test Name",
		Age:        30,
		Email:      "test@example.com",
		Tags:       []string{"tag1", "tag2"},
		Score:      intPtr(85),
		FloodVLANs: []int{100, 200, 4095},
		Protocols:  []string{"OpenFlow13", "OpenFlow15"},
		IsActive:   true,
	}

	err := validateModel(model)
	require.NoError(t, err)

	err = validateModel(&model)
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
			expectedError: "field 'testValidationModel.Name'",
		},
		{
			name: "invalid age",
			model: testValidationModel{
				Name: "Test Name",
				Age:  -5,
			},
			expectedError: "field 'testValidationModel.Age'",
		},
		{
			name: "invalid email",
			model: testValidationModel{
				Name:  "Test Name",
				Age:   30,
				Email: "not-an-email",
			},
			expectedError: "field 'testValidationModel.Email'",
		},
		{
			name: "invalid score",
			model: testValidationModel{
				Name:  "Test Name",
				Age:   30,
				Score: intPtr(150),
			},
			expectedError: "field 'testValidationModel.Score'",
		},
		{
			name: "invalid FloodVLAN value (too high)",
			model: testValidationModel{
				Name:       "Test Name",
				Age:        30,
				FloodVLANs: []int{100, 4096},
			},
			expectedError: "field 'testValidationModel.FloodVLANs[1]'",
		},
		{
			name: "invalid FloodVLAN value (too low)",
			model: testValidationModel{
				Name:       "Test Name",
				Age:        30,
				FloodVLANs: []int{-1, 100},
			},
			expectedError: "field 'testValidationModel.FloodVLANs[0]'",
		},
		{
			name: "invalid Protocol value",
			model: testValidationModel{
				Name:      "Test Name",
				Age:       30,
				Protocols: []string{"OpenFlow13", "InvalidProtocol"},
			},
			expectedError: "field 'testValidationModel.Protocols[1]'",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateModel(tc.model)

			require.Error(t, err)
			var validationErr *ValidationError
			if assert.ErrorAs(t, err, &validationErr) {
				assert.Contains(t, validationErr.Error(), tc.expectedError)
				assert.Equal(t, "client.testValidationModel", validationErr.ModelName)
				assert.NotNil(t, validationErr.FieldValidationErrors)
				require.NoError(t, validationErr.GeneralError)

				assert.Equal(t, validationErr.FieldValidationErrors, validationErr.Unwrap())

				assert.IsType(t, validator.ValidationErrors{}, validationErr.FieldValidationErrors)

				var valErrs validator.ValidationErrors
				assert.ErrorAs(t, err, &valErrs)
				assert.Equal(t, validationErr.FieldValidationErrors, valErrs)
			}
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
		},
	}

	err := validateModel(validNestedModel)
	require.NoError(t, err)

	invalidNestedModel := testValidationNestedModel{
		ID: "invalid-uuid",
		Detail: testValidationModel{
			Name: "Test Name",
			Age:  30,
		},
	}

	err = validateModel(invalidNestedModel)
	require.Error(t, err)

	var validationErr *ValidationError
	if assert.ErrorAs(t, err, &validationErr) {
		assert.Contains(t, validationErr.Error(), "failed on rule 'uuid'")
		assert.Equal(t, "client.testValidationNestedModel", validationErr.ModelName)
		assert.NotNil(t, validationErr.FieldValidationErrors)
		assert.NoError(t, validationErr.GeneralError)
	}
}

func TestValidateModel_SliceValidation(t *testing.T) {
	model := testValidationModel{
		Name: "Test Name",
		Age:  30,
		Tags: []string{"this-tag-is-more-than-ten-characters-long", "valid-tag"},
	}

	err := validateModel(model)
	require.Error(t, err)

	var validationErr *ValidationError
	if assert.ErrorAs(t, err, &validationErr) {
		assert.Contains(t, validationErr.Error(), `field 'testValidationModel.Tags[0]'`)
		assert.NotNil(t, validationErr.FieldValidationErrors)
		assert.NoError(t, validationErr.GeneralError)
	}
}

func TestValidationError_ErrorFormat(t *testing.T) {
	err1 := &ValidationError{
		ModelName:    "TestModel",
		GeneralError: fmt.Errorf("general test error"),
	}
	expectedFormat1 := "validation error for model TestModel: general test error"
	assert.Equal(t, expectedFormat1, err1.Error())

	fieldErrors := validator.ValidationErrors{}
	err2 := &ValidationError{
		ModelName:             "TestModel",
		FieldValidationErrors: fieldErrors,
	}
	assert.Contains(t, err2.Error(), "validation error for model TestModel")
	assert.Contains(t, err2.Error(), "details: [")

	err3 := &ValidationError{
		ModelName:             "TestModel",
		GeneralError:          fmt.Errorf("mutation context"),
		FieldValidationErrors: fieldErrors,
	}
	assert.Contains(t, err3.Error(), "validation error for model TestModel: mutation context")
	assert.Contains(t, err3.Error(), "details: [")
}

func TestValidationError_ErrorInterface(t *testing.T) {
	err := &ValidationError{
		ModelName:    "TestModel",
		GeneralError: fmt.Errorf("test error"),
	}

	var _ error = err

	errStr := err.Error()
	assert.Contains(t, errStr, "validation error for model TestModel")
	assert.Contains(t, errStr, "test error")
}

func TestValidationError_Unwrap(t *testing.T) {
	fieldErrs := validator.ValidationErrors{}
	err1 := &ValidationError{
		ModelName:             "TestModel",
		FieldValidationErrors: fieldErrs,
	}

	assert.Equal(t, fieldErrs, err1.Unwrap())

	originalErr := fmt.Errorf("original error")
	err2 := &ValidationError{
		ModelName:    "TestModel",
		GeneralError: originalErr,
	}

	assert.Equal(t, originalErr, err2.Unwrap())
}
