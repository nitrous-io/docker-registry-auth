package main

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/docker/distribution/registry/auth/token"
	"github.com/docker/libtrust"
	"github.com/facebookgo/httpdown"
	"github.com/gorilla/mux"
	"github.com/justinas/alice"
	"github.com/kelseyhightower/envconfig"
)

const (
	envPrefix   = "REGAUTH"                       // Env vars should be prefixed with "REGAUTH_".
	tokenIssuer = "Nitrous Registry Auth Service" // This should match auth.token.issuer in the registry config.
	tokenExpiry = 15 * time.Minute
)

var (
	tokenPublicKey  libtrust.PublicKey
	tokenPrivateKey libtrust.PrivateKey
)

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
	tokenPublicKey, err = libtrust.FromCryptoPublicKey(x509cert.PublicKey)
	if err != nil {
		logrus.Fatalf("Error getting token public key, err: %v", err)
	}
	tokenPrivateKey, err = libtrust.FromCryptoPrivateKey(cert.PrivateKey)
	if err != nil {
		logrus.Fatalf("Error getting token private key, err: %v", err)
	}

	r := mux.NewRouter()
	r.HandleFunc("/", indexHandler).Methods("GET")
	r.HandleFunc("/auth", authHandler).Methods("GET")

	// TODO Add an X-Request-ID header and log it.
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

func authHandler(w http.ResponseWriter, r *http.Request) {
	ar, err := parseRequest(r)
	if err != nil {
		http.Error(w, fmt.Sprintf("Bad request: %v", err), http.StatusBadRequest)
		return
	}
	logrus.Warnf("got auth request: %#v", ar)

	authenticator := &NitrousUserAuthenticator{} // FIXME This should be a shared instance.
	authenticated, err := authenticator.Authenticate(ar.authorizationInfo.Account, ar.Password)
	if err != nil {
		http.Error(w, fmt.Sprintf("Authentication failed: %v", err), http.StatusInternalServerError)
		return
	}
	if !authenticated {
		http.Error(w, fmt.Sprintf("Authentication failed"), http.StatusUnauthorized)
		return
	}

	// If actions are present, we need to authorize them.
	authorizedActions := []string{}
	if ar.authorizationRequired() {
		authorizer := &NitrousAuthorizer{} // FIXME This should be a shared instance.

		authorizedActions, err = authorizer.Authorize(&ar.authorizationInfo)
		if err != nil {
			http.Error(w, fmt.Sprintf("Authorization failed: %v", err), http.StatusInternalServerError)
			return
		}
	}

	token, err := createToken(ar, authorizedActions)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to generate token: %v", err), http.StatusInternalServerError)
		return
	}

	body, err := json.Marshal(map[string]string{"token": token})
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to encode token to JSON: %v", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")
	logrus.Debugf("Responding with token %q", body)
	w.Write(body)
}

type AuthRequest struct {
	User              string
	Password          string
	authorizationInfo AuthorizationInfo
}

func (r *AuthRequest) authorizationRequired() bool {
	return len(r.authorizationInfo.Actions) > 0
}

func parseRequest(r *http.Request) (*AuthRequest, error) {
	ar := &AuthRequest{}

	user, password, ok := r.BasicAuth()
	if ok {
		ar.User = user
		ar.Password = password
	}

	account := r.FormValue("account")
	if account == "" {
		account = user
	}
	ar.authorizationInfo.Account = account

	ar.authorizationInfo.Service = r.FormValue("service")

	scope := r.FormValue("scope")
	if scope != "" {
		logrus.Warnf("scope present: %q", scope)
		parts := strings.Split(scope, ":")
		if len(parts) != 3 {
			return nil, fmt.Errorf("invalid scope: %q", scope)
		}

		ar.authorizationInfo.Type = parts[0]
		ar.authorizationInfo.Name = parts[1]
		ar.authorizationInfo.Actions = strings.Split(parts[2], ",")
		sort.Strings(ar.authorizationInfo.Actions)
	}

	return ar, nil
}

func createToken(ar *AuthRequest, actions []string) (string, error) {
	// TODO Consider using github.com/dgrijalva/jwt-go.

	// FIXME This is really dumb.
	// Sign something dummy to find out which signing algorithm is used.
	_, alg, err := tokenPrivateKey.Sign(strings.NewReader("dummy"), 0)
	if err != nil {
		return "", fmt.Errorf("failed to sign, err: %s", err)
	}

	header := token.Header{
		Type:       "JWT",
		SigningAlg: alg,
		KeyID:      tokenPublicKey.KeyID(),
	}
	headerJSON, err := json.Marshal(header)
	if err != nil {
		return "", fmt.Errorf("failed to marshal token header, err: %v", err)
	}

	now := time.Now().Unix()
	claims := token.ClaimSet{
		Issuer:     tokenIssuer,
		Subject:    ar.authorizationInfo.Account,
		Audience:   ar.authorizationInfo.Service,
		Expiration: now + int64(tokenExpiry/time.Second),
		NotBefore:  now,
		IssuedAt:   now,
		JWTID:      fmt.Sprintf("%d", rand.Int63()),
		Access:     []*token.ResourceActions{},
	}
	if len(actions) > 0 {
		claims.Access = []*token.ResourceActions{
			{
				Type:    ar.authorizationInfo.Type,
				Name:    ar.authorizationInfo.Name,
				Actions: actions,
			},
		}
	}
	claimsJSON, err := json.Marshal(claims)
	if err != nil {
		return "", fmt.Errorf("failed to marshal token claim set, err: %v", err)
	}

	payload := strings.Join([]string{
		joseBase64UrlEncode(headerJSON),
		joseBase64UrlEncode(claimsJSON),
	}, token.TokenSeparator)

	// Sign the payload.
	signed, _, err := tokenPrivateKey.Sign(strings.NewReader(payload), 0)
	if err != nil {
		return "", fmt.Errorf("failed to sign token, err: %v", err)
	}

	tk := strings.Join([]string{payload, joseBase64UrlEncode(signed)}, token.TokenSeparator)

	return tk, nil
}

func joseBase64UrlEncode(b []byte) string {
	return strings.TrimRight(base64.URLEncoding.EncodeToString(b), "=")
}

type Config struct {
	Address       string `default:":5002"`
	DatabaseURL   string `envconfig:"database_url"`
	LogLevel      string `envconfig:"log_level",default:"debug"`
	LogColors     bool   `envconfig:"log_colors"`
	TokenCertPath string `envconfig:"token_cert_path"`
	TokenKeyPath  string `envconfig:"token_key_path"`
}
