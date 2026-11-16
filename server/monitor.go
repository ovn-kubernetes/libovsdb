package server

import (
	"log"
	"sync"

	"github.com/cenkalti/rpc2"
	"github.com/google/uuid"
	"github.com/ovn-kubernetes/libovsdb/database"
	"github.com/ovn-kubernetes/libovsdb/internal/json"
	"github.com/ovn-kubernetes/libovsdb/model"
	"github.com/ovn-kubernetes/libovsdb/ovsdb"
)

// connectionMonitors maps a connection to a map or monitors
type connectionMonitors struct {
	monitors map[string]*monitor
	mu       sync.RWMutex
}

func newConnectionMonitors() *connectionMonitors {
	return &connectionMonitors{
		monitors: make(map[string]*monitor),
		mu:       sync.RWMutex{},
	}
}

// monitor represents a connection to a client where db changes
// will be reflected
type monitor struct {
	id      string
	kind    monitorKind
	request map[string]*ovsdb.MonitorRequest
	client  *rpc2.Client
	dbModel *model.DatabaseModel
}

type monitorKind int

const (
	monitorKindOriginal monitorKind = iota
	monitorKindConditional
	monitorKindConditionalSince
)

func newMonitor(id string, request map[string]*ovsdb.MonitorRequest, client *rpc2.Client) *monitor {
	m := &monitor{
		id:      id,
		kind:    monitorKindOriginal,
		request: request,
		client:  client,
	}
	return m
}

func newConditionalMonitor(id string, request map[string]*ovsdb.MonitorRequest, client *rpc2.Client) *monitor {
	m := &monitor{
		id:      id,
		kind:    monitorKindConditional,
		request: request,
		client:  client,
	}
	return m
}

func newConditionalSinceMonitor(id string, request map[string]*ovsdb.MonitorRequest, client *rpc2.Client) *monitor {
	m := &monitor{
		id:      id,
		kind:    monitorKindConditionalSince,
		request: request,
		client:  client,
	}
	return m
}

// Send will send an update if it matches the tables and monitor select arguments
// we take the update by value (not reference) so we can mutate it in place before
// queuing it for dispatch
func (m *monitor) Send(update database.Update) {
	// remove updates for tables that we aren't watching
	tu := m.filter(update)
	if len(tu) == 0 {
		return
	}
	args := []any{json.RawMessage([]byte(m.id)), tu}
	var reply any
	err := m.client.Call("update", args, &reply)
	if err != nil {
		log.Printf("client error handling update rpc: %v", err)
	}
}

// Send2 will send an update if it matches the tables and monitor select arguments
// we take the update by value (not reference) so we can mutate it in place before
// queuing it for dispatch
func (m *monitor) Send2(update database.Update) {
	// remove updates for tables that we aren't watching
	tu := m.filter2(update)
	if len(tu) == 0 {
		return
	}
	args := []any{json.RawMessage([]byte(m.id)), tu}
	var reply any
	err := m.client.Call("update2", args, &reply)
	if err != nil {
		log.Printf("client error handling update2 rpc: %v", err)
	}
}

// Send3 will send an update if it matches the tables and monitor select arguments
// we take the update by value (not reference) so we can mutate it in place before
// queuing it for dispatch
func (m *monitor) Send3(id uuid.UUID, update database.Update) {
	// remove updates for tables that we aren't watching
	tu := m.filter2(update)
	if len(tu) == 0 {
		return
	}
	args := []any{json.RawMessage([]byte(m.id)), id.String(), tu}
	var reply any
	err := m.client.Call("update3", args, &reply)
	if err != nil {
		log.Printf("client error handling update3 rpc: %v", err)
	}
}

func filterColumns(row *ovsdb.Row, columns map[string]bool) *ovsdb.Row {
	if row == nil {
		return nil
	}
	newRow := make(ovsdb.Row, len(*row))
	for k, v := range *row {
		if _, ok := columns[k]; ok {
			newRow[k] = v
		}
	}
	return &newRow
}

func (m *monitor) rowMatchesAnyCondition(table string, row model.Model, conditions []ovsdb.Condition) bool {
	if row == nil || m.dbModel == nil {
		return false
	}
	if len(conditions) == 0 {
		return true
	}
	info, err := m.dbModel.NewModelInfo(row)
	if err != nil {
		return false
	}
	tableSchema := m.dbModel.Schema.Table(table)
	if tableSchema == nil {
		return false
	}
	for _, condition := range conditions {
		columnSchema := tableSchema.Column(condition.Column)
		if columnSchema == nil {
			continue
		}
		conditionValue, err := ovsdb.OvsToNative(columnSchema, condition.Value)
		if err != nil {
			continue
		}
		value, err := info.FieldByColumn(condition.Column)
		if err != nil {
			continue
		}
		matches, err := condition.Function.Evaluate(value, conditionValue)
		if err == nil && matches {
			return true
		}
	}
	return false
}

func (m *monitor) monitoredColumns(table string, requested []string) map[string]bool {
	columns := make(map[string]bool)
	if len(requested) > 0 {
		for _, column := range requested {
			columns[column] = true
		}
		return columns
	}
	if m.dbModel == nil {
		return columns
	}
	tableSchema := m.dbModel.Schema.Table(table)
	if tableSchema == nil {
		return columns
	}
	for column := range tableSchema.Columns {
		columns[column] = true
	}
	return columns
}

func rowWithDefaults(dbModel *model.DatabaseModel, table string, row model.Model, columns []string) *ovsdb.Row {
	if row == nil || dbModel == nil {
		return nil
	}
	info, err := dbModel.NewModelInfo(row)
	if err != nil {
		return nil
	}
	tableSchema := dbModel.Schema.Table(table)
	if tableSchema == nil {
		return nil
	}
	selected := append([]string(nil), columns...)
	if len(selected) == 0 {
		for column := range tableSchema.Columns {
			selected = append(selected, column)
		}
	}
	result := make(ovsdb.Row, len(selected))
	for _, columnName := range selected {
		columnSchema := tableSchema.Column(columnName)
		if columnSchema == nil {
			continue
		}
		value, err := info.FieldByColumn(columnName)
		if err != nil {
			continue
		}
		result[columnName], err = ovsdb.NativeToOvs(columnSchema, value)
		if err != nil {
			return nil
		}
	}
	return &result
}

func (m *monitor) rowForMonitor(table string, row model.Model, columns []string) *ovsdb.Row {
	if row == nil || m.dbModel == nil {
		return nil
	}
	info, err := m.dbModel.NewModelInfo(row)
	if err != nil {
		return nil
	}
	// update2/update3 initial and insert rows omit schema-default values.
	result, err := m.dbModel.Mapper.NewRow(info)
	if err != nil {
		return nil
	}
	return filterColumns(&result, m.monitoredColumns(table, columns))
}

func (m *monitor) filter(update database.Update) ovsdb.TableUpdates {
	// remove updates for tables that we aren't watching
	tables := update.GetUpdatedTables()
	tus := make(ovsdb.TableUpdates, len(tables))
	for _, table := range tables {
		if _, ok := m.request[table]; len(m.request) > 0 && !ok {
			// only remove updates for tables that were not requested if other
			// tables were requested, otherwise all tables are watched.
			continue
		}
		tu := ovsdb.TableUpdate{}
		cols := m.monitoredColumns(table, m.request[table].Columns)
		type modelUpdate struct {
			old model.Model
			new model.Model
		}
		models := make(map[string]modelUpdate)
		_ = update.ForEachModelUpdate(table, func(uuid string, old, newModel model.Model) error {
			models[uuid] = modelUpdate{old: old, new: newModel}
			return nil
		})
		_ = update.ForEachRowUpdate(table, func(uuid string, ru2 ovsdb.RowUpdate2) error {
			ru := &ovsdb.RowUpdate{}
			ru.FromRowUpdate2(ru2)
			state := models[uuid]
			switch {
			case ru.Insert() && m.request[table].Select.Insert():
				ru.New = rowWithDefaults(m.dbModel, table, state.new, m.request[table].Columns)
			case ru.Modify() && m.request[table].Select.Modify():
				ru.New = filterColumns(ru.New, cols)
				ru.Old = filterColumns(ru.Old, cols)
			case ru.Delete() && m.request[table].Select.Delete():
				ru.Old = rowWithDefaults(m.dbModel, table, state.old, m.request[table].Columns)
			default:
				return nil
			}
			tu[uuid] = ru
			return nil
		})
		tus[table] = tu
	}
	return tus
}

func (m *monitor) filter2(update database.Update) ovsdb.TableUpdates2 {
	// remove updates for tables that we aren't watching
	tables := update.GetUpdatedTables()
	tus2 := make(ovsdb.TableUpdates2, len(tables))
	for _, table := range tables {
		if _, ok := m.request[table]; len(m.request) > 0 && !ok {
			// only remove updates for tables that were not requested if other
			// tables were requested, otherwise all tables are watched.
			continue
		}
		tu2 := ovsdb.TableUpdate2{}
		type modelUpdate struct {
			old model.Model
			new model.Model
		}
		models := make(map[string]modelUpdate)
		_ = update.ForEachModelUpdate(table, func(uuid string, old, newModel model.Model) error {
			models[uuid] = modelUpdate{old: old, new: newModel}
			return nil
		})
		cols := m.monitoredColumns(table, m.request[table].Columns)
		_ = update.ForEachRowUpdate(table, func(uuid string, ru2 ovsdb.RowUpdate2) error {
			request := m.request[table]
			if len(request.Where) == 0 {
				switch {
				case ru2.Insert != nil && request.Select.Insert():
				case ru2.Modify != nil && request.Select.Modify():
				case ru2.Delete != nil && request.Select.Delete():
				default:
					return nil
				}
				ru2.Insert = filterColumns(ru2.Insert, cols)
				ru2.Modify = filterColumns(ru2.Modify, cols)
				ru2.Delete = filterColumns(ru2.Delete, cols)
				tu2[uuid] = &ru2
				return nil
			}

			state := models[uuid]
			oldMatches := m.rowMatchesAnyCondition(table, state.old, request.Where)
			newMatches := m.rowMatchesAnyCondition(table, state.new, request.Where)
			filtered := &ovsdb.RowUpdate2{}
			switch {
			case !oldMatches && newMatches && request.Select.Insert():
				filtered.Insert = m.rowForMonitor(table, state.new, request.Columns)
			case oldMatches && newMatches && request.Select.Modify():
				filtered.Modify = filterColumns(ru2.Modify, cols)
			case oldMatches && !newMatches && request.Select.Delete():
				// ovsdb-server(7) specifies a null/empty row for update2 deletes.
				empty := ovsdb.Row{}
				filtered.Delete = &empty
			default:
				return nil
			}
			tu2[uuid] = filtered
			return nil
		})
		tus2[table] = tu2
	}
	return tus2
}
