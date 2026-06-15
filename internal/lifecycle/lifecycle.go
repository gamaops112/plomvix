// Package lifecycle provides a minimal component lifecycle manager
// for Plomvix. Components are registered, started in order, and stopped
// in reverse order with context-aware lifecycle methods.
package lifecycle

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

// Component is the interface that all lifecycle-managed components must implement.
type Component interface {
	Name() string
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
}

// State represents a lifecycle phase.
type State string

// Lifecycle states.
const (
	StateNew      State = "new"
	StateStarting State = "starting"
	StateStarted  State = "started"
	StateStopping State = "stopping"
	StateStopped  State = "stopped"
	StateFailed   State = "failed"
)

// Public sentinel errors returned by the lifecycle package.
var (
	ErrNilComponent       = errors.New("lifecycle: nil component")
	ErrEmptyComponentName = errors.New("lifecycle: empty component name")
	ErrAlreadyStarted     = errors.New("lifecycle: already started")
	ErrDuplicateComponent = errors.New("lifecycle: duplicate component")
	ErrInvalidState       = errors.New("lifecycle: invalid state")
)

// Manager controls the lifecycle of registered components.
// All exported methods are safe for concurrent use.
type Manager struct {
	mu                sync.Mutex
	components        []Component
	componentNames    map[string]struct{}
	startedComponents []Component
	state             State
}

// NewManager returns an initialized, empty Manager.
func NewManager() *Manager {
	return &Manager{
		state:          StateNew,
		componentNames: make(map[string]struct{}),
	}
}

// State returns the current lifecycle state. A nil receiver returns StateFailed.
func (m *Manager) State() State {
	if m == nil {
		return StateFailed
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	return m.state
}

// Register adds a component to the lifecycle. Components are started in
// registration order and stopped in reverse order.
func (m *Manager) Register(component Component) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if component == nil {
		return ErrNilComponent
	}
	if component.Name() == "" {
		return ErrEmptyComponentName
	}
	if m.state != StateNew {
		return ErrAlreadyStarted
	}
	name := component.Name()
	if _, exists := m.componentNames[name]; exists {
		return fmt.Errorf("lifecycle: duplicate component %q: %w", name, ErrDuplicateComponent)
	}
	m.componentNames[name] = struct{}{}
	m.components = append(m.components, component)
	return nil
}

// Start begins the lifecycle by starting each registered component in order.
func (m *Manager) Start(ctx context.Context) error {
	m.mu.Lock()
	if m.state != StateNew {
		m.mu.Unlock()
		return ErrAlreadyStarted
	}
	m.state = StateStarting
	components := make([]Component, len(m.components))
	copy(components, m.components)
	m.mu.Unlock()

	for _, c := range components {
		if err := startComponent(ctx, c); err != nil {
			m.mu.Lock()
			m.state = StateFailed
			m.mu.Unlock()
			return err
		}
		m.mu.Lock()
		m.startedComponents = append(m.startedComponents, c)
		m.mu.Unlock()
	}

	m.mu.Lock()
	m.state = StateStarted
	m.mu.Unlock()
	return nil
}

// Stop ends the lifecycle by stopping successfully started components in
// reverse order. It is safe to call multiple times; repeated calls after
// a successful or failed stop return nil.
func (m *Manager) Stop(ctx context.Context) error {
	m.mu.Lock()
	switch m.state {
	case StateNew:
		m.state = StateStopped
		m.mu.Unlock()
		return nil
	case StateStopped:
		m.mu.Unlock()
		return nil
	case StateStarting, StateStopping:
		m.mu.Unlock()
		return ErrInvalidState
	case StateStarted, StateFailed:
		components := make([]Component, len(m.startedComponents))
		copy(components, m.startedComponents)
		m.state = StateStopping
		m.startedComponents = nil
		m.mu.Unlock()

		var errs []error
		for i := len(components) - 1; i >= 0; i-- {
			if err := stopComponent(ctx, components[i]); err != nil {
				errs = append(errs, err)
			}
		}

		m.mu.Lock()
		m.state = StateStopped
		m.mu.Unlock()
		return errors.Join(errs...)
	default:
		m.mu.Unlock()
		return ErrInvalidState
	}
}

// startComponent calls component.Start with panic recovery.
func startComponent(ctx context.Context, component Component) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("lifecycle: start component %q panic: %v", component.Name(), r)
		}
	}()

	if err := component.Start(ctx); err != nil {
		return fmt.Errorf("start %s: %w", component.Name(), err)
	}
	return nil
}

// stopComponent calls component.Stop with panic recovery.
func stopComponent(ctx context.Context, component Component) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("lifecycle: stop component %q panic: %v", component.Name(), r)
		}
	}()

	if err := component.Stop(ctx); err != nil {
		return fmt.Errorf("stop %s: %w", component.Name(), err)
	}
	return nil
}
