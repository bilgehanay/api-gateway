package main

import (
	"fmt"
	"io"
	"net/http"
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

func Router() *http.ServeMux {
	mux := http.NewServeMux()

	for _, t := range config.Targets {
		for bp, eps := range t.Endpoints {
			for _, ep := range eps {
				fullPath := t.BaseURL + path.Join(bp, ep.Path)
				handler := proxy(fullPath, ep.Method)
				fmt.Printf("Setting up route: %s %s -> %s\n", ep.Method, ep.Path, fullPath)
				mux.HandleFunc(ep.Path, handler)
			}
		}
	}
	return mux
}

func proxy(fullPath, method string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		dumpReq, err := httputil.DumpRequest(r, true)
		if err != nil {
			fmt.Println(err)
		}

		if r.Method != method {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			go L.Log(NewLog(time.Now(), false, "user", "Method Not Allowed", fullPath, r.RemoteAddr, string(dumpReq[:]), "", http.StatusMethodNotAllowed))
			return
		}

		req, err := http.NewRequest(r.Method, fullPath, r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			go L.Log(NewLog(time.Now(), false, "user", err.Error(), fullPath, r.RemoteAddr, string(dumpReq[:]), "", http.StatusBadGateway))
			return
		}

		req.Header = r.Header

		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			go L.Log(NewLog(time.Now(), false, "user", err.Error(), fullPath, r.RemoteAddr, string(dumpReq[:]), "", http.StatusBadGateway))
			return
		}

		dumpRes, err := httputil.DumpResponse(resp, true)
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
			go L.Log(NewLog(time.Now(), false, "user", err.Error(), fullPath, r.RemoteAddr, string(dumpReq[:]), string(dumpRes[:]), http.StatusBadGateway))
			return
		}
		go L.Log(NewLog(time.Now(), true, "user", "", fullPath, r.RemoteAddr, string(dumpReq[:]), string(dumpRes[:]), http.StatusOK))
		fmt.Println("cevap iletildi")
	}
}
