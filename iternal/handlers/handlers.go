package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/Tomap-Tomap/go-loyalty-service/iternal/compresses"
	"github.com/Tomap-Tomap/go-loyalty-service/iternal/logger"
	"github.com/Tomap-Tomap/go-loyalty-service/iternal/luhnalg"
	"github.com/Tomap-Tomap/go-loyalty-service/iternal/models"
	"github.com/Tomap-Tomap/go-loyalty-service/iternal/storage"
	"github.com/Tomap-Tomap/go-loyalty-service/iternal/tokenworker"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type Repository interface {
	CreateUser(ctx context.Context, u models.User) error
	GetUser(ctx context.Context, login string) (*models.User, error)
	AddOrder(ctx context.Context, order string, login string) error
	GetOrders(ctx context.Context, login string) ([]models.Order, error)
	GetBalance(ctx context.Context, login string) (*models.UserBalance, error)
	DoWithdrawal(ctx context.Context, login string, ob models.OrderBalance) error
	GetWithdrawal(ctx context.Context, login string) ([]models.OrderBalance, error)
}

type Handlers struct {
	storage Repository
	tw      tokenworker.TokenWorker
}

func NewHandlers(storage Repository, tw tokenworker.TokenWorker) Handlers {
	return Handlers{storage: storage, tw: tw}
}

func (h *Handlers) register(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "text/plain; charset=utf-8")

	u, err := models.NewUserByRequestBody(r.Body)

	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	err = h.storage.CreateUser(r.Context(), *u)

	var tError *pgconn.PgError
	if errors.As(err, &tError) && tError.Code == pgerrcode.UniqueViolation {
		http.Error(w, "this login is busy", http.StatusConflict)
		return
	}

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	err = h.tw.WriteTokenInCookie(w, u.Login)

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (h *Handlers) login(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "text/plain; charset=utf-8")

	u, err := models.NewUserByRequestBody(r.Body)

	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	uDB, err := h.storage.GetUser(r.Context(), u.Login)

	if errors.Is(err, pgx.ErrNoRows) {
		http.Error(w, "invalid login or password", http.StatusUnauthorized)
		return
	}

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	err = uDB.CheckPassword(u.Password)

	if errors.Is(err, models.ErrPWDNotEqual) {
		http.Error(w, "invalid login or password", http.StatusUnauthorized)
		return
	}

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	err = h.tw.WriteTokenInCookie(w, u.Login)

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (h *Handlers) ordersPost(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "text/plain; charset=utf-8")

	id, err := luhnalg.GetNumberFromBody(r.Body)

	if errors.Is(err, luhnalg.ErrInvalidNumber) {
		http.Error(w, err.Error(), http.StatusUnprocessableEntity)
		return
	}

	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	login := r.Header.Get("login")

	err = h.storage.AddOrder(r.Context(), id, login)

	if errors.Is(err, storage.ErrIDExistForAnotherUsr) {
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}

	if errors.Is(err, storage.ErrIDExistForCurUsr) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(err.Error()))
		return
	}

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusAccepted)
}

func (h *Handlers) ordersGet(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "application/json; charset=utf-8")

	login := r.Header.Get("login")

	orders, err := h.storage.GetOrders(r.Context(), login)

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if len(orders) == 0 {
		http.Error(w, "", http.StatusNoContent)
		return
	}

	resp, err := json.MarshalIndent(orders, "", "    ")

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write(resp)
}

func (h *Handlers) balancesGet(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "application/json; charset=utf-8")

	login := r.Header.Get("login")

	orders, err := h.storage.GetBalance(r.Context(), login)

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	resp, err := json.MarshalIndent(orders, "", "    ")

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write(resp)
}

func (h *Handlers) withdrawal(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "text/plain; charset=utf-8")

	ob, err := models.NewOrderBalanceByRequestBody(r.Body)

	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	luhnCheck := luhnalg.CheckNumber([]byte(ob.Order))

	if !luhnCheck {
		http.Error(w, "invalid order", http.StatusUnprocessableEntity)
		return
	}

	login := r.Header.Get("login")
	err = h.storage.DoWithdrawal(r.Context(), login, *ob)

	if errors.Is(err, storage.ErrInsufficientFunds) {
		http.Error(w, err.Error(), http.StatusPaymentRequired)
		return
	}

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (h *Handlers) withdrawalGet(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "application/json; charset=utf-8")

	login := r.Header.Get("login")
	ob, err := h.storage.GetWithdrawal(r.Context(), login)

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if len(ob) == 0 {
		http.Error(w, "", http.StatusNoContent)
		return
	}

	resp, err := json.MarshalIndent(ob, "", "    ")

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write(resp)
}

type middleware func(http.Handler) http.Handler

func chooseHandler(mm map[string]http.Handler) http.Handler {
	logFn := func(w http.ResponseWriter, r *http.Request) {
		if v, ok := mm[r.Method]; ok {
			v.ServeHTTP(w, r)
			return
		}

		http.Error(w, "", http.StatusMethodNotAllowed)
	}

	return http.HandlerFunc(logFn)
}

func conveyor(mm map[string]http.Handler, middlewares ...middleware) http.Handler {
	h := chooseHandler(mm)
	for _, middleware := range middlewares {
		h = middleware(h)
	}
	return h
}

func ServiceMux(h Handlers) *http.ServeMux {
	mux := http.NewServeMux()

	mux.Handle("/api/user/register",
		conveyor(
			map[string]http.Handler{http.MethodPost: http.HandlerFunc(h.register)},
			compresses.CompressHandle,
			logger.RequestLogger),
	)
	mux.Handle("/api/user/login",
		conveyor(
			map[string]http.Handler{http.MethodPost: http.HandlerFunc(h.login)},
			compresses.CompressHandle,
			logger.RequestLogger),
	)
	mux.Handle("/api/user/orders",
		conveyor(
			map[string]http.Handler{
				http.MethodPost: http.HandlerFunc(h.ordersPost),
				http.MethodGet:  http.HandlerFunc(h.ordersGet),
			},
			h.tw.RequestToken,
			compresses.CompressHandle,
			logger.RequestLogger),
	)

	mux.Handle("/api/user/balance",
		conveyor(
			map[string]http.Handler{
				http.MethodGet: http.HandlerFunc(h.balancesGet),
			},
			h.tw.RequestToken,
			compresses.CompressHandle,
			logger.RequestLogger),
	)

	mux.Handle("/api/user/balance/withdraw",
		conveyor(
			map[string]http.Handler{
				http.MethodPost: http.HandlerFunc(h.withdrawal),
			},
			h.tw.RequestToken,
			compresses.CompressHandle,
			logger.RequestLogger),
	)

	mux.Handle("/api/user/withdrawals",
		conveyor(
			map[string]http.Handler{
				http.MethodGet: http.HandlerFunc(h.withdrawalGet),
			},
			h.tw.RequestToken,
			compresses.CompressHandle,
			logger.RequestLogger),
	)

	return mux
}
