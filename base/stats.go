package base

import (
	"errors"
	"strings"
	"time"

	stathat "github.com/stathat/go"
)

type StatsBackend interface {
	Count(name string) error
	CountMult(name string, count int) error
	Value(name string, value float64) error
	Shutdown()
}

type StatsBackendType int

const (
	StathatStatsBackendType StatsBackendType = iota
	DummyStatsBackendType
)

type DummyStatsBackend struct {
	*DebugOutput
	prefix string
}

func NewDummyStatsBackend(debugConfig *ChatDebugOutputConfig, prefix string) *DummyStatsBackend {
	return &DummyStatsBackend{
		DebugOutput: NewDebugOutput("DummyStatsBackend - "+prefix, debugConfig),
		prefix:      prefix,
	}
}

func (d *DummyStatsBackend) stripPrefix(name string) string {
	return strings.ReplaceAll(name, d.prefix, "")
}

func (d *DummyStatsBackend) Count(name string) error {
	d.Debug("Count: %s", d.stripPrefix(name))
	return nil
}

func (d *DummyStatsBackend) CountMult(name string, count int) error {
	d.Debug("CountMulti: %s - %d", d.stripPrefix(name), count)
	return nil
}

func (d *DummyStatsBackend) Value(name string, value float64) error {
	d.Debug("Value: %s - %.2f", d.stripPrefix(name), value)
	return nil
}

func (d *DummyStatsBackend) GetPrefix() string { return d.Name() }
func (d *DummyStatsBackend) Shutdown()         {}

var _ StatsBackend = (*DummyStatsBackend)(nil)

type StathatConfig struct {
	ezkey           string
	shutdownTimeout time.Duration
}

func NewStathatConfig(ezkey string, shutdownTimeout time.Duration) StathatConfig {
	return StathatConfig{ezkey: ezkey, shutdownTimeout: shutdownTimeout}
}

type StathatBackend struct {
	config   StathatConfig
	reporter stathat.Reporter
}

var _ StatsBackend = (*StathatBackend)(nil)

func (s *StathatBackend) Count(name string) error {
	return s.reporter.PostEZCountOne(name, s.config.ezkey)
}

func (s *StathatBackend) CountMult(name string, count int) error {
	return s.reporter.PostEZCount(name, s.config.ezkey, count)
}

func (s *StathatBackend) Value(name string, value float64) error {
	return s.reporter.PostEZValue(name, s.config.ezkey, value)
}

func (s *StathatBackend) Shutdown() {
	s.reporter.WaitUntilFinished(s.config.shutdownTimeout)
}

func NewStatsBackend(btype StatsBackendType, prefix string, config interface{}) (StatsBackend, error) {
	switch btype {
	case StathatStatsBackendType:
		if config, ok := config.(StathatConfig); ok {
			reporter := stathat.NewBatchReporter(stathat.DefaultReporter, 200*time.Millisecond)
			return &StathatBackend{config: config, reporter: reporter}, nil
		} else {
			return nil, errors.New("invalid stathat config")
		}
	case DummyStatsBackendType:
		if config, ok := config.(*ChatDebugOutputConfig); ok {
			return NewDummyStatsBackend(config, prefix), nil
		} else {
			return nil, errors.New("invalid DummyStatsBackend config")
		}
	default:
		return nil, errors.New("unknown stats registry type")
	}
}

type StatsRegistry struct {
	*DebugOutput
	backend StatsBackend
	prefix  string
}

func (r *StatsRegistry) makeFname(name string) string {
	return r.prefix + name
}

func (r *StatsRegistry) SetPrefix(prefix string) *StatsRegistry {
	prefix = r.prefix + prefix + " - "
	return NewStatsRegistryWithPrefix(r.DebugOutput.Config(), r.backend, prefix)
}

func (r *StatsRegistry) ResetPrefix() *StatsRegistry {
	return NewStatsRegistryWithBackend(r.DebugOutput.Config(), r.backend)
}

func (r *StatsRegistry) GetPrefix() string {
	return r.prefix
}

func (r *StatsRegistry) Count(name string) {
	if err := r.backend.Count(r.makeFname(name)); err != nil {
		r.Errorf("failed to post stat: err: %s name: %s", err, name)
	}
}

func (r *StatsRegistry) CountMult(name string, count int) {
	if err := r.backend.CountMult(r.makeFname(name), count); err != nil {
		r.Errorf("failed to post stat: err: %s name: %s", err, name)
	}
}

func (r *StatsRegistry) ValueInt(name string, value int) {
	r.Value(name, float64(value))
}

func (r *StatsRegistry) Value(name string, value float64) {
	if err := r.backend.Value(r.makeFname(name), value); err != nil {
		r.Errorf("failed to post stat: err: %s name: %s", err, name)
	}
}

func (r *StatsRegistry) Shutdown() {
	r.Debug("shutting down stats backend")
	r.backend.Shutdown()
}

func NewStatsRegistryWithBackend(debugConfig *ChatDebugOutputConfig, backend StatsBackend) *StatsRegistry {
	return &StatsRegistry{
		DebugOutput: NewDebugOutput("StatsRegistry", debugConfig),
		backend:     backend,
	}
}

func NewStatsRegistryWithPrefix(debugConfig *ChatDebugOutputConfig, backend StatsBackend, prefix string) *StatsRegistry {
	return &StatsRegistry{
		DebugOutput: NewDebugOutput("StatsRegistry - "+prefix, debugConfig),
		backend:     backend,
		prefix:      prefix,
	}
}

func NewStatsRegistry(debugConfig *ChatDebugOutputConfig, stathatEZKey string, prefix string) (reg *StatsRegistry, err error) {
	var backend StatsBackend
	if stathatEZKey != "" {
		config := NewStathatConfig(stathatEZKey, 10*time.Second)
		backend, err = NewStatsBackend(StathatStatsBackendType, prefix, config)
		if err != nil {
			return nil, err
		}
	} else {
		backend, err = NewStatsBackend(DummyStatsBackendType, prefix, debugConfig)
		if err != nil {
			return nil, err
		}
	}
	return NewStatsRegistryWithPrefix(debugConfig, backend, prefix), nil
}
