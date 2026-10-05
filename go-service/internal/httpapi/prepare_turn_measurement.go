package httpapi

import (
	"reflect"
	"time"
)

// Request-owned observations only. Never use these values to select memory.
// Spans are used on the preparation goroutine, not embedding worker goroutines.
type prepareTurnMeasurement struct {
	stages  map[string]*prepareTurnMeasuredStage
	counts  map[string]int64
	current *prepareTurnMeasuredSpan
}

type prepareTurnMeasuredStage struct {
	inclusive, exclusive time.Duration
	calls                int64
}

type prepareTurnMeasuredSpan struct {
	owner    *prepareTurnMeasurement
	parent   *prepareTurnMeasuredSpan
	stage    *prepareTurnMeasuredStage
	started  time.Time
	children time.Duration
}

func newPrepareTurnMeasurement() *prepareTurnMeasurement {
	return &prepareTurnMeasurement{stages: map[string]*prepareTurnMeasuredStage{}, counts: map[string]int64{}}
}

func (m *prepareTurnMeasurement) start(name string) *prepareTurnMeasuredSpan {
	if m == nil {
		return nil
	}
	stage := m.stages[name]
	if stage == nil {
		stage = &prepareTurnMeasuredStage{}
		m.stages[name] = stage
	}
	span := &prepareTurnMeasuredSpan{owner: m, parent: m.current, stage: stage, started: time.Now()}
	m.current = span
	return span
}

func (s *prepareTurnMeasuredSpan) end() {
	if s == nil {
		return
	}
	elapsed := time.Since(s.started)
	s.stage.inclusive += elapsed
	s.stage.exclusive += elapsed - s.children
	s.stage.calls++
	if s.parent != nil {
		s.parent.children += elapsed
	}
	s.owner.current = s.parent
}

func (m *prepareTurnMeasurement) add(name string, n int) {
	if m != nil {
		m.counts[name] += int64(n)
	}
}

func (m *prepareTurnMeasurement) snapshot() map[string]any {
	if m == nil {
		return nil
	}
	stages := map[string]any{}
	for name, stage := range m.stages {
		stages[name] = map[string]any{"inclusive_ms": durationMilliseconds(stage.inclusive), "exclusive_ms": durationMilliseconds(stage.exclusive), "calls": stage.calls}
	}
	counts := map[string]int64{}
	for name, count := range m.counts {
		counts[name] = count
	}
	return map[string]any{"contract_version": "prepare_turn.measurement.v1", "stages": stages, "counts": counts,
		"time_basis": "wall_clock; nested inclusive stages must not be summed", "row_bytes_basis": "decoded UTF-8 string fields, not SQL wire bytes"}
}

func (p *prepareTurnRequestPreparation) measurement() *prepareTurnMeasurement {
	if p == nil {
		return nil
	}
	return p.metrics
}

// Read timing excludes the observational row-size walk. Existing errors/rows
// pass through unchanged, including partial results and unavailable readers.
func prepareTurnMeasureRead[T any](m *prepareTurnMeasurement, name string, read func() (T, error)) (T, error) {
	span := m.start(name)
	rows, err := read()
	span.end()
	if m != nil {
		observing := m.start("measurement.row_size")
		v := reflect.ValueOf(rows)
		n := 1
		if v.Kind() == reflect.Slice || v.Kind() == reflect.Map {
			n = v.Len()
		} else if v.Kind() == reflect.Pointer && v.IsNil() {
			n = 0
		}
		m.add(name+".rows", n)
		m.add(name+".text_bytes", prepareTurnMeasuredTextBytes(v))
		if err != nil {
			m.add(name+".errors", 1)
		}
		observing.end()
	}
	return rows, err
}

func prepareTurnMeasuredTextBytes(v reflect.Value) int {
	if !v.IsValid() {
		return 0
	}
	switch v.Kind() {
	case reflect.String:
		return v.Len()
	case reflect.Interface, reflect.Pointer:
		if !v.IsNil() {
			return prepareTurnMeasuredTextBytes(v.Elem())
		}
	case reflect.Slice, reflect.Array:
		n := 0
		for i := 0; i < v.Len(); i++ {
			n += prepareTurnMeasuredTextBytes(v.Index(i))
		}
		return n
	case reflect.Struct:
		n := 0
		for i := 0; i < v.NumField(); i++ {
			if v.Type().Field(i).IsExported() {
				n += prepareTurnMeasuredTextBytes(v.Field(i))
			}
		}
		return n
	case reflect.Map:
		n := 0
		it := v.MapRange()
		for it.Next() {
			n += prepareTurnMeasuredTextBytes(it.Key()) + prepareTurnMeasuredTextBytes(it.Value())
		}
		return n
	}
	return 0
}
