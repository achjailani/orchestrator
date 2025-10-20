package orchestrator_test

import (
	"context"
	"errors"
	"fmt"
	"github.com/achjailani/orchestrator"
	"testing"
)

type SignWorkflow struct {
	RequestValid    bool
	AccessOK        bool
	BalanceOK       bool
	DocumentSigned  bool
	BalanceDeducted bool
}

func TestOrchestrator(t *testing.T) {
	ctx := context.Background()
	data := &SignWorkflow{
		RequestValid: true,
		AccessOK:     true,
		BalanceOK:    true,
	}

	store := orchestrator.NewInMemoryStore()
	executor := orchestrator.NewOrchestrator[SignWorkflow]("sign-document",
		orchestrator.WithStore[SignWorkflow](store),
	)

	executor.
		Then(&orchestrator.StepDefinition[SignWorkflow]{
			Name: "ValidateRequest",
			Perform: func(ctx context.Context, data *SignWorkflow) error {
				if !data.RequestValid {
					return errors.New("invalid request")
				}
				fmt.Println("✅ Request validated")
				return nil
			},
		})

	executor.Then(&orchestrator.StepDefinition[SignWorkflow]{
		Name: "ValidateAccess",
		Perform: func(ctx context.Context, data *SignWorkflow) error {
			if !data.AccessOK {
				return errors.New("access denied")
			}
			fmt.Println("✅ Access validated")
			return nil
		},
	})

	// append
	executor.Then(&orchestrator.StepDefinition[SignWorkflow]{
		Name: "CheckBalance",
		Perform: func(ctx context.Context, data *SignWorkflow) error {
			if !data.BalanceOK {
				return errors.New("insufficient balance")
			}
			fmt.Println("✅ Balance checked")
			return nil
		},
	})

	executor.
		Then(&orchestrator.StepDefinition[SignWorkflow]{
			Name: "SignDocument",
			Perform: func(ctx context.Context, data *SignWorkflow) error {
				data.DocumentSigned = true
				fmt.Println("🖋️ Document signed")
				return nil
			},
			Compensate: func(ctx context.Context, data *SignWorkflow) error {
				if data.DocumentSigned {
					data.DocumentSigned = false
					fmt.Println("↩️ Undo document signing")
				}
				return nil
			},
		})

	executor.Then(&orchestrator.StepDefinition[SignWorkflow]{
		Name: "DeductBalance",
		Perform: func(ctx context.Context, data *SignWorkflow) error {
			data.BalanceDeducted = true
			fmt.Println("💰 Balance deducted")
			return nil
		},
		Compensate: func(ctx context.Context, data *SignWorkflow) error {
			if data.BalanceDeducted {
				data.BalanceDeducted = false
				fmt.Println("↩️ Revert balance deduction")
			}
			return nil
		},
		Skip: func(ctx context.Context) bool {
			return false
		},
		ContinueOnErr: true,
	})

	if err := executor.Run(ctx, data); err != nil {
		fmt.Println("Saga failed:", err)
	}
}
