package orchestrator

import (
	"context"
	"errors"
	"time"
)

var (
	// ErrNoSteps signals that Orchestrator executed without steps
	ErrNoSteps = errors.New("no steps registered")

	// ErrStepNameRequired signals that when appends step no step name provided
	ErrStepNameRequired = errors.New("step name is required")

	// ErrStepPerformNil signals that when appends step Perform function not implemented
	ErrStepPerformNil = errors.New("step Perform function is nil")

	// ErrInvalidStateForRun signals when invalid state found
	ErrInvalidStateForRun = errors.New("cannot run orchestrator in compensated or completed state")
)

// FlowState defines a type of the Flow state
type FlowState string

const (
	// FlowStateRunning sets when the Orchestrator running the flow
	FlowStateRunning FlowState = "RUNNING"

	// FlowStateCompleted sets when the Orchestrator completed the flow
	FlowStateCompleted FlowState = "COMPLETED"

	// FlowStateFailed sets when the Orchestrator failed the flow
	FlowStateFailed FlowState = "FAILED"

	// FlowStateCompensating sets when the Orchestrator compensating the flow step compensation
	FlowStateCompensating FlowState = "COMPENSATING"

	// FlowStateCompensated sets when the Orchestrator compensated the flow step compensation
	FlowStateCompensated FlowState = "COMPENSATED"
)

// StepState represents the lifecycle state of a step.
type StepState string

const (
	// StepStatePending sets for the StepDefinition on queue to execute
	StepStatePending StepState = "PENDING"

	// StepStateSkipped sets for optional StepDefinition on certain condition
	StepStateSkipped StepState = "SKIPPED"

	// StepStateSuccess sets for the StepDefinition on succeeded
	StepStateSuccess StepState = "SUCCESS"

	// StepStateFailed sets for the StepDefinition on failed
	StepStateFailed StepState = "FAILED"
)

// StepFunc defines a type of step execution function
// that will be implemented by the transaction
type StepFunc[T any] func(ctx context.Context, data *T) error

// StepCompensatorFunc defines a type of step compensation that
// will be executed by the transaction based on the step compensation
type StepCompensatorFunc[T any] func(ctx context.Context, data *T) error

// SkipFunc defines a type of skipping process based on specified condition
type SkipFunc[T any] func(ctx context.Context) bool

// StepDefinition defines one saga step, using generics for type safety.
type StepDefinition[T any] struct {
	Name          string
	Perform       StepFunc[T]
	Compensate    StepCompensatorFunc[T]
	Skip          SkipFunc[T]
	ContinueOnErr bool

	// managed by orchestrator
	state       StepState
	startedAt   time.Time
	completedAt time.Time
	err         error
}

// StepDefinitions defines slice object of StepDefinition
type StepDefinitions[T any] []*StepDefinition[T]

// FailOnIndex marks the step at the given index as failed with the provided error.
func (sd StepDefinitions[T]) FailOnIndex(index int, err error) {
	if index < 0 || index >= len(sd) {
		// Should never happen in correct code, but better safe than panic.
		// Log in real impl if logger available; here we silently ignore.
		return
	}

	sd[index].state = StepStateFailed
	sd[index].err = err
}

// CompleteOnIndex marks the step at the given index as successfully completed.
func (sd StepDefinitions[T]) CompleteOnIndex(index int) {
	if index < 0 || index >= len(sd) {
		return
	}

	sd[index].state = StepStateSuccess
	sd[index].completedAt = time.Now()
}

// SkipOnIndex marks the step at the given index as skipped.
func (sd StepDefinitions[T]) SkipOnIndex(index int) {
	if index < 0 || index >= len(sd) {
		return
	}

	sd[index].state = StepStateSkipped
}

type FlowStep struct {
	// ID defines the step Identifier
	ID string `json:"id"`

	// Name defines the step name
	Name string `json:"name"`

	// State defines the state of the step execution statue
	State StepState `json:"state"`

	// ContinueOnFailure indicates that the next step can still be continued if current step failed
	ContinueOnFailure bool `json:"continue_on_error"`

	// Skipped means the step was skipped due to some condition
	Skipped bool `json:"skipped"`

	// StartedAt sets when the step were started
	StartedAt *time.Time `json:"started_at"`

	// CompletedAt sets when the step where completed
	CompletedAt *time.Time `json:"completed_at"`

	// CompensatedAt sets when the step has compensation
	CompensatedAt *time.Time `json:"compensated_at"`

	// Reason refines the reason name for failed or completed step
	Reason string `json:"reason"`
}

// Flow stores the whole transaction information
type Flow struct {
	ID             string      `json:"id"`
	IdempotencyKey string      `json:"idempotency_key"`
	Name           string      `json:"name"`
	State          FlowState   `json:"state"`
	Steps          []*FlowStep `json:"steps"`
	Attempts       uint8       `json:"attempts"`
	StartedAt      time.Time   `json:"started_at"`
	CompletedAt    *time.Time  `json:"completed_at"`
}

// RetryPolicy defines as retry settings to StepDefinition level
type RetryPolicy struct {
	MaxAttempts uint8
	Backoff     func(attempt uint8) time.Duration
}
