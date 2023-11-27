package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type App struct {
	mu             sync.Mutex
	sharedSuccess  bool
	totalRequests  prometheus.Counter
	failedRequests prometheus.Counter
}

func NewApp() *App {
	app := &App{
		sharedSuccess: true,
		totalRequests: prometheus.NewCounter(
			prometheus.CounterOpts{
				Name: "total_requests",
				Help: "Total number of requests",
			}),
		failedRequests: prometheus.NewCounter(
			prometheus.CounterOpts{
				Name: "failed_requests",
				Help: "Total number of failed requests",
			}),
	}
	prometheus.MustRegister(app.totalRequests)
	prometheus.MustRegister(app.failedRequests)
	return app
}

func (a *App) handleExample(w http.ResponseWriter, r *http.Request) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.totalRequests.Inc()
	if !a.sharedSuccess {
		a.failedRequests.Inc()
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal server error"))
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Hello, World!"))
}

func (a *App) handleShared(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		w.Write([]byte("Only POST is allowed"))
		return
	}

	var requestBody struct {
		Success bool
	}

	err := json.NewDecoder(r.Body).Decode(&requestBody)
	if err != nil {
		fmt.Println(err)
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("Invalid JSON"))
		return
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	a.sharedSuccess = requestBody.Success
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Shared variable updated"))
}

func (a *App) handleMetrics(w http.ResponseWriter, r *http.Request) {
	promhttp.Handler().ServeHTTP(w, r)
}
func main() {
	app := NewApp()

	http.HandleFunc("/example", app.handleExample)
	http.HandleFunc("/shared", app.handleShared)
	http.HandleFunc("/metrics", app.handleMetrics)

	http.ListenAndServe(":8080", nil)

}
