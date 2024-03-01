package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
)

const (
	ServiceAddr = "https://flutter-deferred-deep-link-23vgxprgkq-uc.a.run.app"
)

// empty
type TemplateParams struct{}

type ServiceRequest struct {
	DeviceID string `json:"device_id"`
	Pill     string `json:"pill"`
}

type ServiceResponse struct{}

func getClientIPFromHttpHeaders(header http.Header) (string, error) {
	// See https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/X-Forwarded-For#syntax
	xForwardedFor := header.Get("X-Forwarded-For")
	if xForwardedFor == "" {
		return "", fmt.Errorf("X-Forwarded-For header is empty")
	}
	ips := strings.Split(xForwardedFor, ", ")
	if ips[0] == "" {
		return "", fmt.Errorf("client ip is empty")
	}
	return ips[0], nil
}

func saveHandler(w http.ResponseWriter, r *http.Request) {
	clientIP, err := getClientIPFromHttpHeaders(r.Header)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
	}

	pill := r.URL.Query().Get("pill") // red or blue

	req := &ServiceRequest{Pill: pill, DeviceID: clientIP}
	data, err := json.Marshal(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

	resp, err := http.Post(ServiceAddr, "application/json", bytes.NewBuffer(data))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

	fmt.Fprintln(w, "Got response: "+string(body))
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	renderTemplate(w, "index")
}

var templates = template.Must(template.ParseFiles("index.html"))

func renderTemplate(w http.ResponseWriter, tmpl string) {
	err := templates.ExecuteTemplate(w, tmpl+".html", &TemplateParams{})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func setupFrontend(ctx context.Context, port string) {
	http.HandleFunc("/", indexHandler)
	http.HandleFunc("/save", saveHandler)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
