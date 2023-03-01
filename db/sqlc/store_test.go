package db

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTransferTx(t *testing.T) {

	store := NewStore(testDB)

	// 送信元
	account1 := createRandomAccount(t)
	// 送信先
	account2 := createRandomAccount(t)

	fmt.Printf("登録直後の送受信するAccounts の残高: account1=%v, account2=%v\n", account1.Balance, account2.Balance)

	n := 5
	amount := int64(100)

	errs := make(chan error)
	results := make(chan TransferTxResult)

	// テスト実行時にトランザクションの場合には、ロック等を考慮してgoroutine で複数はしらす
	for i := 0; i < n; i++ {
		go func() {
			result, err := store.TransferTx(context.Background(), TransferTxParams{
				FromAccountID: account1.ID,
				ToAccountID:   account2.ID,
				Amount:        amount,
			})

			errs <- err
			results <- result
		}()
	}

	updatedLoopNumMap := make(map[int]bool)
	for i := 0; i < n; i++ {
		// goroutine で取得した値にエラーがないかどうかのチェック
		err := <-errs
		require.NoError(t, err)

		// goroutine で登録した値が空でないかのチェック
		result := <-results
		require.NotEmpty(t, result)

		// transfers テーブルのチェック
		transfer := result.Transfer
		// 送信者のアカウントIDが一致しているかどうか
		require.Equal(t, account1.ID, transfer.FromAccountID)
		// 受信者のアカウントIDが一致しているかどうか
		require.Equal(t, account2.ID, transfer.ToAccountID)
		// 送信者の送った金額が一致しているかどうか
		require.Equal(t, amount, transfer.Amount)
		// IDはインクリメントになるので、transferが登録されているかどうかは、0でも空かどうかを判定できる
		require.NotZero(t, transfer.ID)
		// 登録時間もチェック
		require.NotZero(t, transfer.CreatedAt)

		_, err = store.GetTransfer(context.Background(), transfer.ID)
		// err が存在しない場合には、transfers.テーブルにも該当のレコードが登録されている。
		require.NoError(t, err)

		// entries テーブルのチェック
		// 送信側のチェック
		fromEntry := result.FromEntry
		require.NotEmpty(t, fromEntry)
		// 送信者のアカウントIDが一致しているかどうか
		require.Equal(t, account1.ID, fromEntry.AccountID)
		// 送信者の送った金額が一致しているかどうか
		require.Equal(t, -amount, fromEntry.Amount)
		// IDはインクリメントになるので、entriesが登録されているかどうかは、0でも空かどうかを判定できる
		require.NotZero(t, fromEntry.ID)
		// 登録時間もチェック
		require.NotZero(t, fromEntry.CreatedAt)

		// 受信側のチェック
		toEntry := result.ToEntry
		require.NotEmpty(t, toEntry)
		// 送信者のアカウントIDが一致しているかどうか
		require.Equal(t, account2.ID, toEntry.AccountID)
		// 送信者の送った金額が一致しているかどうか
		require.Equal(t, amount, toEntry.Amount)
		// IDはインクリメントになるので、entriesが登録されているかどうかは、0でも空かどうかを判定できる
		require.NotZero(t, toEntry.ID)
		// 登録時間もチェック
		require.NotZero(t, toEntry.CreatedAt)

		// err が存在しない場合には、entries.テーブルにも該当のレコードが登録されている。
		_, err = store.GetEntry(context.Background(), fromEntry.ID)
		require.NoError(t, err)
		_, err = store.GetEntry(context.Background(), toEntry.ID)
		require.NoError(t, err)

		// アカウントのチェック
		fromAccount := result.FromAccount
		toAccount := result.ToAccount
		fmt.Printf("bofore update: from = %v, to = %v\n", fromAccount.Balance, toAccount.Balance)

		// 更新したアカウントが空でないこと
		require.NotEmpty(t, fromAccount)
		require.NotEmpty(t, toAccount)

		// 送信者のIDが一致していること
		require.Equal(t, fromAccount.ID, account1.ID)
		// 受信者のIDが一致していること
		require.Equal(t, toAccount.ID, account2.ID)

		diff1 := account1.Balance - fromAccount.Balance
		diff2 := toAccount.Balance - account2.Balance

		// 送信者と受信者が受け取った値が一致することの確認
		require.Equal(t, diff1, diff2)

		// 送信する値は、正の数字でなければならない
		require.True(t, diff1 > 0)

		// デッドロックを考慮してループで同じ金額を入力しているので、送信した金額は固定値である「amount」で割り切れなければならない。
		require.True(t, diff1%amount == 0)

		fmt.Printf("送受信中の送受信するAccounts の残高: account1=%v, account2=%v\n", fromAccount.Balance, toAccount.Balance)
		k := int(diff1 / amount)
		// 1 ~ 5 のループにしているので、送金額は固定値で割ったあとの数は　1以上5以下でなければならない。
		require.True(t, k >= 1 && k <= n)
		require.NotContains(t, updatedLoopNumMap, k)
		updatedLoopNumMap[k] = true
	}

	// 送信者がデータベースから取得できることの確認
	updatedFromAccount, err := testqueries.GetAccount(context.Background(), account1.ID)
	require.NoError(t, err)
	// 送信者がデータベースから取得できることの確認
	updatedToAccount, err := testqueries.GetAccount(context.Background(), account2.ID)
	require.NoError(t, err)

	fmt.Printf("更新完了後の送受信するAccounts の残高: account1=%v, account2=%v\n", updatedFromAccount.Balance, updatedToAccount.Balance)
	// 送信者の送信前の残高 - 送信した総金額 == 更新後の残高
	require.Equal(t, account1.Balance-int64(n)*amount, updatedFromAccount.Balance)
	// 受信者の受信前の残高 + 送信された総金額 == 更新後の残高
	require.Equal(t, account2.Balance+int64(n)*amount, updatedToAccount.Balance)
}

func TestTransferTxDeadLock(t *testing.T) {

	store := NewStore(testDB)

	// 送信元
	account1 := createRandomAccount(t)
	// 送信先
	account2 := createRandomAccount(t)

	fmt.Println("---------------------------------------------------------------------------------------------------------------------")
	fmt.Printf("登録直後の送受信するAccounts の残高: account1=%v, account2=%v\n", account1.Balance, account2.Balance)

	n := 10
	amount := int64(100)

	errs := make(chan error)

	// テスト実行時にトランザクションの場合には、ロック等を考慮してgoroutine で複数はしらす
	for i := 0; i < n; i++ {
		fromAccountID := account1.ID
		toAccountID := account2.ID

		// 10ループ中、送受信するアカウント半分を逆転させてデッドロックが起きないかを検証
		if i%2 == 1 {
			fromAccountID = account2.ID
			toAccountID = account1.ID
		}
		go func() {
			_, err := store.TransferTx(context.Background(), TransferTxParams{
				FromAccountID: fromAccountID,
				ToAccountID:   toAccountID,
				Amount:        amount,
			})

			errs <- err
		}()
	}

	for i := 0; i < n; i++ {
		err := <-errs
		require.NoError(t, err)
	}

	// 送信者がデータベースから取得できることの確認
	updatedFromAccount, err := testqueries.GetAccount(context.Background(), account1.ID)
	require.NoError(t, err)
	// 送信者がデータベースから取得できることの確認
	updatedToAccount, err := testqueries.GetAccount(context.Background(), account2.ID)
	require.NoError(t, err)

	fmt.Println("---------------------------------------------------------------------------------------------------------------------")
	fmt.Printf("更新完了後の送受信するAccounts の残高: account1=%v, account2=%v\n", updatedFromAccount.Balance, updatedToAccount.Balance)
	// 送信者の送信前の残高 - 送信した総金額 == 更新後の残高
	require.Equal(t, account1.Balance, updatedFromAccount.Balance)
	// 受信者の受信前の残高 + 送信された総金額 == 更新後の残高
	require.Equal(t, account2.Balance, updatedToAccount.Balance)
}
