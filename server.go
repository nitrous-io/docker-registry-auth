package main

import (
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/docker/distribution/registry/auth/token"
	"github.com/docker/libtrust"
	"github.com/gorilla/mux"
	"github.com/nitrous-io/docker-registry-auth/authentication"
	"github.com/nitrous-io/docker-registry-auth/authorization"
)

// Server is an auth server that conforms to the Docker Registry spec:
// https://github.com/docker/distribution/blob/master/docs/spec/auth/token.md
// It implements the http.Handler interface.
type Server struct {
	config        *Config
	router        *mux.Router
	authenticator authentication.Authenticator
	authorizer    authorization.Authorizer
}

type Config struct {
	Address       string `default:":5002"`
	DatabaseURL   string `envconfig:"database_url"`
	LogLevel      string `envconfig:"log_level",default:"debug"`
	LogColors     bool   `envconfig:"log_colors"`
	TokenCertPath string `envconfig:"token_cert_path"`
	TokenKeyPath  string `envconfig:"token_key_path"`

	token TokenConfig
	db    *sql.DB
}

type TokenConfig struct {
	publicKey  libtrust.PublicKey
	privateKey libtrust.PrivateKey
}

type AuthRequest struct {
	User              string
	Password          authentication.PasswordString
	authorizationInfo authorization.AuthorizationInfo
}

const (
	tokenIssuer = "Nitrous Registry Auth Service" // This should match auth.token.issuer in the registry config.
	tokenExpiry = 15 * time.Minute
)

func (r *AuthRequest) authorizationRequired() bool {
	return len(r.authorizationInfo.Actions) > 0
}

func NewServer(config *Config) *Server {
	svr := &Server{config: config}

	r := mux.NewRouter()
	r.HandleFunc("/", svr.indexHandler).Methods("GET")
	r.HandleFunc("/auth", svr.authHandler).Methods("GET")
	svr.router = r

	// TODO Allow these 2 to be configured so that we can replace them in tests.
	if config.db != nil {
		svr.authenticator = &authentication.NitrousAuthenticator{DB: config.db}
		svr.authorizer = &authorization.NitrousAuthorizer{DB: config.db}
	}

	return svr
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.router.ServeHTTP(w, r)
}

func (s *Server) indexHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(fmt.Sprintf("Konnichiwa docker client! Your user agent is %s.", r.UserAgent())))
}

func (s *Server) authHandler(w http.ResponseWriter, r *http.Request) {
	ar, err := s.parseRequest(r)
	if err != nil {
		msg := fmt.Sprintf("Bad auth request: %v", err)
		logrus.Info(msg)
		http.Error(w, msg, http.StatusBadRequest)
		return
	}
	logrus.Debugf("Auth request received: %+v", ar)

	authenticated, err := s.authenticator.Authenticate(ar.authorizationInfo.Account, authentication.PasswordString(ar.Password))
	if err != nil {
		msg := fmt.Sprintf("Authentication failed: %v", err)
		logrus.Info(msg)
		http.Error(w, msg, http.StatusInternalServerError)
		return
	}
	if !authenticated {
		msg := "Authentication failed"
		logrus.Info(msg)
		http.Error(w, msg, http.StatusUnauthorized)
		return
	}

	// If actions are present, we need to authorize them.
	authorizedActions := []string{}
	if ar.authorizationRequired() {
		authorizedActions, err = s.authorizer.Authorize(&ar.authorizationInfo)
		if err != nil {
			msg := fmt.Sprintf("Authorization failed: %v", err)
			logrus.Info(msg)
			http.Error(w, msg, http.StatusInternalServerError)
			return
		}
		logrus.Debugf("Authorized actions: %v", authorizedActions)
	}

	token, err := s.createToken(ar, authorizedActions)
	if err != nil {
		msg := fmt.Sprintf("Failed to generate token: %v", err)
		logrus.Info(msg)
		http.Error(w, msg, http.StatusInternalServerError)
		return
	}

	body, err := json.Marshal(map[string]string{"token": token})
	if err != nil {
		msg := fmt.Sprintf("Failed to encode token to JSON: %v", err)
		logrus.Info(msg)
		http.Error(w, msg, http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")
	logrus.Debugf("Responding with token %q", body)
	w.Write(body)
}

func (s *Server) parseRequest(r *http.Request) (*AuthRequest, error) {
	ar := &AuthRequest{}

	user, password, ok := r.BasicAuth()
	if ok {
		ar.User = user
		ar.Password = authentication.PasswordString(password)
	}

	account := r.FormValue("account")
	if account == "" {
		account = user
	}
	ar.authorizationInfo.Account = account

	ar.authorizationInfo.Service = r.FormValue("service")

	scope := r.FormValue("scope")
	if scope != "" {
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

// See https://github.com/docker/distribution/blob/master/docs/spec/auth/jwt.md.
func (s *Server) createToken(ar *AuthRequest, actions []string) (string, error) {
	// TODO Consider using github.com/dgrijalva/jwt-go.

	tokenPrivateKey := s.config.token.privateKey
	tokenPublicKey := s.config.token.publicKey

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
