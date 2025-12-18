package repository

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/caspianex/exchange-backend/const/queries"
	"github.com/caspianex/exchange-backend/internal/domain"
	"github.com/caspianex/exchange-backend/pkg/database"
)

type TransactionRepository struct {
	db *database.Postgres
}

func NewTransactionRepository(db *database.Postgres) *TransactionRepository {
	return &TransactionRepository{db: db}
}

func (r *TransactionRepository) Create(ctx context.Context, tx *domain.Transaction) error {
	return r.db.QueryRowContext(
		ctx, queries.TransactionCreateQuery,
		tx.UserID, tx.WalletID, tx.Type, tx.Amount, tx.Fee, tx.Status, tx.TxHash,
	).Scan(&tx.ID, &tx.CreatedAt, &tx.UpdatedAt)
}

func (r *TransactionRepository) GetByID(ctx context.Context, id int64) (*domain.Transaction, error) {
	var tx domain.Transaction
	err := r.db.GetContext(ctx, &tx, queries.TransactionGetByIDQuery, id)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("transaction not found")
	}
	return &tx, err
}

func (r *TransactionRepository) GetUserTransactions(ctx context.Context, userID int64, limit, offset int) ([]domain.Transaction, error) {
	var transactions []domain.Transaction
	err := r.db.SelectContext(ctx, &transactions, queries.TransactionGetUserTransactionsQuery, userID, limit, offset)
	return transactions, err
}

func (r *TransactionRepository) Update(ctx context.Context, tx *domain.Transaction) error {
	return r.db.QueryRowContext(
		ctx, queries.TransactionUpdateQuery,
		tx.Status, tx.TxHash, tx.ID,
	).Scan(&tx.UpdatedAt)
}

func (r *TransactionRepository) GetPendingTransactions(ctx context.Context) ([]domain.Transaction, error) {
	var transactions []domain.Transaction
	err := r.db.SelectContext(ctx, &transactions, queries.TransactionGetPendingQuery)
	return transactions, err
}

func (r *TransactionRepository) GetAllTransactions(ctx context.Context, limit, offset int) ([]domain.Transaction, error) {
	var transactions []domain.Transaction
	err := r.db.SelectContext(ctx, &transactions, queries.TransactionGetAllQuery, limit, offset)
	return transactions, err
}
