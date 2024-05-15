package main

import (
	"context"
	"fmt"
	"testing"

	"github.com/pashagolub/pgxmock"
)

// will test that order with a different status, cannot be cancelled
func TestShouldNotCancelOrderWithNonPendingStatus(t *testing.T) {
	// open database stub
	mock, err := pgxmock.NewConn()
	if err != nil {
		t.Errorf("An error '%s' was not expected when opening a stub database connection", err)
	}
	defer mock.Close(context.Background())

	// columns are prefixed with "o" since we used sqlstruct to generate them
	columns := []string{"o_id", "o_status"}
	// expect transaction begin
	mock.ExpectBegin()
	// expect query to fetch order and user, match it with regexp
	mock.ExpectQuery("SELECT (.+) FROM orders AS o INNER JOIN users AS u (.+) FOR UPDATE").
		WithArgs(1).
		WillReturnRows(mock.NewRows(columns).AddRow(1, 1))
	// expect transaction rollback, since order status is "cancelled"
	mock.ExpectRollback()

	// run the cancel order function
	err = cancelOrder(1, mock)
	if err != nil {
		t.Errorf("Expected no error, but got %s instead", err)
	}
	// we make sure that all expectations were met
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}

// will test order cancellation
func TestShouldRefundUserWhenOrderIsCancelled(t *testing.T) {
	// open database stub
	mock, err := pgxmock.NewConn()
	if err != nil {
		t.Errorf("An error '%s' was not expected when opening a stub database connection", err)
	}
	defer mock.Close(context.Background())

	// columns are prefixed with "o" since we used sqlstruct to generate them
	columns := []string{"o_id", "o_status", "o_value", "o_reserved_fee", "u_id", "u_balance"}
	// expect transaction begin
	mock.ExpectBegin()
	// expect query to fetch order and user, match it with regexp
	mock.ExpectQuery("SELECT (.+) FROM orders AS o INNER JOIN users AS u (.+) FOR UPDATE").
		WithArgs(1).
		WillReturnRows(mock.NewRows(columns).AddRow(1, 0, 25.75, 3.25, 2, 10.00))
	// expect user balance update
	mock.ExpectPrepare("balance_stmt", "UPDATE users SET balance").ExpectExec().
		WithArgs(25.75+3.25, 2).                         // refund amount, user id
		WillReturnResult(pgxmock.NewResult("UPDATE", 1)) // no insert id, 1 affected row
	// expect order status update
	mock.ExpectPrepare("order_stmt", "UPDATE orders SET status").ExpectExec().
		WithArgs(orderCancelled, 1).                     // status, id
		WillReturnResult(pgxmock.NewResult("UPDATE", 1)) // no insert id, 1 affected row
	// expect a transaction commit
	mock.ExpectCommit()

	// run the cancel order function
	err = cancelOrder(1, mock)
	if err != nil {
		t.Errorf("Expected no error, but got %s instead", err)
	}
	// we make sure that all expectations were met
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}

// will test order cancellation
func TestShouldRollbackOnError(t *testing.T) {
	// open database stub
	mock, err := pgxmock.NewConn()
	if err != nil {
		t.Errorf("An error '%s' was not expected when opening a stub database connection", err)
	}
	defer mock.Close(context.Background())

	// expect transaction begin
	mock.ExpectBegin()
	// expect query to fetch order and user, match it with regexp
	mock.ExpectQuery("SELECT (.+) FROM orders AS o INNER JOIN users AS u (.+) FOR UPDATE").
		WithArgs(1).
		WillReturnError(fmt.Errorf("Some error"))
	// should rollback since error was returned from query execution
	mock.ExpectRollback()

	// run the cancel order function
	err = cancelOrder(1, mock)
	// error should return back
	if err == nil {
		t.Error("Expected error, but got none")
	}
	// we make sure that all expectations were met
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}
