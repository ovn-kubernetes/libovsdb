package ovsdb

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestErrorFromResult(t *testing.T) {
	type args struct {
		op *Operation
		r  OperationResult
	}
	tests := []struct {
		name     string
		args     args
		expected any
	}{
		{
			referentialIntegrityViolation,
			args{nil, OperationResult{Error: referentialIntegrityViolation}},
			&ReferentialIntegrityViolation{},
		},
		{
			constraintViolation,
			args{nil, OperationResult{Error: constraintViolation}},
			&ConstraintViolation{},
		},
		{
			resourcesExhausted,
			args{nil, OperationResult{Error: resourcesExhausted}},
			&ResourcesExhausted{},
		},
		{
			ioError,
			args{nil, OperationResult{Error: ioError}},
			&IOError{},
		},
		{
			duplicateUUIDName,
			args{nil, OperationResult{Error: duplicateUUIDName}},
			&DuplicateUUIDName{},
		},
		{
			domainError,
			args{nil, OperationResult{Error: domainError}},
			&DomainError{},
		},
		{
			rangeError,
			args{nil, OperationResult{Error: rangeError}},
			&RangeError{},
		},
		{
			timedOut,
			args{nil, OperationResult{Error: timedOut}},
			&TimedOut{},
		},
		{
			notSupported,
			args{nil, OperationResult{Error: notSupported}},
			&NotSupported{},
		},
		{
			aborted,
			args{nil, OperationResult{Error: aborted}},
			&Aborted{},
		},
		{
			notOwner,
			args{nil, OperationResult{Error: notOwner}},
			&NotOwner{},
		},
		{
			"generic error",
			args{nil, OperationResult{Error: "foo"}},
			&Error{},
		},
		{
			"nil",
			args{nil, OperationResult{Error: ""}},
			nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := errorFromResult(tt.args.op, tt.args.r)
			assert.IsType(t, tt.expected, err)
		})
	}
}

func TestCheckOperationResults(t *testing.T) {
	type args struct {
		result []OperationResult
		ops    []Operation
	}
	tests := []struct {
		name    string
		args    args
		want    []OperationError
		wantErr bool
	}{
		{
			"success",
			args{[]OperationResult{{}}, []Operation{{Op: "insert"}}},
			nil,
			false,
		},
		{
			"commit error",
			args{[]OperationResult{{}, {Error: constraintViolation}}, []Operation{{Op: "insert"}}},
			nil,
			true,
		},
		{
			"transaction error",
			args{[]OperationResult{{Error: constraintViolation, Details: "foo"}, {Error: constraintViolation, Details: "bar"}}, []Operation{{Op: "insert"}, {Op: "mutate"}}},
			[]OperationError{&ConstraintViolation{details: "foo", operation: &Operation{Op: "insert"}}, &ConstraintViolation{details: "bar", operation: &Operation{Op: "mutate"}}},
			true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := CheckOperationResults(tt.args.result, tt.args.ops)
			if (err != nil) != tt.wantErr {
				t.Errorf("CheckOperationResults() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("CheckOperationResults() = %v, want %v", got, tt.want)
			}
		})
	}
}
