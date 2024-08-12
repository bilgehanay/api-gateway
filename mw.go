package main

import (
	"errors"
	"fmt"
	"github.com/dgrijalva/jwt-go"
	"golang.org/x/time/rate"
	"net/http"
	"net/http/httputil"
	"time"
)

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
