package handlers

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Tomap-Tomap/go-loyalty-service/iternal/hasher"
	"github.com/Tomap-Tomap/go-loyalty-service/iternal/models"
	"github.com/Tomap-Tomap/go-loyalty-service/iternal/storage"
	"github.com/Tomap-Tomap/go-loyalty-service/iternal/tokenworker"
	"github.com/go-resty/resty/v2"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type RepositoryMockedObject struct {
	mock.Mock
}

func (rm *RepositoryMockedObject) CreateUser(ctx context.Context, u models.User) error {
	args := rm.Called(u)

	return args.Error(0)
}

func (rm *RepositoryMockedObject) GetUser(ctx context.Context, login string) (*models.User, error) {
	args := rm.Called(login)

	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.User), args.Error(1)
}

func (rm *RepositoryMockedObject) AddOrder(ctx context.Context, order string, login string) error {
	args := rm.Called(order, login)

	return args.Error(0)
}

func (rm *RepositoryMockedObject) GetOrders(ctx context.Context, login string) ([]models.Order, error) {
	args := rm.Called(login)

	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.Order), args.Error(1)
}

func (rm *RepositoryMockedObject) GetBalance(ctx context.Context, login string) (*models.UserBalance, error) {
	args := rm.Called(login)

	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.UserBalance), args.Error(1)
}

func (rm *RepositoryMockedObject) DoWithdrawal(ctx context.Context, login string, ob models.OrderBalance) error {
	args := rm.Called(login, ob)

	return args.Error(0)
}

func (rm *RepositoryMockedObject) GetWithdrawal(ctx context.Context, login string) ([]models.OrderBalance, error) {
	args := rm.Called(login)

	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.OrderBalance), args.Error(1)
}

func testRequest(t *testing.T, srv *httptest.Server, method, url string, body string, token string) *resty.Response {
	req := resty.New().R()
	req.SetCookie(&http.Cookie{
		Name:  "token",
		Value: token,
	})
	req.Method = method
	req.URL = srv.URL + url
	req.SetBody(body)
	res, err := req.Send()
	require.NoError(t, err)

	return res
}

func TestHandlers_register(t *testing.T) {
	rm := new(RepositoryMockedObject)
	uniqErrUsr := models.User{Login: "uniqErr", Password: "uniqErr"}
	errUsr := models.User{Login: "err", Password: "err"}
	usr := models.User{Login: "usr", Password: "usr"}
	rm.On("CreateUser", uniqErrUsr).Return(&pgconn.PgError{Code: pgerrcode.UniqueViolation})
	rm.On("CreateUser", errUsr).Return(fmt.Errorf("test error"))
	rm.On("CreateUser", usr).Return(nil)
	h := NewHandlers(rm, *tokenworker.NewToken("secret", 3*time.Hour))
	mux := ServiceMux(h)

	srv := httptest.NewServer(mux)
	defer srv.Close()

	t.Run("test bad request", func(t *testing.T) {
		res := testRequest(t, srv, http.MethodPost, "/api/user/register", "", "")
		require.Equal(t, "text/plain; charset=utf-8", res.Header().Get("Content-Type"))
		require.Equal(t, http.StatusBadRequest, res.StatusCode())
	})

	t.Run("test 409", func(t *testing.T) {
		res := testRequest(t, srv, http.MethodPost, "/api/user/register", `{
			"login": "uniqErr",
			"password": "uniqErr"
		} `, "")
		require.Equal(t, "text/plain; charset=utf-8", res.Header().Get("Content-Type"))
		require.Equal(t, http.StatusConflict, res.StatusCode())
	})

	t.Run("test 500", func(t *testing.T) {
		res := testRequest(t, srv, http.MethodPost, "/api/user/register", `{
			"login": "err",
			"password": "err"
		} `, "")
		require.Equal(t, "text/plain; charset=utf-8", res.Header().Get("Content-Type"))
		require.Equal(t, http.StatusInternalServerError, res.StatusCode())
	})

	t.Run("test 200", func(t *testing.T) {
		res := testRequest(t, srv, http.MethodPost, "/api/user/register", `{
			"login": "usr",
			"password": "usr"
		} `, "")
		require.Equal(t, "text/plain; charset=utf-8", res.Header().Get("Content-Type"))
		require.Equal(t, http.StatusOK, res.StatusCode())
	})

	rm.AssertExpectations(t)
}

func TestHandlers_login(t *testing.T) {
	sp, err := hasher.NewSaltPassword("pwd")
	usr := &models.User{Login: "login", Password: sp.Password, Salt: sp.Salt}
	require.NoError(t, err)

	rm := new(RepositoryMockedObject)
	rm.On("GetUser", "noRow").Return(nil, pgx.ErrNoRows)
	rm.On("GetUser", "internalErr").Return(nil, fmt.Errorf("test error"))
	rm.On("GetUser", "login").Return(usr, nil)
	h := NewHandlers(rm, *tokenworker.NewToken("secret", 3*time.Hour))
	mux := ServiceMux(h)

	srv := httptest.NewServer(mux)
	defer srv.Close()

	t.Run("test bad request", func(t *testing.T) {
		res := testRequest(t, srv, http.MethodPost, "/api/user/login", "", "")
		require.Equal(t, "text/plain; charset=utf-8", res.Header().Get("Content-Type"))
		require.Equal(t, http.StatusBadRequest, res.StatusCode())
	})

	t.Run("test 401", func(t *testing.T) {
		res := testRequest(t, srv, http.MethodPost, "/api/user/login", `{
			"login": "noRow",
			"password": "noRow"
		} `, "")
		require.Equal(t, "text/plain; charset=utf-8", res.Header().Get("Content-Type"))
		require.Equal(t, http.StatusUnauthorized, res.StatusCode())
	})

	t.Run("test 500", func(t *testing.T) {
		res := testRequest(t, srv, http.MethodPost, "/api/user/login", `{
			"login": "internalErr",
			"password": "internalErr"
		} `, "")
		require.Equal(t, "text/plain; charset=utf-8", res.Header().Get("Content-Type"))
		require.Equal(t, http.StatusInternalServerError, res.StatusCode())
	})

	t.Run("test invalid pwd", func(t *testing.T) {
		res := testRequest(t, srv, http.MethodPost, "/api/user/login", `{
			"login": "login",
			"password": "checkPwdErr"
		} `, "")
		require.Equal(t, "text/plain; charset=utf-8", res.Header().Get("Content-Type"))
		require.Equal(t, http.StatusUnauthorized, res.StatusCode())
	})

	t.Run("test OK", func(t *testing.T) {
		res := testRequest(t, srv, http.MethodPost, "/api/user/login", `{
			"login": "login",
			"password": "pwd"
		} `, "")
		require.Equal(t, "text/plain; charset=utf-8", res.Header().Get("Content-Type"))
		require.Equal(t, http.StatusOK, res.StatusCode())
	})

	rm.AssertExpectations(t)
}

func TestHandlers_ordersPost(t *testing.T) {
	numberExistAU := "9278923470"
	numberExistCU := "12345678903"
	numberErr := "346436439"
	numberOK := "2377225624"
	rm := new(RepositoryMockedObject)
	rm.On("AddOrder", numberExistAU, "test").Return(storage.ErrIDExistForAnotherUsr)
	rm.On("AddOrder", numberExistCU, "test").Return(storage.ErrIDExistForCurUsr)
	rm.On("AddOrder", numberErr, "test").Return(fmt.Errorf("test"))
	rm.On("AddOrder", numberOK, "test").Return(nil)
	h := NewHandlers(rm, *tokenworker.NewToken("secret", 3*time.Hour))
	tokenString, err := h.tw.GetToken("test")
	require.NoError(t, err)
	mux := ServiceMux(h)

	srv := httptest.NewServer(mux)
	defer srv.Close()

	t.Run("test 422", func(t *testing.T) {
		res := testRequest(t, srv, http.MethodPost, "/api/user/orders", "123", tokenString)
		require.Equal(t, "text/plain; charset=utf-8", res.Header().Get("Content-Type"))
		require.Equal(t, http.StatusUnprocessableEntity, res.StatusCode())
	})

	t.Run("test 400", func(t *testing.T) {
		res := testRequest(t, srv, http.MethodPost, "/api/user/orders", "", tokenString)
		require.Equal(t, "text/plain; charset=utf-8", res.Header().Get("Content-Type"))
		require.Equal(t, http.StatusBadRequest, res.StatusCode())
	})

	t.Run("test 409", func(t *testing.T) {
		res := testRequest(t, srv, http.MethodPost, "/api/user/orders", numberExistAU, tokenString)
		require.Equal(t, "text/plain; charset=utf-8", res.Header().Get("Content-Type"))
		require.Equal(t, http.StatusConflict, res.StatusCode())
	})

	t.Run("test 200", func(t *testing.T) {
		res := testRequest(t, srv, http.MethodPost, "/api/user/orders", numberExistCU, tokenString)
		require.Equal(t, "text/plain; charset=utf-8", res.Header().Get("Content-Type"))
		require.Equal(t, http.StatusOK, res.StatusCode())
	})

	t.Run("test 500", func(t *testing.T) {
		res := testRequest(t, srv, http.MethodPost, "/api/user/orders", numberErr, tokenString)
		require.Equal(t, "text/plain; charset=utf-8", res.Header().Get("Content-Type"))
		require.Equal(t, http.StatusInternalServerError, res.StatusCode())
	})

	t.Run("test 202", func(t *testing.T) {
		res := testRequest(t, srv, http.MethodPost, "/api/user/orders", numberOK, tokenString)
		require.Equal(t, "text/plain; charset=utf-8", res.Header().Get("Content-Type"))
		require.Equal(t, http.StatusAccepted, res.StatusCode())
	})

	rm.AssertExpectations(t)
}

func TestHandlers_ordersGet(t *testing.T) {
	rm := new(RepositoryMockedObject)
	rm.On("GetOrders", "ISR").Return(nil, fmt.Errorf("test"))
	rm.On("GetOrders", "NoContent").Return(make([]models.Order, 0), nil)
	curTime := time.Now()
	rm.On("GetOrders", "OK").Return([]models.Order{{Number: "1", Status: "OK", UploadedAt: &curTime}}, nil)
	h := NewHandlers(rm, *tokenworker.NewToken("secret", 3*time.Hour))
	tokenISR, err := h.tw.GetToken("ISR")
	require.NoError(t, err)
	tokenNC, err := h.tw.GetToken("NoContent")
	require.NoError(t, err)
	tokenOK, err := h.tw.GetToken("OK")
	require.NoError(t, err)
	mux := ServiceMux(h)

	srv := httptest.NewServer(mux)
	defer srv.Close()

	t.Run("test 500", func(t *testing.T) {
		res := testRequest(t, srv, http.MethodGet, "/api/user/orders", "", tokenISR)
		require.Equal(t, "text/plain; charset=utf-8", res.Header().Get("Content-Type"))
		require.Equal(t, http.StatusInternalServerError, res.StatusCode())
	})

	t.Run("test 204", func(t *testing.T) {
		res := testRequest(t, srv, http.MethodGet, "/api/user/orders", "", tokenNC)
		require.Equal(t, "text/plain; charset=utf-8", res.Header().Get("Content-Type"))
		require.Equal(t, http.StatusNoContent, res.StatusCode())
	})

	t.Run("test 200", func(t *testing.T) {
		res := testRequest(t, srv, http.MethodGet, "/api/user/orders", "", tokenOK)
		require.Equal(t, "application/json; charset=utf-8", res.Header().Get("Content-Type"))
		require.Equal(t, http.StatusOK, res.StatusCode())
		exJSON := fmt.Sprintf(`[{
			"number":"1",
			"status":"OK",
			"uploaded_at": "%s"
		}]`, curTime.Format(time.RFC3339))
		acJSON := string(res.Body())
		require.JSONEq(t, exJSON, acJSON)
	})

	rm.AssertExpectations(t)
}

func TestHandlers_balancesGet(t *testing.T) {
	rm := new(RepositoryMockedObject)
	rm.On("GetBalance", "ISR").Return(nil, fmt.Errorf("test"))
	wd := float64(-500)
	rm.On("GetBalance", "OK").Return(&models.UserBalance{Current: 500, Withdrawn: &wd}, nil)
	h := NewHandlers(rm, *tokenworker.NewToken("secret", 3*time.Hour))
	tokenISR, err := h.tw.GetToken("ISR")
	require.NoError(t, err)
	tokenOK, err := h.tw.GetToken("OK")
	require.NoError(t, err)
	mux := ServiceMux(h)

	srv := httptest.NewServer(mux)
	defer srv.Close()

	t.Run("test 500", func(t *testing.T) {
		res := testRequest(t, srv, http.MethodGet, "/api/user/balance", "", tokenISR)
		require.Equal(t, "text/plain; charset=utf-8", res.Header().Get("Content-Type"))
		require.Equal(t, http.StatusInternalServerError, res.StatusCode())
	})

	t.Run("test 200", func(t *testing.T) {
		res := testRequest(t, srv, http.MethodGet, "/api/user/balance", "", tokenOK)
		require.Equal(t, "application/json; charset=utf-8", res.Header().Get("Content-Type"))
		require.Equal(t, http.StatusOK, res.StatusCode())
		exJSON := `{
			"current": 500,
     		"withdrawn": -500
		}`
		acJSON := string(res.Body())
		require.JSONEq(t, exJSON, acJSON)
	})

	rm.AssertExpectations(t)
}

func TestHandlers_withdrawal(t *testing.T) {
	rm := new(RepositoryMockedObject)
	rm.On("DoWithdrawal", "test", models.OrderBalance{Order: "2377225624", Sum: 123}).Return(storage.ErrInsufficientFunds)
	rm.On("DoWithdrawal", "test", models.OrderBalance{Order: "2377225624", Sum: 0}).Return(fmt.Errorf("test"))
	rm.On("DoWithdrawal", "test", models.OrderBalance{Order: "2377225624", Sum: 100}).Return(nil)

	h := NewHandlers(rm, *tokenworker.NewToken("secret", 3*time.Hour))
	tokenString, err := h.tw.GetToken("test")
	require.NoError(t, err)
	mux := ServiceMux(h)

	srv := httptest.NewServer(mux)
	defer srv.Close()

	t.Run("test 400", func(t *testing.T) {
		res := testRequest(t, srv, http.MethodPost, "/api/user/balance/withdraw", "", tokenString)
		require.Equal(t, "text/plain; charset=utf-8", res.Header().Get("Content-Type"))
		require.Equal(t, http.StatusBadRequest, res.StatusCode())
	})

	t.Run("test 402", func(t *testing.T) {
		res := testRequest(t, srv, http.MethodPost, "/api/user/balance/withdraw", `{"order":"2377225624", "sum": 123}`, tokenString)
		require.Equal(t, "text/plain; charset=utf-8", res.Header().Get("Content-Type"))
		require.Equal(t, http.StatusPaymentRequired, res.StatusCode())
	})

	t.Run("test 500", func(t *testing.T) {
		res := testRequest(t, srv, http.MethodPost, "/api/user/balance/withdraw", `{"order":"2377225624", "sum": 0}`, tokenString)
		require.Equal(t, "text/plain; charset=utf-8", res.Header().Get("Content-Type"))
		require.Equal(t, http.StatusInternalServerError, res.StatusCode())
	})

	t.Run("test 200", func(t *testing.T) {
		res := testRequest(t, srv, http.MethodPost, "/api/user/balance/withdraw", `{"order":"2377225624", "sum": 100}`, tokenString)
		require.Equal(t, "text/plain; charset=utf-8", res.Header().Get("Content-Type"))
		require.Equal(t, http.StatusOK, res.StatusCode())
	})

	rm.AssertExpectations(t)
}

func TestHandlers_withdrawalGet(t *testing.T) {
	rm := new(RepositoryMockedObject)
	rm.On("GetWithdrawal", "ISR").Return(nil, fmt.Errorf("test"))
	rm.On("GetWithdrawal", "NoContent").Return(make([]models.OrderBalance, 0), nil)
	curTime := time.Now()
	rm.On("GetWithdrawal", "OK").Return([]models.OrderBalance{{Order: "123", Sum: 500, ProcessedAt: &curTime}}, nil)
	h := NewHandlers(rm, *tokenworker.NewToken("secret", 3*time.Hour))
	tokenISR, err := h.tw.GetToken("ISR")
	require.NoError(t, err)
	tokenNC, err := h.tw.GetToken("NoContent")
	require.NoError(t, err)
	tokenOK, err := h.tw.GetToken("OK")
	require.NoError(t, err)
	mux := ServiceMux(h)

	srv := httptest.NewServer(mux)
	defer srv.Close()

	t.Run("test 500", func(t *testing.T) {
		res := testRequest(t, srv, http.MethodGet, "/api/user/withdrawals", "", tokenISR)
		require.Equal(t, "text/plain; charset=utf-8", res.Header().Get("Content-Type"))
		require.Equal(t, http.StatusInternalServerError, res.StatusCode())
	})

	t.Run("test 204", func(t *testing.T) {
		res := testRequest(t, srv, http.MethodGet, "/api/user/withdrawals", "", tokenNC)
		require.Equal(t, "text/plain; charset=utf-8", res.Header().Get("Content-Type"))
		require.Equal(t, http.StatusNoContent, res.StatusCode())
	})

	t.Run("test 200", func(t *testing.T) {
		res := testRequest(t, srv, http.MethodGet, "/api/user/withdrawals", "", tokenOK)
		require.Equal(t, "application/json; charset=utf-8", res.Header().Get("Content-Type"))
		require.Equal(t, http.StatusOK, res.StatusCode())
		exJSON := fmt.Sprintf(`[{
			"order":"123",
			"sum":500,
			"processed_at": "%s"
		}]`, curTime.Format(time.RFC3339))
		acJSON := string(res.Body())
		require.JSONEq(t, exJSON, acJSON)
	})

	rm.AssertExpectations(t)
}
