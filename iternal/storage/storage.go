package storage

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Tomap-Tomap/go-loyalty-service/iternal/hasher"
	"github.com/Tomap-Tomap/go-loyalty-service/iternal/models"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

var ErrIDExistForCurUsr error = fmt.Errorf("id exist for current user")
var ErrIDExistForAnotherUsr error = fmt.Errorf("id exist for another user")
var ErrInsufficientFunds error = fmt.Errorf("insufficient funds")

type retryPolicy struct {
	retryCount int
	duration   int
	increment  int
}

type Storage struct {
	conn        *pgx.Conn
	retryPolicy retryPolicy
}

func NewStorage(conn *pgx.Conn) (*Storage, error) {
	rp := retryPolicy{3, 1, 2}
	s := &Storage{conn: conn, retryPolicy: rp}

	if err := s.createTables(); err != nil {
		return nil, fmt.Errorf("create tables in database: %w", err)
	}

	return s, nil
}

func (s *Storage) createTables() error {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Second)
	defer cancel()

	createUserQuery := `
		CREATE TABLE IF NOT EXISTS users (
			Login VARCHAR(150) PRIMARY KEY,
			Password CHAR(64),
			Salt VARCHAR(150)
		);
	`

	createStatusQuery := `
		CREATE TABLE IF NOT EXISTS statuses (
			Name VARCHAR(50) PRIMARY KEY
		);
		INSERT INTO statuses VALUES ('NEW'), ('PROCESSING'), ('INVALID'), ('PROCESSED')
			ON CONFLICT (Name) DO NOTHING;
	`

	createOrdersQuery := `
		CREATE TABLE IF NOT EXISTS orders (
			Number VARCHAR(150) PRIMARY KEY,
			Login VARCHAR(150) REFERENCES users(Login),
			Status VARCHAR(50) REFERENCES statuses(Name),
			UploadedAt TIMESTAMP WITH TIME ZONE
		);
		CREATE INDEX IF NOT EXISTS uploaded_at_idx ON orders (UploadedAt);
		CREATE OR REPLACE FUNCTION orders_stamp() RETURNS trigger AS $orders_stamp$
			BEGIN
				NEW.UploadedAt := current_timestamp;
				RETURN NEW;
			END;
		$orders_stamp$ LANGUAGE plpgsql;
		CREATE OR REPLACE TRIGGER orders_stamp BEFORE INSERT OR UPDATE ON orders
			FOR EACH ROW EXECUTE PROCEDURE orders_stamp();
	`

	createBalanceQuery := `
		CREATE TABLE IF NOT EXISTS balances (
			Login VARCHAR(150) REFERENCES users(Login),
			Order_number VARCHAR(150),
			ProcessedAt TIMESTAMP WITH TIME ZONE,
			Sum DOUBLE PRECISION
		);
		CREATE INDEX IF NOT EXISTS processed_at_idx ON balances (ProcessedAt);
		CREATE INDEX IF NOT EXISTS order_number_idx ON balances (Order_number);
		CREATE OR REPLACE FUNCTION balances_stamp() RETURNS trigger AS $balances_stamp$
			DECLARE
				total_balance DOUBLE PRECISION;
			BEGIN
				SELECT Sum INTO total_balance FROM balances WHERE Login = NEW.Login FOR UPDATE;

				IF (total_balance IS NULL AND NEW.SUM < 0) OR (SELECT SUM(Sum) FROM balances WHERE Login = NEW.Login) + NEW.SUM < 0 THEN
					RAISE EXCEPTION 'insufficient funds';
				END IF;	

				NEW.ProcessedAt := current_timestamp;
				RETURN NEW;
			END;
		$balances_stamp$ LANGUAGE plpgsql;
		CREATE OR REPLACE TRIGGER balances_stamp BEFORE INSERT OR UPDATE ON balances
			FOR EACH ROW EXECUTE PROCEDURE balances_stamp();
	`
	err := pgx.BeginFunc(ctx, s.conn, func(tx pgx.Tx) error {
		_, err := retry2(ctx, s.retryPolicy, func() (pgconn.CommandTag, error) {
			return s.conn.Exec(ctx, createUserQuery)
		})

		if err != nil {
			return fmt.Errorf("create users table: %w", err)
		}

		_, err = retry2(ctx, s.retryPolicy, func() (pgconn.CommandTag, error) {
			return s.conn.Exec(ctx, createStatusQuery)
		})

		if err != nil {
			return fmt.Errorf("create statuses table: %w", err)
		}

		_, err = retry2(ctx, s.retryPolicy, func() (pgconn.CommandTag, error) {
			return s.conn.Exec(ctx, createOrdersQuery)
		})

		if err != nil {
			return fmt.Errorf("create orders table: %w", err)
		}

		_, err = retry2(ctx, s.retryPolicy, func() (pgconn.CommandTag, error) {
			return s.conn.Exec(ctx, createBalanceQuery)
		})

		if err != nil {
			return fmt.Errorf("create balances table: %w", err)
		}

		return nil
	})

	return err
}

func (s *Storage) CreateUser(ctx context.Context, u models.User) error {
	query := `
		INSERT INTO users (Login, Password, Salt) VALUES ($1, $2, $3);
	`

	sp, err := hasher.NewSaltPassword(u.Password)

	if err != nil {
		return fmt.Errorf("generate password hash: %w", err)
	}

	_, err = retry2(ctx, s.retryPolicy, func() (pgconn.CommandTag, error) {
		return s.conn.Exec(ctx, query, u.Login, sp.Password, sp.Salt)
	})

	return err
}

func (s *Storage) GetUser(ctx context.Context, login string) (*models.User, error) {
	u := &models.User{}
	err := retry(ctx, s.retryPolicy, func() error {
		return s.conn.QueryRow(ctx, "SELECT Login, Password, Salt FROM users WHERE Login = $1", login).Scan(u)
	})

	if err != nil {
		return nil, fmt.Errorf("get user %s: %w", login, err)
	}

	return u, nil
}

func (s *Storage) AddOrder(ctx context.Context, order string, login string) error {
	query := `
		INSERT INTO orders (Number, Login, Status)
			VALUES ($1, $2, $3)
	`

	_, err := retry2(ctx, s.retryPolicy, func() (pgconn.CommandTag, error) {
		return s.conn.Exec(ctx, query, order, login, models.StatusNew)
	})

	var tError *pgconn.PgError
	if errors.As(err, &tError) && tError.Code == pgerrcode.UniqueViolation {
		var l string
		err := retry(ctx, s.retryPolicy, func() error {
			return s.conn.QueryRow(ctx, "SELECT Login FROM orders WHERE Number = $1", order).Scan(&l)
		})

		if err != nil {
			return err
		}

		if l == login {
			return ErrIDExistForCurUsr
		}

		return ErrIDExistForAnotherUsr
	}

	return err
}

func (s *Storage) GetOrders(ctx context.Context, login string) ([]models.Order, error) {
	query := `
		SELECT o.number, b.sum as accrual, o.uploadedat, o.status FROM orders as o
		LEFT JOIN balances as b ON o.number = b.Order_number AND b.sum > 0
		WHERE o.Login = $1
		ORDER BY UploadedAt;
	`
	orders, err := retry2(ctx, s.retryPolicy, func() ([]models.Order, error) {
		rows, err := s.conn.Query(ctx, query, login)

		if err != nil {
			return nil, err
		}

		orders := make([]models.Order, 0)

		defer rows.Close()

		for rows.Next() {
			var o models.Order
			err := rows.Scan(&o)

			if err != nil {
				return nil, err
			}

			orders = append(orders, o)
		}

		return orders, nil
	})

	return orders, err
}

func (s *Storage) GetBalance(ctx context.Context, login string) (*models.UserBalance, error) {
	query := `
		SELECT cur_sum.Current, withdrawn_sum.Withdrawn
			FROM (
				SELECT Login, SUM(Sum) as Current
					FROM balances WHERE Login = $1 GROUP BY Login
			) as cur_sum
			LEFT JOIN (
				SELECT Login, SUM(-Sum) as Withdrawn
					FROM balances WHERE Sum < 0 AND LOGIN = $1 GROUP BY Login
			) as withdrawn_sum ON cur_sum.Login = withdrawn_sum.Login;
	`
	var b models.UserBalance
	err := retry(ctx, s.retryPolicy, func() error {
		return s.conn.QueryRow(ctx, query, login).Scan(&b)
	})

	if errors.Is(err, pgx.ErrNoRows) {
		return &b, nil
	}

	return &b, err
}

func (s *Storage) DoWithdrawal(ctx context.Context, login string, ob models.OrderBalance) error {
	queryBalances := `
		INSERT INTO balances (Login, Order_number, Sum)
			VALUES ($1, $2, $3)
	`
	_, err := retry2(ctx, s.retryPolicy, func() (pgconn.CommandTag, error) {
		return s.conn.Exec(ctx, queryBalances, login, ob.Order, -ob.Sum)
	})

	var tError *pgconn.PgError
	if errors.As(err, &tError) && tError.Message == "insufficient funds" {
		return ErrInsufficientFunds
	}

	return err
}

func (s *Storage) GetWithdrawal(ctx context.Context, login string) ([]models.OrderBalance, error) {
	query := `
		SELECT order_number as order, -sum as sum, processedat FROM balances
		WHERE Login = $1 AND sum < 0
		ORDER BY processedat
	`
	orderBalance, err := retry2(ctx, s.retryPolicy, func() ([]models.OrderBalance, error) {
		rows, err := s.conn.Query(ctx, query, login)

		if err != nil {
			return nil, err
		}

		orderBalance := make([]models.OrderBalance, 0)

		defer rows.Close()

		for rows.Next() {
			var ob models.OrderBalance
			err := rows.Scan(&ob)

			if err != nil {
				return nil, err
			}

			orderBalance = append(orderBalance, ob)
		}

		return orderBalance, nil
	})

	return orderBalance, err
}

func (s *Storage) GetNotProcessedOrders(ctx context.Context) ([]string, error) {
	query := `
		SELECT
			number
		FROM
			orders
		WHERE
			status NOT IN ($1, $2)
		ORDER BY
			uploadedat;
	`

	numbers, err := retry2(ctx, s.retryPolicy, func() ([]string, error) {
		rows, err := s.conn.Query(ctx, query, models.StatusInvalid, models.StatusProcessed)

		if err != nil {
			return nil, err
		}

		numbers := make([]string, 0)

		defer rows.Close()

		for rows.Next() {
			var n string
			err := rows.Scan(&n)

			if err != nil {
				return nil, err
			}

			numbers = append(numbers, n)
		}

		return numbers, nil
	})

	return numbers, err
}

func (s *Storage) UpdateOrder(ctx context.Context, o models.Order) error {
	query := `
		WITH t AS (
			UPDATE orders
			SET status = $1
			WHERE number = $2
			RETURNING *
		)
		INSERT INTO balances (login, order_number, sum)
		SELECT t.login, t.number, $3 FROM t;
	`

	_, err := retry2(ctx, s.retryPolicy, func() (pgconn.CommandTag, error) {
		return s.conn.Exec(ctx, query, o.Status, o.Number, o.Accrual)
	})

	return err
}

func retry(ctx context.Context, rp retryPolicy, fn func() error) error {
	fnWithReturn := func() (struct{}, error) {
		return struct{}{}, fn()
	}

	_, err := retry2(ctx, rp, fnWithReturn)
	return err
}

func retry2[T any](ctx context.Context, rp retryPolicy, fn func() (T, error)) (T, error) {
	if val1, err := fn(); err == nil || !isonnectionException(err) {
		return val1, err
	}

	var err error
	var ret1 T
	duration := rp.duration
	for i := 0; i < rp.retryCount; i++ {
		select {
		case <-time.NewTimer(time.Duration(duration) * time.Second).C:
			ret1, err = fn()
			if err == nil || !isonnectionException(err) {
				return ret1, err
			}
		case <-ctx.Done():
			return ret1, err
		}

		duration += rp.increment
	}

	return ret1, err
}

func isonnectionException(err error) bool {
	var tError *pgconn.PgError
	if errors.As(err, &tError) && pgerrcode.IsConnectionException(tError.Code) {
		return true
	}

	return false
}
