package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"

	"github.com/Tomap-Tomap/go-loyalty-service/iternal/agent"
	"github.com/Tomap-Tomap/go-loyalty-service/iternal/client"
	"github.com/Tomap-Tomap/go-loyalty-service/iternal/handlers"
	"github.com/Tomap-Tomap/go-loyalty-service/iternal/logger"
	"github.com/Tomap-Tomap/go-loyalty-service/iternal/parameters"
	"github.com/Tomap-Tomap/go-loyalty-service/iternal/storage"
	"github.com/Tomap-Tomap/go-loyalty-service/iternal/tokenworker"
	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

func main() {
	p := parameters.ParseFlags()

	if err := logger.Initialize("INFO", "stderr"); err != nil {
		panic(err)
	}

	logger.Log.Info("Create database storage")

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, os.Kill)
	defer cancel()
	eg, egCtx := errgroup.WithContext(ctx)

	conn, err := pgx.Connect(ctx, p.DataBaseURI)

	if err != nil {
		logger.Log.Fatal("Connect to database", zap.Error(err))
	}
	defer conn.Close(ctx)

	storage, err := storage.NewStorage(conn)

	if err != nil {
		logger.Log.Fatal("Create storage", zap.Error(err))
	}

	logger.Log.Info("Create token worker")
	tw := tokenworker.NewToken(p.SecretKey, p.SecetKeyLife)
	logger.Log.Info("Create handlers")
	h := handlers.NewHandlers(storage, *tw)
	logger.Log.Info("Create mux")
	mux := handlers.ServiceMux(h)
	logger.Log.Info("Create client")
	c := client.NewClient(p.AccrualSystemAddr)
	logger.Log.Info("Create agent")
	a := agent.NewAgent(storage, c, p.GetInterval, p.WorkerLimit)

	httpServer := &http.Server{
		Addr:    p.RunAddr,
		Handler: mux,
	}
	eg.Go(func() error {
		logger.Log.Info("Run server")
		err := httpServer.ListenAndServe()
		if err != nil && err != http.ErrServerClosed {
			return err
		}

		return nil
	})

	eg.Go(func() error {
		<-egCtx.Done()
		logger.Log.Info("Stor serve")
		return httpServer.Shutdown(context.Background())
	})

	eg.Go(func() error {
		logger.Log.Info("Run agent")
		if err := a.Run(egCtx); err != nil {
			return err
		}

		return nil
	})

	if err := eg.Wait(); err != nil {
		logger.Log.Fatal("Problem with working server", zap.Error(err))
	}
}
