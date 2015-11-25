package main

import (
	"crypto/tls"
	"crypto/x509"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/docker/libtrust"
	"github.com/facebookgo/httpdown"
	"github.com/justinas/alice"
	"github.com/kelseyhightower/envconfig"
)

const envPrefix = "REGAUTH" // Env vars should be prefixed with "REGAUTH_".

func main() {
	var cfg Config
	if err := envconfig.Process(envPrefix, &cfg); err != nil {
		logrus.Fatalf("Error reading config, err: %v", err)
	}
	logrus.Infof("Using config: %#v", cfg)
	if lvl, err := logrus.ParseLevel(cfg.LogLevel); err == nil {
		logrus.SetLevel(lvl)
	}
	logrus.SetFormatter(&logrus.TextFormatter{DisableColors: !cfg.LogColors})

	// Load certificate and key used for signing token.
	cert, err := tls.LoadX509KeyPair(cfg.TokenCertPath, cfg.TokenKeyPath)
	if err != nil {
		logrus.Fatalf("Error loading token cert and/or key, err: %v", err)
	}
	x509cert, err := x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		logrus.Fatalf("Error parsing token public key key, err: %v", err)
	}
	tokenPublicKey, err := libtrust.FromCryptoPublicKey(x509cert.PublicKey)
	if err != nil {
		logrus.Fatalf("Error getting token public key, err: %v", err)
	}
	cfg.token.publicKey = tokenPublicKey
	tokenPrivateKey, err := libtrust.FromCryptoPrivateKey(cert.PrivateKey)
	if err != nil {
		logrus.Fatalf("Error getting token private key, err: %v", err)
	}
	cfg.token.privateKey = tokenPrivateKey

	svr := NewServer(&cfg)

	// TODO Add an X-Request-ID header and log it.
	logHeaders := true
	logFormData := true
	chain := alice.New(logging(logHeaders, logFormData)).Then(svr)

	httpSvr := &http.Server{Addr: cfg.Address, Handler: chain}
	hd := &httpdown.HTTP{
		StopTimeout: 15 * time.Minute,
		KillTimeout: 1 * time.Minute,
	}
	logrus.Infof("Starting server at %s (pid: %d)", httpSvr.Addr, os.Getpid())
	hsvr, err := hd.ListenAndServe(httpSvr)
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
