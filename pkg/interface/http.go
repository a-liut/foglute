package _interface

import (
	"context"
	"encoding/json"
	"fmt"
	"foglute/internal/model"
	"foglute/pkg/deployment"
	"github.com/gorilla/mux"
	"log"
	"net/http"
	"time"
)

const (
	Address = ""
	Port    = "8080"
)

func handleError(w http.ResponseWriter, status int, message string, args ...interface{}) {
	http.Error(w, fmt.Sprintf(message, args), status)
}

func applicationsHandler(manager *deployment.Manager, w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		deployments := manager.GetDeployments()

		w.Header().Set("Content-Type", "application/json")
		err := json.NewEncoder(w).Encode(deployments)
		if err != nil {
			log.Println(err)
		}
	case http.MethodPost:
		// Decode the JSON in the body and overwrite 'tom' with it
		d := json.NewDecoder(r.Body)
		app := &model.Application{}
		err := d.Decode(app)
		if err != nil {
			handleError(w, http.StatusInternalServerError, err.Error())
			return
		}

		err = manager.AddApplication(app)
		if err != nil {
			handleError(w, http.StatusInternalServerError, "Cannot add application %s", app.Name)
			return
		}

		_, err = fmt.Fprintln(w, "Application added successfully")
		if err != nil {
			log.Println(err)
		}
	default:
		handleError(w, http.StatusMethodNotAllowed, "I can't do that.")
		return
	}
}

func applicationHandler(manager *deployment.Manager, w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	deploy, exists := manager.GetDeployByApplicationID(id)
	if !exists {
		handleError(w, http.StatusNotFound, "Application %s not found", id)
		return
	}

	switch r.Method {
	case http.MethodGet:
		w.Header().Set("Content-Type", "application/json")
		err := json.NewEncoder(w).Encode(deploy)
		if err != nil {
			log.Println(err)
		}
	case http.MethodDelete:
		err := manager.DeleteApplication(deploy.Application)
		if err != nil {
			log.Println()
			handleError(w, http.StatusNotFound, "Cannot delete application %s: %s", deploy.Application.Name, err)
			return
		}

		_, err = fmt.Fprintln(w, "Application deleted successfully")
		if err != nil {
			log.Println(err)
		}
	}
}

func StartHTTPInterface(manager *deployment.Manager, quit chan struct{}) {
	s := http.Server{
		Addr: fmt.Sprintf("%s:%s", Address, Port),
	}

	r := mux.NewRouter()
	r.HandleFunc("/applications", func(writer http.ResponseWriter, request *http.Request) {
		applicationsHandler(manager, writer, request)
	}).
		Methods(http.MethodGet, http.MethodPost)

	r.HandleFunc("/applications/{id}", func(writer http.ResponseWriter, request *http.Request) {
		applicationHandler(manager, writer, request)
	}).Methods(http.MethodGet, http.MethodDelete)

	s.Handler = r

	go func() {
		<-quit

		log.Println("Stopping HTTP server...")
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		//shutdown the server
		err := s.Shutdown(ctx)
		if err != nil {
			log.Printf("Shutdown request error: %v", err)
		}
	}()

	log.Println("Starting HTTP server")

	err := s.ListenAndServe()
	if err != nil {
		log.Println(err)
	}

	log.Println("HTTP server stopped!")
}
