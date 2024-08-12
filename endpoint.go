package main

import (
	"errors"
	"fmt"
	"github.com/dgrijalva/jwt-go"
	"golang.org/x/time/rate"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httputil"
	"path"
	"time"
)

type Endpoint struct {
	Path        string `json:"path"`
	Method      string `json:"method"`
	Description string `json:"description"`
}

type ServiceEndpoints map[string][]Endpoint

type Target struct {
	BaseURL   string           `json:"baseUrl"`
	Service   string           `json:"service"`
	Endpoints ServiceEndpoints `json:"endpoints"`
}

type ConfigModel struct {
	Targets []Target `json:"targets"`
}

type authRoundTripper struct {
	next http.RoundTripper
}

type loggingRoundTripper struct {
	next http.RoundTripper
}

type retryRoundTripper struct {
	next       http.RoundTripper
	maxRetries int
	delay      time.Duration
}

type rateLimitRoundTripper struct {
	next    http.RoundTripper
	limiter *rate.Limiter
}

func newRateLimitRoundTripper(next http.RoundTripper, rps, burst int) *rateLimitRoundTripper {
	limiter := rate.NewLimiter(rate.Limit(rps), burst)
	return &rateLimitRoundTripper{next: next, limiter: limiter}
}

func (rl *rateLimitRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) {
	if err := rl.limiter.Wait(r.Context()); err != nil {
		return nil, err
	}
	return rl.next.RoundTrip(r)
}

func (a *authRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) {
	token := r.Header.Get("Authorization")

	if token == "" || !ValidateJWT(token) {
		fmt.Println("Token invalid dönüyor")
		return nil, errors.New("invalid token")
	}
	fmt.Println("Token valid")
	return a.next.RoundTrip(r)
}

func ValidateJWT(t string) bool {
	jwtSecret := []byte("b0272461f7855e2f088cf50221886bb7e894569baf143144f28b81119c5ba809")

	token, err := jwt.Parse(t, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("Unexpected signing method: %v", token.Header["alg"])
		}
		return jwtSecret, nil
	})
	if err != nil || !token.Valid {
		return false
	}

	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		if exp, ok := claims["exp"].(float64); ok {
			if time.Unix(int64(exp), 0).Before(time.Now()) {
				return false
			}
		}
	}
	return true
}

func (rr *retryRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) {
	var attempts int
	for {
		res, err := rr.next.RoundTrip(r)
		attempts++

		if attempts == rr.maxRetries {
			return res, err
		}

		if err == nil && res.StatusCode < http.StatusInternalServerError {
			return res, err
		}

		select {
		case <-r.Context().Done():
			return res, r.Context().Err()
		case <-time.After(rr.delay):
		}
	}
}

func (l *loggingRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) {
	dumpReq, err := httputil.DumpRequest(r, true)
	if err != nil {
		fmt.Println("Request dump error:", err)
	}
	resp, err := l.next.RoundTrip(r)

	if err != nil {
		go L.Log(NewLog(time.Now(), "user", err.Error(), r.URL.String(), r.RemoteAddr, string(dumpReq[:]), "", 0))
		return nil, err
	}

	dumpRes, err := httputil.DumpResponse(resp, true)
	if err != nil {
		fmt.Println("Response dump error:", err)
	}

	go L.Log(NewLog(time.Now(), "user", "", r.URL.String(), r.RemoteAddr, string(dumpReq[:]), string(dumpRes[:]), resp.StatusCode))
	return resp, err
}

func Router() *http.ServeMux {
	mux := http.NewServeMux()

	jar, _ := cookiejar.New(nil)
	client := &http.Client{
		Transport: newRateLimitRoundTripper(
			&authRoundTripper{
				next: &loggingRoundTripper{
					next: &retryRoundTripper{
						next:       http.DefaultTransport,
						maxRetries: 3,
						delay:      1 * time.Second,
					},
				},
			},
			10,
			20,
		),
		Timeout: 10 * time.Second,
		Jar:     jar,
	}

	for _, t := range config.Targets {
		for bp, eps := range t.Endpoints {
			for _, ep := range eps {
				fullPath := t.BaseURL + path.Join(bp, ep.Path)
				handler := proxy(fullPath, ep.Method, client)
				fmt.Printf("Setting up route: %s %s -> %s\n", ep.Method, ep.Path, fullPath)
				mux.Handle(ep.Path, handler)
			}
		}
	}
	return mux
}

func proxy(fullPath, method string, client *http.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		dumpReq, err := httputil.DumpRequest(r, true)
		if err != nil {
			fmt.Println(err)
		}

		if r.Method != method {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			go L.Log(NewLog(time.Now(), "user", "Method Not Allowed", fullPath, r.RemoteAddr, string(dumpReq[:]), "", http.StatusMethodNotAllowed))
			return
		}

		req, err := http.NewRequest(r.Method, fullPath, r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			go L.Log(NewLog(time.Now(), "user", err.Error(), fullPath, r.RemoteAddr, string(dumpReq[:]), "", http.StatusBadGateway))
			return
		}

		req.Header = r.Header

		fmt.Printf("Client memory addr: %v \n", client)
		resp, err := client.Do(req)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			go L.Log(NewLog(time.Now(), "user", err.Error(), fullPath, r.RemoteAddr, string(dumpReq[:]), "", http.StatusBadGateway))
			return
		}

		if err != nil {
			fmt.Println(err)
		}

		defer resp.Body.Close()

		for k, vv := range resp.Header {
			for _, v := range vv {
				w.Header().Add(k, v)
			}
		}
		w.WriteHeader(resp.StatusCode)

		_, err = io.Copy(w, resp.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
	}
}
