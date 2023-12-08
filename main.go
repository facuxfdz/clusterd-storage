package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"sync/atomic"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type App struct {
	totalRequests  prometheus.Counter
	failedRequests prometheus.Counter
	clusterMode    bool
	hosts          []string
	ready          bool
}

type CheckReady struct {
	handler http.Handler
	app     *App
}

func (cr *CheckReady) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// TODO: if there are no hosts and cluster mode is enabled, then reject request since we are not ready
	if cr.app.clusterMode && len(cr.app.hosts) == 0 {
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte("Not ready yet"))
		return
	}
	cr.handler.ServeHTTP(w, r)
}

func CheckReadyMiddleware(handlerFunc http.HandlerFunc, app *App) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {

		if !app.ready {
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte("Not ready yet"))
			return
		}

		handlerFunc.ServeHTTP(w, r)
	}

}

var leader atomic.Bool

func iamTheLeader() bool {
	return leader.Load()
}

func setIamTheLeader() {
	leader.Store(true)
}

func NewApp() *App {
	app := &App{
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
		ready: false,
	}
	prometheus.MustRegister(app.totalRequests)
	prometheus.MustRegister(app.failedRequests)
	return app
}

func (a *App) handleRead(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		w.Write([]byte("Only GET is allowed"))
		return
	}

	w.Write([]byte("Hello, World!"))
}

func (a *App) handleWrite(w http.ResponseWriter, r *http.Request) {

	if r.Method != http.MethodPost {
		w.Write([]byte("Only POST is allowed"))
		return
	}

	if !iamTheLeader() {
		fmt.Println("Write request received but I am not the leader")
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("I am not the leader"))
		return
	}

	w.Write([]byte("Not implemented yet"))
}

func (a *App) handleMetrics(w http.ResponseWriter, r *http.Request) {
	promhttp.Handler().ServeHTTP(w, r)
}

func ReadConfigFile() map[string]interface{} {
	serverConfigFile := "server.config"
	body, err := ioutil.ReadFile(serverConfigFile)
	if err != nil {
		fmt.Println("Error reading config file")
	}
	configs := make(map[string]interface{})
	lines := strings.Split(string(body), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}

		parts := strings.Split(line, "=")
		if len(parts) != 2 {
			fmt.Println("Invalid line in config file: " + line)
			continue
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		configs[key] = value
	}
	return configs
}

func setupCluster(a *App) {
	http.HandleFunc("/leader", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			w.Write([]byte("Only POST is allowed"))
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("This is the leader"))
		setIamTheLeader()
	})

	http.HandleFunc("/ready", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			w.Write([]byte("Only POST is allowed"))
			return
		}
		a.ready = true
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Join signal received"))
	})

	http.HandleFunc("/hosts", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			w.Write([]byte("Only POST is allowed"))
			return
		}
		// get hosts from body and store in hosts
		decoder := json.NewDecoder(r.Body)
		// hosts will be as follows: {hosts: []string}
		var hostsReq struct {
			Hosts []string
		}
		err := decoder.Decode(&hostsReq)
		if err != nil {
			fmt.Println("Error decoding hosts")
			fmt.Println(err)
		}

		a.hosts = hostsReq.Hosts
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Hosts received"))
	})

}

func main() {
	configs := ReadConfigFile()
	port := configs["port"].(string)
	cluster_mode := configs["cluster_mode"].(string)

	app := NewApp()
	app.clusterMode = false

	if cluster_mode == "yes" {
		app.clusterMode = true
		go setupCluster(app)
		readHandler := http.HandlerFunc(app.handleRead)
		writeHandler := http.HandlerFunc(app.handleWrite)
		http.HandleFunc("/read", CheckReadyMiddleware(readHandler, app))
		http.HandleFunc("/write", CheckReadyMiddleware(writeHandler, app))
	} else {
		http.HandleFunc("/read", app.handleRead)
		http.HandleFunc("/write", app.handleWrite)
		app.ready = true
	}

	http.HandleFunc("/metrics", app.handleMetrics)

	fmt.Println("Listening on port " + port)

	http.ListenAndServe(":"+port, nil)

}
