package client

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/ovn-kubernetes/libovsdb/mapper"

	"github.com/go-playground/validator/v10"
	"github.com/ovn-kubernetes/libovsdb/model"
)

// global validator instance
var validate *validator.Validate

func init() {
	validate = validator.New(validator.WithRequiredStructEnabled())
	// Register custom validations if needed in the future
	// e.g., validate.RegisterValidation("custom_tag", customValidationFunc)
}

// validateModel performs validation on a given model struct using its tags.
func validateModel(m interface{}) error {
	if m == nil {
		return fmt.Errorf("model cannot be nil")
	}

	value := reflect.ValueOf(m)
	modelType := value.Type()

	actualStructValue := value
	modelNameStr := ""

	if modelType.Kind() == reflect.Ptr {
		if value.IsNil() {
			// Get underlying struct name for the error message if it's a pointer type
			if modelType.Elem().Kind() == reflect.Struct {
				modelNameStr = modelType.Elem().String()
			} else {
				modelNameStr = modelType.String() // Fallback to pointer type string
			}
			// Let validator.Struct handle this. It will return an error (e.g. UnsupportedTypeError).
			err := validate.Struct(m) // m is a nil pointer
			return &ValidationError{ModelName: modelNameStr, GeneralError: fmt.Errorf("validation attempt on nil model: %w", err)}
		}
		actualStructValue = value.Elem()
	}

	if actualStructValue.Kind() != reflect.Struct {
		// Should not happen if m implements model.Model, but good practice to check
		return fmt.Errorf("model must be a struct or a pointer to a struct, got %T", m)
	}
	modelNameStr = actualStructValue.Type().String() // e.g. "MyStruct"

	// Perform the validation
	err := validate.Struct(m) // Pass the original m, validator handles pointer vs struct
	if err != nil {
		if validationErrs, ok := err.(validator.ValidationErrors); ok {
			return &ValidationError{ModelName: modelNameStr, FieldValidationErrors: validationErrs}
		}
		// For other types of errors from validate.Struct (e.g., unsupported type if m was nil pointer and validator handled it differently)
		return &ValidationError{ModelName: modelNameStr, GeneralError: fmt.Errorf("validation system error: %w", err)}
	}
	return nil
}

// ValidationError is an error type representing model validation failures.
type ValidationError struct {
	// ModelName is the type name of the model that failed validation.
	ModelName string
	// FieldValidationErrors contains the specific validation errors from the validator.
	// This can be nil if the error is more general.
	FieldValidationErrors validator.ValidationErrors
	// GeneralError provides additional context or a general validation message,
	// especially if FieldValidationErrors is not available or doesn't cover the whole story.
	GeneralError error
}

// Error implements the error interface, providing a human-readable representation of the validation error.
func (e *ValidationError) Error() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("validation error for model %s", e.ModelName))

	// Append context from GeneralError, if any (e.g., "mutation on column X")
	if e.GeneralError != nil && e.GeneralError.Error() != "" {
		sb.WriteString(fmt.Sprintf(": %s", e.GeneralError.Error()))
	}

	if e.FieldValidationErrors != nil {
		if sb.Len() > 0 && !strings.HasSuffix(sb.String(), ": ") && !strings.HasSuffix(sb.String(), "; ") {
			sb.WriteString("; ")
		}
		sb.WriteString("details: [")
		var fieldErrorMessages []string
		for _, fe := range e.FieldValidationErrors {
			targetField := fe.Namespace() // e.g., "Model.Field" or "Model.Nested.Field"
			// For validate.Var on simple type, Namespace might be empty.
			if targetField == "" {
				targetField = fe.Field() // Fallback to field name if any
			}
			if targetField == "" { // If still empty, use a generic term
				targetField = "<value>"
			}

			errMsg := fmt.Sprintf("field '%s' (value: '%v') failed on rule '%s'", targetField, fe.Value(), fe.ActualTag())
			if fe.Param() != "" {
				errMsg += fmt.Sprintf(" (param: %s)", fe.Param())
			}
			fieldErrorMessages = append(fieldErrorMessages, errMsg)
		}
		sb.WriteString(strings.Join(fieldErrorMessages, ", "))
		sb.WriteString("]")
	}
	return sb.String()
}

// Unwrap provides compatibility for errors.Is and errors.As.
// It allows checking against the wrapped FieldValidationErrors or GeneralError.
func (e *ValidationError) Unwrap() error {
	if e.FieldValidationErrors != nil {
		// validator.ValidationErrors itself implements error
		return e.FieldValidationErrors
	}
	return e.GeneralError
}

// validateMutations performs validation on a given slice of mutations.
func validateMutations(model model.Model, info *mapper.Info, mutations ...model.Mutation) error {
	modelType := reflect.TypeOf(model).Elem()
	modelNameStr := modelType.String()

	for _, mutation := range mutations {
		columnName, err := info.ColumnByPtr(mutation.Field)
		if err != nil {
			return fmt.Errorf("could not get column for mutation field: %w", err)
		}
		// Find the struct field corresponding to the column name
		var structField reflect.StructField
		var found bool
		for i := 0; i < modelType.NumField(); i++ {
			if modelType.Field(i).Tag.Get("ovsdb") == columnName {
				structField = modelType.Field(i)
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("could not find struct field for column %s", columnName)
		}

		// Extract the validate tag
		validateTag := structField.Tag.Get("validate")

		// Validate the mutation value if a tag exists
		if validateTag != "" {
			err = validate.Var(mutation.Value, validateTag)
			if err != nil {
				if validationErrs, ok := err.(validator.ValidationErrors); ok {
					return &ValidationError{
						ModelName:             modelNameStr,
						FieldValidationErrors: validationErrs,
						GeneralError:          fmt.Errorf("mutation on column '%s'", columnName),
					}
				}
				return &ValidationError{
					ModelName:    modelNameStr,
					GeneralError: fmt.Errorf("validation for mutation value on column '%s' failed: %w", columnName, err),
				}
			}
		}
	}
	return nil
}
