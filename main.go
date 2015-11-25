package main

import (
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/facebookgo/httpdown"
	"github.com/gorilla/mux"
	"github.com/justinas/alice"
	"github.com/kelseyhightower/envconfig"
)

const envPrefix = "REGAUTH" // Env vars should be prefixed with "REGAUTH_".

func main() {
	var cfg Config
	if err := envconfig.Process(envPrefix, &cfg); err != nil {
		logrus.Fatalf("Error reading config: %v", err)
	}
	logrus.Infof("Using config: %#v", cfg)
	if lvl, err := logrus.ParseLevel(cfg.LogLevel); err == nil {
		logrus.SetLevel(lvl)
	}
	logrus.SetFormatter(&logrus.TextFormatter{DisableColors: !cfg.LogColors})

	r := mux.NewRouter()
	r.HandleFunc("/", indexHandler).Methods("GET")

	logHeaders := true
	logFormData := true
	chain := alice.New(logging(logHeaders, logFormData)).Then(r)

	svr := &http.Server{Addr: cfg.Address, Handler: chain}

	hd := &httpdown.HTTP{
		StopTimeout: 15 * time.Minute,
		KillTimeout: 1 * time.Minute,
	}
	logrus.Infof("Starting server at %s (pid: %d)", svr.Addr, os.Getpid())
	hsvr, err := hd.ListenAndServe(svr)
	if err != nil {
		logrus.Fatalf("Error starting server, err: %v", err)
	}

	waitErr := make(chan error, 1)
	go func() {
		waitErr <- hsvr.Wait()
		close(waitErr)
	}()

	signals := make(chan os.Signal)
	signal.Notify(signals, syscall.SIGTERM, syscall.SIGINT)

Loop:
	for {
		select {
		case err := <-waitErr:
			if err != nil {
				logrus.Fatalf("Failed to wait for HTTP serve loop to finish, err: %v", err)
			}
			break Loop
		case sig := <-signals:
			logrus.Infof("Signal received: %s, stopping server", sig)
			signal.Stop(signals)
			if err := hsvr.Stop(); err != nil {
				logrus.Fatalf("Error stopping server, err: %v", err)
			}
			break Loop
		}
	}

	logrus.Println("Server stopped")
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(fmt.Sprintf("Konnichiwa docker client! Your user agent is %s.", r.UserAgent())))
}

type Config struct {
	Address     string `default:":5002"`
	DatabaseURL string `envconfig:"database_url"`
	LogLevel    string `envconfig:"log_level",default:"debug"`
	LogColors   bool   `envconfig:"log_colors"`
}
