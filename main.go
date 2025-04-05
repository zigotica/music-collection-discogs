package main

import (
	"html/template"
	"log"
	"net/http"
)

// Helper functions
func dict(values ...interface{}) map[string]interface{} {
	m := make(map[string]interface{})
	for i := 0; i < len(values); i += 2 {
		key, _ := values[i].(string)
		m[key] = values[i+1]
	}
	return m
}

func slice(values ...interface{}) []interface{} {
	return values
}

var Templates *template.Template

func init() {
	// Parse all templates at startup
	var err error
	Templates = template.New("").Funcs(template.FuncMap{
		"dict": func(values ...interface{}) map[string]interface{} {
			m := make(map[string]interface{})
			for i := 0; i < len(values); i += 2 {
				key, _ := values[i].(string)
				m[key] = values[i+1]
			}
			return m
		},
		"slice": func(values ...interface{}) []interface{} {
			return values
		},
	})

	// Enable more detailed error reporting for templates
	Templates = Templates.Option("missingkey=error")

	// Parse all templates
	templateFiles := []string{
		"web/templates/base.html",
		"web/templates/index.html",
		"web/templates/releases.html",
		"web/templates/release.html",
		"web/templates/edit.html",
		"web/templates/admin.html",
		"web/templates/sorting.html",
	}

	Templates, err = Templates.ParseFiles(templateFiles...)
	if err != nil {
		log.Fatalf("Error parsing templates: %v", err)
	}

	log.Printf("Templates loaded successfully")
}

func main() {
	initDB()

	// Serve static files
	fs := http.FileServer(http.Dir("web/static"))
	http.Handle("/static/", http.StripPrefix("/static/", fs))

	// Route handlers
	http.HandleFunc("/", indexHandler)
	http.HandleFunc("/admin", adminHandler)
	http.HandleFunc("/scrape", handleScrape)
	http.HandleFunc("/releases/wanted", wantedReleasesHandler)
	http.HandleFunc("/releases/need-scraping", needScrapingReleasesHandler)
	http.HandleFunc("/format/", releasesHandler)
	http.HandleFunc("/release/", releaseHandler)
	http.HandleFunc("/artist/", releasesHandler)
	http.HandleFunc("/year/", releasesHandler)
	http.HandleFunc("/tag/", releasesHandler)
	http.HandleFunc("/upload", uploadHandler)
	http.HandleFunc("/upload-wanted", uploadWantedHandler)
	http.HandleFunc("/search", searchHandler)

	log.Println("Server started on :8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
