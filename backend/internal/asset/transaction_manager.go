package asset

import (
	"context"
	"fmt"

	"gorm.io/gorm"
)

type TransactionManager interface {
	WithTransaction(
		ctx context.Context,
		fn func(repos Repositories) error,
	) error
}

type transactionManager struct {
	db *gorm.DB
}

func NewTransactionManager(db *gorm.DB) TransactionManager {
	return &transactionManager{db: db}
}

func (m *transactionManager) WithTransaction(
	ctx context.Context,
	fn func(repos Repositories) error,
) error {
	err := m.db.
		WithContext(ctx).
		Transaction(func(tx *gorm.DB) error {
			repos := newRepositories(tx)
			return fn(repos)
		})
	if err != nil {
		return fmt.Errorf("execute transaction: %w", err)
	}

	return nil
}
