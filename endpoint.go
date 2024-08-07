package main

import (
	"fmt"
	"net/http"
	"path"
)

type Endpoint struct {
	Path        string `json:"path"`
	Method      string `json:"method"`
	Description string `json:"description"`
}

type ServiceEndpoints map[string][]Endpoint

type Target struct {
	BaseURL   string           `json:"baseUrl"`
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
				handler := Proxy(fullPath, ep.Method)
				fmt.Printf("Setting up route: %s %s -> %s\n", ep.Method, ep.Path, fullPath)
				mux.HandleFunc(ep.Path, handler)
			}
		}
	}
	return mux
}
