package parameters

import (
	"flag"
	"os"
	"strconv"
	"time"
)

type Parameters struct {
	RunAddr           string
	DataBaseURI       string
	AccrualSystemAddr string
	SecretKey         string
	SecetKeyLife      time.Duration
	GetInterval       uint
	WorkerLimit       uint
}

func ParseFlags() (p Parameters) {
	f := flag.NewFlagSet(os.Args[0], flag.ContinueOnError)
	f.StringVar(&p.RunAddr, "a", "localhost:8081", "address and port to run server")
	f.StringVar(&p.DataBaseURI,
		"d",
		"host=localhost user=test password=test dbname=loyaltyservice sslmode=disable",
		"connection string to database")
	f.StringVar(&p.AccrualSystemAddr, "r", "http://localhost:8080", "address and port to accrual system")
	f.StringVar(&p.SecretKey, "k", "secret", "secret key for jwt")

	var skLife uint
	f.UintVar(&skLife, "kl", 3, "secret key life in hours")

	f.UintVar(&p.GetInterval, "gi", 5, "interval for communicate to accrual system")
	f.UintVar(&p.WorkerLimit, "wl", 5, "worker limit for communicate to accrual system")
	f.Parse(os.Args[1:])

	p.SecetKeyLife = time.Hour * time.Duration(skLife)

	if envAddr := os.Getenv("RUN_ADDRESS"); envAddr != "" {
		p.RunAddr = envAddr
	}

	if envDB := os.Getenv("DATABASE_URI"); envDB != "" {
		p.DataBaseURI = envDB
	}

	if envAS := os.Getenv("ACCRUAL_SYSTEM_ADDRESS"); envAS != "" {
		p.AccrualSystemAddr = envAS
	}

	if envSK := os.Getenv("SECRET_KEY"); envSK != "" {
		p.SecretKey = envSK
	}

	if envSKL := os.Getenv("SECRET_KEY_LIFE"); envSKL != "" {
		intSKL, err := strconv.ParseUint(envSKL, 10, 32)

		if err == nil {
			p.SecetKeyLife = time.Hour * time.Duration(intSKL)
		}
	}

	if envGI := os.Getenv("GET_INTERVAL"); envGI != "" {
		intGI, err := strconv.ParseUint(envGI, 10, 32)

		if err == nil {
			p.GetInterval = uint(intGI)
		}
	}

	if envWL := os.Getenv("WORKER_LIMIT"); envWL != "" {
		intWL, err := strconv.ParseUint(envWL, 10, 32)

		if err == nil {
			p.WorkerLimit = uint(intWL)
		}
	}

	return
}
