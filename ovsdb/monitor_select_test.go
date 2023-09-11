package ovsdb

import (
	"testing"

	"github.com/ovn-org/libovsdb/internal/json"

	"github.com/stretchr/testify/assert"
)

func TestNewMonitorSelect(t *testing.T) {
	ms := NewMonitorSelect(true, false, true, false)
	assert.True(t, ms.Initial(), "initial")
	assert.False(t, ms.Insert(), "insert")
	assert.True(t, ms.Delete(), "delete")
	assert.False(t, ms.Modify(), "modify")
}

func TestNewDefaultMonitorSelect(t *testing.T) {
	ms := NewDefaultMonitorSelect()
	assert.True(t, ms.Initial(), "initial")
	assert.True(t, ms.Insert(), "insert")
	assert.True(t, ms.Delete(), "delete")
	assert.True(t, ms.Modify(), "modify")
}

func TestMonitorSelectInitial(t *testing.T) {
	tt := true
	f := false
	ms1 := MonitorSelect{initial: nil}
	ms2 := MonitorSelect{initial: &tt}
	ms3 := MonitorSelect{initial: &f}
	assert.True(t, ms1.Initial(), "nil")
	assert.True(t, ms2.Initial(), "true")
	assert.False(t, ms3.Initial(), "false")
}

func TestMonitorSelectInsert(t *testing.T) {
	tt := true
	f := false
	ms1 := MonitorSelect{insert: nil}
	ms2 := MonitorSelect{insert: &tt}
	ms3 := MonitorSelect{insert: &f}
	assert.True(t, ms1.Insert(), "nil")
	assert.True(t, ms2.Insert(), "true")
	assert.False(t, ms3.Insert(), "false")
}

func TestMonitorSelectDelete(t *testing.T) {
	tt := true
	f := false
	ms1 := MonitorSelect{delete: nil}
	ms2 := MonitorSelect{delete: &tt}
	ms3 := MonitorSelect{delete: &f}
	assert.True(t, ms1.Delete(), "nil")
	assert.True(t, ms2.Delete(), "true")
	assert.False(t, ms3.Delete(), "false")
}

func TestMonitorSelectModify(t *testing.T) {
	tt := true
	f := false
	ms1 := MonitorSelect{modify: nil}
	ms2 := MonitorSelect{modify: &tt}
	ms3 := MonitorSelect{modify: &f}
	assert.True(t, ms1.Modify(), "nil")
	assert.True(t, ms2.Modify(), "true")
	assert.False(t, ms3.Modify(), "false")
}

func TestMonitorSelectMarshalUnmarshalJSON(t *testing.T) {
	tests := []struct {
		name string
		ms   *MonitorSelect
		want string
	}{
		{
			"nil",
			&MonitorSelect{},
			`{}`,
		},
		{
			"default",
			NewDefaultMonitorSelect(),
			`{"delete":true, "initial":true, "insert":true, "modify":true}`,
		},
		{
			"falsey",
			NewMonitorSelect(false, false, false, false),
			`{"delete":false, "initial":false, "insert":false, "modify":false}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := json.Marshal(tt.ms)
			assert.Nil(t, err)
			assert.JSONEq(t, tt.want, string(got))
			var ms2 MonitorSelect
			err = json.Unmarshal(got, &ms2)
			assert.Nil(t, err)
			assert.Equal(t, tt.ms, &ms2)
		})
	}
}
