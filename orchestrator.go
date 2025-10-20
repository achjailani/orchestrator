package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"time"
)

// Orchestrator is the public API for building and running sagas.
// It's intentionally minimal: just chain steps and run. Everything else is an implementation detail.
type Orchestrator[T any] interface {
	// Then appends a step to the orchestrator flow.
	// Returns self to enable fluent chaining. Idiomatic Go for builders.
	Then(step *StepDefinition[T]) Orchestrator[T]

	// Run executes the saga end-to-end: forward steps, failure handling, compensation, and persistence.
	// It's idempotent—if a store is configured and the same idempotency key is used,
	// it will resume from the last known state instead of restarting.
	// Returns the first non-ContinueOnErr error encountered, or nil on full success.
	Run(ctx context.Context, data *T) error
}

// ctx holds the internal state of the orchestrator.
type ctx[T any] struct {
	// idempotencyKey uniquely identifies this saga instance across retries.
	// If a Store is provided, this key is used to load/resume execution.
	// If no Store is set, this key is ignored
	idempotencyKey string

	// name is a human-readable identifier for the transaction name.
	// Used in logs and persisted execution records. Not required to be unique.
	name string

	// flow is currently unused—likely a leftover from an earlier design.
	flow *Flow

	// steps is the ordered list of flow steps to execute.
	flowSteps StepDefinitions[T]

	// store is optional. If nil, the saga runs entirely in-memory with no recovery.
	// If set, state is persisted before/after each step for crash recovery
	store Store

	// logger is never nil—defaults to no-op if not provided.
	logger Logger

	// retryPolicy controls retry behavior for forward steps only.
	// Compensation steps never retry—cleanup is best-effort.
	// If nil, no retries are attempted
	retryPolicy *RetryPolicy
}

// NewOrchestrator creates a new orchestrator instance.
func NewOrchestrator[T any](name string, opts ...OptionFunc[T]) Orchestrator[T] {
	// Initialize default orchestrator dependencies.
	o := &ctx[T]{
		name:   name,
		logger: &DefaultLogger{},
		flow: &Flow{
			Name: name,
		},
	}

	// Apply user-provided options to the default settings.
	for _, opt := range opts {
		opt(o)
	}

	return o
}

// Then appends a step to the orchestrator definition.
// This is a builder method—designed to be called during setup, not at runtime.
// Returns self to enable fluent chaining: orchestrator.Then(a).Then(b).Then(c).
func (o *ctx[T]) Then(step *StepDefinition[T]) Orchestrator[T] {
	if err := o.validateStep(step); err != nil {
		panic(fmt.Sprintf("invalid step: %v", err))
	}

	// Initialize step state to pending. This field is internal bookkeeping;
	step.state = StepStatePending

	o.flowSteps = append(o.flowSteps, step)
	return o
}

// Run executes the saga from start to finish, with full support for resumption, retries, and compensation.
// This is the only entry point for execution—everything else is set up.
func (o *ctx[T]) Run(ctx context.Context, data *T) error {
	if len(o.flowSteps) == 0 {
		o.logger.Warnf("orchestrator '%s' has no steps defined", o.name)
		return ErrNoSteps
	}

	if o.store != nil {
		persisted, err := o.store.Load(ctx, o.idempotencyKey)
		if err != nil {
			o.logger.Errorf("failed to load execution state for key '%s': %v", o.idempotencyKey, err)
		}

		if persisted != nil {
			o.logger.Infof("resuming existing orchestrator '%s' [key=%s, state=%s, attempts=%d]",
				o.name, o.idempotencyKey, persisted.State, persisted.Attempts)

			o.flow = persisted
			if o.flow.State == FlowStateCompleted {
				o.logger.Infof("orchestrator '%s' already completed; skipping execution (idempotent resume)", o.name)
				return nil
			}

			o.flow.StartedAt = time.Now()
			o.flow.Attempts++

			for _, ps := range o.flow.Steps {
				for i, fs := range o.flowSteps {
					if ps.Name == fs.Name {
						o.flowSteps[i].state = ps.State
					}
				}
			}

			o.logger.Infof("resuming orchestrator '%s' from last known state", o.name)
			return o.resume(ctx, data)
		}
	}

	o.flow.StartedAt = time.Now()
	o.flow.Attempts++
	o.logger.Infof("starting new orchestrator '%s' execution [attempt=%d]", o.name, o.flow.Attempts)
	return o.runOnIndex(ctx, 0, data)
}

// runOnIndex executes the orchestrator steps sequentially, starting from the given index.
// It supports retry logic, step skipping, error handling, and compensation.
// If a step fails and ContinueOnErr=false, execution halts and the error is returned.
// On success or allowed continuation, subsequent steps are processed in order.
// State is persisted before exit if a Store is configured.
func (o *ctx[T]) runOnIndex(ctx context.Context, startIdx int, data *T) error {
	o.logger.Infof("Starting flow: %q (from step index %d)", o.name, startIdx)
	defer func() {
		o.Persist(ctx)
	}()

	for index := startIdx; index < len(o.flowSteps); index++ {
		step := o.flowSteps[index]

		o.flowSteps[index].startedAt = time.Now()

		if err := ctx.Err(); err != nil {
			o.flow.State = FlowStateFailed
			o.flowSteps.FailOnIndex(index, err)
			o.logger.Errorf("Flow canceled before executing step %q: %v", step.Name, err)
			return ctx.Err()
		}

		if step.Skip != nil && step.Skip(ctx) {
			o.flowSteps.SkipOnIndex(index)
			o.logger.Infof("Step %q skipped (Skip function returned true)", step.Name)
			continue
		}

		var execErr error
		attempt := uint8(1)
		if o.retryPolicy != nil {
			for {
				execErr = step.Perform(ctx, data)
				if execErr == nil {
					break
				}

				o.logger.Warnf("Step %q failed on attempt %d: %v", step.Name, attempt, execErr)
				if attempt >= o.retryPolicy.MaxAttempts {
					o.logger.Errorf("Step %q exceeded max retry attempts (%d)", step.Name, o.retryPolicy.MaxAttempts)
					break
				}

				if o.retryPolicy.Backoff != nil {
					sleep := o.retryPolicy.Backoff(attempt)
					o.logger.Infof("Retrying step %q after backoff: %v", step.Name, sleep)
					select {
					case <-time.After(sleep):
					case <-ctx.Done():
						o.flow.State = FlowStateFailed
						o.flowSteps.FailOnIndex(index, ctx.Err())

						return ctx.Err()
					}
				}
				attempt++
			}
		} else {
			execErr = step.Perform(ctx, data)
		}

		if execErr != nil {
			o.logger.Errorf("Step %q execution failed: %v", step.Name, execErr)
			if step.Compensate != nil {
				o.logger.Infof("Starting compensation for step %q", step.Name)
				if compErr := step.Compensate(ctx, data); compErr != nil {
					o.logger.Errorf("Compensation for step %q failed: %v", step.Name, compErr)
				} else {
					o.logger.Infof("Compensation for step %q completed successfully", step.Name)
				}
			}

			if !step.ContinueOnErr {
				o.flow.State = FlowStateFailed
				o.flowSteps.FailOnIndex(index, ctx.Err())
				o.logger.Infof("Halting flow: step %q disallowed continuation", step.Name)
				return execErr
			}

			o.logger.Warnf("Step %q failed but ContinueOnErr=true; continuing flow", step.Name)
			continue
		}

		o.flowSteps.CompleteOnIndex(index)
		o.logger.Infof("Step %q completed successfully", step.Name)
	}

	completedAt := time.Now()
	o.flow.State = FlowStateCompleted
	o.flow.CompletedAt = &completedAt

	o.logger.Infof("Flow %q completed successfully", o.name)
	return nil
}

// resume continues a previously persisted flow from the last failed step.
// It validates store configuration and idempotency key before resuming.
// If all steps were completed, the flow is marked as completed and no action is taken.
// Otherwise, it restarts execution from the first failed step index.
func (o *ctx[T]) resume(ctx context.Context, data *T) error {
	if o.store == nil {
		return errors.New("persistence not enabled or store not configured for resume")
	}
	if o.idempotencyKey == "" {
		return errors.New("idempotency key required to resume")
	}

	if o.flow.State == FlowStateCompleted {
		o.logger.Infof("Execution already completed for idempotency key '%s'", o.idempotencyKey)
		return nil
	}

	resumeIdx := -1
	for i, fs := range o.flowSteps {
		if fs.state == StepStateFailed {
			resumeIdx = i
			break
		}
	}

	if resumeIdx == -1 {
		completedAt := time.Now()
		o.flow.State = FlowStateCompleted
		o.flow.CompletedAt = &completedAt
		return nil
	}

	err := o.runOnIndex(ctx, resumeIdx, data)
	return err
}

// validateStep checks that a step is minimally valid before adding it to the saga.
// We require a name and a Perform function—everything else is optional.
func (o *ctx[T]) validateStep(step *StepDefinition[T]) error {
	// Returns error when
	if step.Name == "" {
		return ErrStepNameRequired
	}

	if reflect.ValueOf(step.Perform).IsNil() {
		return ErrStepPerformNil
	}

	return nil
}

// Persist saves the current flow state to the configured Store, if available.
// It ensures progress and recovery data are recorded between steps.
// If no Store is configured, this method performs no operation.
func (o *ctx[T]) Persist(ctx context.Context) {
	if o.store != nil {
		err := o.store.Save(ctx, o.flow)
		if err != nil {
			o.logger.Errorf("error saving execution: %v", err)
		}
	}
}
