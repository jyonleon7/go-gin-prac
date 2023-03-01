package db

import (
	"context"
	"database/sql"
	"fmt"
)

type Store struct {
	// Queries のvalue 部分を省略すると、Store.Queries.xxx ではなく、Store.xxxのように、直接メソッドを呼ぶことができる
	*Queries
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{
		Queries: New(db),
		db:      db,
	}
}

func (store *Store) execTx(ctx context.Context, fn func(*Queries) error) error {
	tx, err := store.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}

	// トランザクションの準備
	q := New(tx)
	// コールバック内で受け取ったsqlの実行
	err = fn(q)
	if err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			return fmt.Errorf("err: %v, rbErr: %v\n", err, rbErr)
		}
		return err
	}

	return tx.Commit()
}

type TransferTxParams struct {
	FromAccountID int64 `json:"from_account_id"`
	ToAccountID   int64 `json:"to_account_id"`
	Amount        int64 `json:"amount"`
}

type TransferTxResult struct {
	// 送金履歴
	Transfer Transfer `json:"transfer"`
	// 残高が更新された、送信元のアカウント
	FromAccount Account `json:"from_account"`
	// 残高が更新された、送信先のアカウント
	ToAccount Account `json:"to_account"`
	// 送信元の入出金履歴
	FromEntry Entry `json:"from_entry"`
	// 送信先の入出金履歴
	ToEntry Entry `json:"to_entry"`
}

func (store *Store) TransferTx(ctx context.Context, arg TransferTxParams) (TransferTxResult, error) {
	var result TransferTxResult

	err := store.execTx(ctx, func(q *Queries) error {
		var err error
		// ① 送金を行う
		result.Transfer, err = q.CreateTransfer(ctx, CreateTransferParams{
			FromAccountID: arg.FromAccountID,
			ToAccountID:   arg.ToAccountID,
			Amount:        arg.Amount,
		})

		if err != nil {
			return err
		}

		// ② 送信先と送信元のアカウント入出金を登録する
		// ②-1, 送信元
		result.FromEntry, err = q.CreateEntry(ctx, CreateEntryParams{
			AccountID: arg.FromAccountID,
			Amount:    -arg.Amount,
		})

		if err != nil {
			return err
		}
		// ②-2, 送信先
		result.ToEntry, err = q.CreateEntry(ctx, CreateEntryParams{
			AccountID: arg.ToAccountID,
			Amount:    arg.Amount,
		})

		if err != nil {
			return err
		}

		// ③ 送信元と送信先のアカウントの合計金額を計算し、アップデートする
		// TODO: Update

		// loop 処理で同時更新を行なった場合には、排他ロックは必ずかかるが順序次第でdeadlockが起こる。そのため、更新する順番を揃える必要があるらしい。
		// ③-1
		// BEGIN;
		// UPDATE accounts SET balance = balance + 100 WHERE id = 1;
		// UPDATE accounts SET balance = balance - 100 WHERE id = 2;
		// COMMIT;

		// ③-2
		// BEGIN;
		// UPDATE accounts SET balance = balance + 100 WHERE id = 2;
		// UPDATE accounts SET balance = balance - 100 WHERE id = 1;
		// COMMIT;

		// ③-1 と ③-2を同時に行うと、③-1の2つ目が何らかの処理で遅れると③-2の1つ目がロックされる。
		// それを回避するために、accounts.id が低いものを順番に行うらしい。※めちゃむずい。
		if arg.FromAccountID < arg.ToAccountID {
			result.FromAccount, result.ToAccount, err = addMoney(ctx, q, arg.FromAccountID, -arg.Amount, arg.ToAccountID, arg.Amount)
		} else {
			result.ToAccount, result.FromAccount, err = addMoney(ctx, q, arg.ToAccountID, arg.Amount, arg.FromAccountID, -arg.Amount)
		}
		if err != nil {
			return err
		}

		return nil
	})

	return result, err
}

func addMoney(
	ctx context.Context,
	q *Queries,
	fromAccountID int64,
	amount1 int64,
	toAccountID int64,
	amount2 int64,
) (fromAccount Account, toAccount Account, err error) {
	fromAccount, err = q.AddAccountBalance(ctx, AddAccountBalanceParams{
		ID:     fromAccountID,
		Amount: amount1,
	})
	if err != nil {
		return
	}
	toAccount, err = q.AddAccountBalance(ctx, AddAccountBalanceParams{
		ID:     toAccountID,
		Amount: amount2,
	})
	return
}
