package main

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"regexp"
)

// empty
type TemplateParams struct{}

type GoServiceRequest struct {
	Pill string `json: pill`
}

type GoServiceResponse struct{}

func saveHandler(w http.ResponseWriter, r *http.Request) {
	// Get form params and send call with hardcoded device id to backend
	pill := r.URL.Query().Get("pill") // red or blue
	fmt.Fprintln(w, "pill = "+pill)
	// req := GoServiceRequest{Pill: pill}
	// // json marshal
	// resp, err := callBackend()
	// if err != nil {
	// 	http.Error(w, err.Error(), http.StatusInternalServerError)
	// }
	// http.Redirect(w, r, "/", http.StatusFound)
}

func callBackend() error {
	return nil
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	renderTemplate(w, "index")
}

var templates = template.Must(template.ParseFiles("edit.html", "view.html", "index.html"))

func renderTemplate(w http.ResponseWriter, tmpl string) {
	err := templates.ExecuteTemplate(w, tmpl+".html", &TemplateParams{})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

var validPath = regexp.MustCompile("^/(edit|save|view)/([a-zA-Z0-9]+)$")

func makeHandler(fn func(http.ResponseWriter, *http.Request, string)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		m := validPath.FindStringSubmatch(r.URL.Path)
		if m == nil {
			http.NotFound(w, r)
			return
		}
		fn(w, r, m[2])
	}
}

func main() {
	http.HandleFunc("/", indexHandler)
	// http.HandleFunc("/", makeHandler(viewHandler))
	// http.HandleFunc("/view/", makeHandler(viewHandler))
	// http.HandleFunc("/edit/", makeHandler(editHandler))
	http.HandleFunc("/save", saveHandler)

	log.Fatal(http.ListenAndServe(":8080", nil))
}
