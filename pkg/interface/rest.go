/*
 * FogLute
 *
 * A Microservice Fog Orchestration platform.
 *
 * API version: 1.0.0
 * Contact: andrea.liut@gmail.com
 */
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

// A Response is a wrapper object for server's responses
type Response struct {
	Message string `json:"message"`
	Error   string `json:"error"`
}

type IotCapData struct {
	Name string `json:"name"`
}

func newResponse(message string, errorMessage string) *Response {
	return &Response{
		Message: message,
		Error:   errorMessage,
	}
}

// Handles error responses
func handleError(w http.ResponseWriter, status int, message string, args ...interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	r := newResponse("", fmt.Sprintf(message, args))
	j, _ := json.Marshal(r)
	http.Error(w, string(j), status)
}

func applicationsHandler(manager *deployment.Manager, w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")

	switch r.Method {
	case http.MethodGet:
		// Returns all active deployments
		deployments := manager.GetDeployments()

		err := json.NewEncoder(w).Encode(deployments)
		if err != nil {
			log.Println(err)
		}
	case http.MethodPost:
		// Decode the application
		var app model.Application

		err := json.NewDecoder(r.Body).Decode(&app)
		if err != nil {
			handleError(w, http.StatusInternalServerError, err.Error())
			return
		}

		go func() {
			// Add the application to the manager
			addErrors := manager.AddApplication(&app)
			if addErrors != nil {
				log.Printf("Some errors have been reported during application %s deployment: %s", app.Name, addErrors)
			} else {
				log.Printf("No errors during %s app deployment", app.Name)
			}
		}()

		// Send a successful response
		r := newResponse("Application deployment request added successfully", "")
		if err = json.NewEncoder(w).Encode(r); err != nil {
			log.Println(err)
		}
	default:
		handleError(w, http.StatusMethodNotAllowed, "Operation not allowed")
		return
	}
}

func applicationHandler(manager *deployment.Manager, w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	vars := mux.Vars(r)
	id := vars["id"]

	// Fetch the application
	deploy, exists := manager.GetDeployByApplicationID(id)
	if !exists {
		handleError(w, http.StatusNotFound, "Application %s not found", id)
		return
	}

	switch r.Method {
	case http.MethodGet:
		// Send the application
		err := json.NewEncoder(w).Encode(deploy)
		if err != nil {
			log.Println(err)
		}
	case http.MethodDelete:
		// Remove the application from the manager
		go func() {
			if deleteErrors := manager.DeleteApplication(deploy.Application); deleteErrors != nil {
				log.Printf("Error during deleting application %s: %s", deploy.Application.Name, deleteErrors)
			} else {
				log.Printf("No errors during %s app deletion", deploy.Application.Name)
			}
		}()

		// Send a successful response
		r := newResponse("Application deletion request added successfully", "")
		if sendErr := json.NewEncoder(w).Encode(r); sendErr != nil {
			log.Println(sendErr)
		}
	}
}

// Starts the HTTP server
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
			log.Printf("Shutdown request error: %v\n", err)
		}
	}()

	log.Printf("Starting HTTP server on port %s\n", Port)

	err := s.ListenAndServe()
	if err != nil {
		log.Println(err)
	}

	log.Println("HTTP server stopped!")
}
