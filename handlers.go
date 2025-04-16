package main

import (
	"io"
	"log"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"encoding/csv"
)

func getSortingData(r *http.Request) map[string]interface{} {
	orderBy := r.URL.Query().Get("order_by")
	orderDirection := r.URL.Query().Get("order_direction")
	artist := r.URL.Query().Get("artist")
	year := r.URL.Query().Get("year")

	// Determine the current route
	requestPath := r.URL.Path

	sortingFields := []map[string]string{
		{"Field": "title", "Label": "Title", "IconUp": "bi-sort-alpha-up", "IconDown": "bi-sort-alpha-down"},
	}

	// Only add "Artist" sorting if we're not on an Artist-specific page
	if !strings.HasPrefix(requestPath, "/artist/") && artist == "" {
		sortingFields = append(sortingFields, map[string]string{"Field": "artist", "Label": "Artist", "IconUp": "bi-sort-alpha-up", "IconDown": "bi-sort-alpha-down"})
	}

	// Only add "Year" sorting if we're not on a Year-specific page
	if !strings.HasPrefix(requestPath, "/year/") && year == "" {
		sortingFields = append(sortingFields, map[string]string{"Field": "year", "Label": "Year", "IconUp": "bi-sort-numeric-up", "IconDown": "bi-sort-numeric-down"})
	}

	return map[string]interface{}{
		"OrderBy":        orderBy,
		"OrderDirection": orderDirection,
		"SortingFields":  sortingFields,
		"Filters": map[string]string{
			"year":     year,
			"artist":   artist,
			"tag":      r.URL.Query().Get("tag"),
			"physical": r.URL.Query().Get("physical"),
		},
	}
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	sortingData := getSortingData(r)

	// Only handle the exact root path
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	log.Printf("Handling index request from %s", r.RemoteAddr)

	orderBy := r.URL.Query().Get("order_by")
	orderDirection := r.URL.Query().Get("order_direction")

	releases, err := fetchReleases(orderBy, orderDirection)
	if err != nil {
		log.Printf("Error fetching releases: %v", err)
		http.Error(w, "Database query error", http.StatusInternalServerError)
		return
	}
	log.Printf("Fetched %d releases", len(releases))

	
	data := struct {
		Releases      []Release
		Year          string
		Tag           string
		Artist        string
		Title         string
		Template      string
		OrderBy       string
		OrderDirection string
		SortingFields  []map[string]string
		Filters        map[string]string
	}{
		Releases:      releases,
		Title:         constructTitle("Music Collection", len(releases)),
		Template:      "index",
		OrderBy:       sortingData["OrderBy"].(string),
		OrderDirection: sortingData["OrderDirection"].(string),
		SortingFields: sortingData["SortingFields"].([]map[string]string),
		Filters:       sortingData["Filters"].(map[string]string),
	}

	// Render template directly to the response writer
	if err := Templates.ExecuteTemplate(w, "base.html", data); err != nil {
		log.Printf("Error rendering template: %v", err)
		// At this point, we've likely already written some content to the response
		// so we can't send a proper HTTP error status code
		return
	}

	log.Printf("Successfully rendered index page")
}

func searchHandler(w http.ResponseWriter, r *http.Request) {
	sortingData := getSortingData(r)
	query := r.URL.Query().Get("query")
	if query == "" {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	releases, err := searchReleases(query)
	if err != nil {
		log.Printf("Error searching releases: %v", err)
		http.Error(w, "Database query error", http.StatusInternalServerError)
		return
	}

	data := struct {
		Releases      []Release
		Title         string
		Template      string
		Wanted        bool
		NeedScrape    bool
		OrderBy       string
		OrderDirection string
		Year          string
		Tag           string
		Artist        string
		Physical      string
		IsSearch      bool
		SortingFields  []map[string]string
		Filters        map[string]string
	}{
		Releases:      releases,
		Title:         fmt.Sprintf("Search results for '%s'", query),
		Template:      "releases",
		Wanted:        false,
		NeedScrape:    false,
		OrderBy:       "",
		OrderDirection: "",
		Year:          "",
		Tag:           "",
		Artist:        "",
		Physical:      "",
		IsSearch:      true,
		SortingFields: sortingData["SortingFields"].([]map[string]string),
		Filters:       sortingData["Filters"].(map[string]string),
	}

	if err := Templates.ExecuteTemplate(w, "base.html", data); err != nil {
		log.Printf("Error rendering template: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}


func releaseHandler(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(r.URL.Path[len("/release/"):], "/")
if len(parts) < 2 {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	id := parts[0]
	action := parts[1]

	switch action {

	case "edit":
		release, err := fetchReleaseByID(id, "", "")
		if err != nil {
			http.Error(w, "Release not found", http.StatusNotFound)
			return
		}
		data := struct {
			*Release
			Title    string
			Template string
		}{
			Release:  release,
			Title:    release.Title,
			Template: "edit",
		}
		if err := Templates.ExecuteTemplate(w, "base.html", data); err != nil {
			log.Printf("Error rendering edit template: %v", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		}
	case "update":
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if err := updateReleaseHandler(id, r); err != nil {
			http.Error(w, "Error updating release", http.StatusInternalServerError)
			return
		}
		http.Redirect(w, r, "/", http.StatusSeeOther)
	case "add-tag":
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		tag := r.FormValue("tag")
		if tag == "" {
			http.Error(w, "Tag cannot be empty", http.StatusBadRequest)
			return
		}
		log.Printf("Adding tag '%s' to release ID: %s", tag, id)
		if err := addTagToRelease(id, tag); err != nil {
			http.Error(w, "Error adding tag", http.StatusInternalServerError)
			return
		}
		http.Redirect(w, r, "/release/"+id+"/edit", http.StatusSeeOther)
	case "remove-tag":
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		tag := r.FormValue("tag")
		if err := removeTagFromRelease(id, tag); err != nil {
			http.Error(w, "Error removing tag", http.StatusInternalServerError)
			return
		}
		http.Redirect(w, r, "/release/"+id+"/edit", http.StatusSeeOther)
	default:
		http.Error(w, "Invalid action", http.StatusBadRequest)
	}
}

func wantedReleasesHandler(w http.ResponseWriter, r *http.Request) {
	sortingData := getSortingData(r)

	orderBy := r.URL.Query().Get("order_by")
	orderDirection := r.URL.Query().Get("order_direction")

	releases, err := fetchWantedReleases(orderBy, orderDirection)
	if err != nil {
		log.Printf("Error fetching wanted releases: %v", err)
		http.Error(w, "Database query error", http.StatusInternalServerError)
		return
	}

	title := fmt.Sprintf("Wanted Releases (%d)", len(releases))

	data := struct {
		Year          string
		Tag           string
		Artist        string
		Releases      []Release
		Template      string
		Title         string
		NeedScrape    bool
		Wanted        bool
		Physical      string
		OrderBy       string
		OrderDirection string
		SortingFields  []map[string]string
		Filters        map[string]string
	}{
		Releases:      releases,
		Template:      "releases",
		Title:         title,
		NeedScrape:    false,
		Wanted:        true,
		Physical:      "",
		OrderBy:       sortingData["OrderBy"].(string),
		OrderDirection: sortingData["OrderDirection"].(string),
		SortingFields: sortingData["SortingFields"].([]map[string]string),
		Filters:       sortingData["Filters"].(map[string]string),
	}

	if err := Templates.ExecuteTemplate(w, "base.html", data); err != nil {
		log.Printf("Error rendering template: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
}

func statsHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("Handling stats request from %s", r.RemoteAddr)

	decadeStats, err := fetchStatsByDecade()
	if err != nil {
		http.Error(w, "Error fetching decade statistics", http.StatusInternalServerError)
		return
	}

	formatStats, err := fetchStatsByFormat()
	if err != nil {
		http.Error(w, "Error fetching format statistics", http.StatusInternalServerError)
		return
	}

	artistStats, err := fetchStatsTopArtists()
	if err != nil {
		http.Error(w, "Error fetching artist statistics", http.StatusInternalServerError)
		return
	}

	data := struct {
		Title       string
		Template    string
		DecadeStats []StatItem
		FormatStats []StatItem
		ArtistStats []StatItem
	}{
		Title:       "Collection Statistics",
		Template:    "stats",
		DecadeStats: decadeStats,
		FormatStats: formatStats,
		ArtistStats: artistStats,
	}

	if err := Templates.ExecuteTemplate(w, "base.html", data); err != nil {
		log.Printf("Error rendering stats template: %v", err)
		// Avoid sending another error if headers already sent
	}
	log.Printf("Successfully rendered stats page")
}

func needScrapingReleasesHandler(w http.ResponseWriter, r *http.Request) {
	sortingData := getSortingData(r) // Get sorting data

	orderBy := r.URL.Query().Get("order_by")
	orderDirection := r.URL.Query().Get("order_direction")

	releases, err := fetchNeedScrapingReleases(orderBy, orderDirection)
	if err != nil {
		log.Printf("Error fetching releases that need scraping: %v", err)
		http.Error(w, "Database query error", http.StatusInternalServerError)
		return
	}

	title := fmt.Sprintf("Need scraping (%d)", len(releases))

	// Update the data struct to include SortingFields and Filters
	data := struct {
		Year           string
		Tag            string
		Artist         string
		Releases       []Release
		Template       string
		Title          string
		NeedScrape     bool
		Wanted         bool
		OrderBy        string
		OrderDirection string
		Physical       string
		SortingFields  []map[string]string // Added
		Filters        map[string]string   // Added
	}{
		Releases:       releases,
		Template:       "releases",
		Title:          title,
		NeedScrape:     true,
		Wanted:         false,
		OrderBy:        sortingData["OrderBy"].(string),       // Use sortingData
		OrderDirection: sortingData["OrderDirection"].(string), // Use sortingData
		Physical:       "",
		SortingFields:  sortingData["SortingFields"].([]map[string]string), // Populate
		Filters:        sortingData["Filters"].(map[string]string),         // Populate
	}

	if err := Templates.ExecuteTemplate(w, "base.html", data); err != nil {
		log.Printf("Error rendering template: %v", err)
		// Avoid calling http.Error if headers might have been written
		// http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
}

type ReleasesData struct {
	Year     string
	Tag      string
	Artist   string
	Releases []Release
	Template string
}

func constructTitle(title string, count int) string {
	return fmt.Sprintf("%s (%d)", title, count)
}

func releasesHandler(w http.ResponseWriter, r *http.Request) {
	sortingData := getSortingData(r)

	path := r.URL.Path
	var releases []Release
	var err error
	var year, tag, artist, physical string

	orderBy := r.URL.Query().Get("order_by")
	orderDirection := r.URL.Query().Get("order_direction")

	// log.Printf("Handling releases request for path: %s", path)

	// Parse the URL path to determine the filter
	parts := strings.Split(path, "/")
	if len(parts) >= 3 {
		filterType := parts[1]
		filterValue := parts[2]

		switch filterType {
		case "year":
			year = filterValue
			releases, err = fetchReleasesByYear(filterValue, orderBy, orderDirection)
		case "tag":
			tag = filterValue
			releases, err = fetchReleasesByTag(filterValue, orderBy, orderDirection)
		case "artist":
			artist = filterValue
			releases, err = fetchReleasesByArtist(filterValue, orderBy, orderDirection)
		case "format":
			physical = filterValue
			releases, err = fetchReleasesByPhysical(filterValue, orderBy, orderDirection)
		default:
			releases, err = fetchReleases(orderBy, orderDirection)
		}
	} else {
		releases, err = fetchReleases(orderBy, orderDirection)
	}

	if err != nil {
		log.Printf("Error fetching releases: %v", err)
		http.Error(w, "Database query error", http.StatusInternalServerError)
		return
	}

	title := "All Releases"
	if year != "" {
		title = "Albums from " + year
	} else if tag != "" {
		title = "Albums tagged " + tag
	} else if artist != "" {
		title = "Albums by " + artist
	} else if physical != "" {
		title = "Albums in " + physical
	}

	data := struct {
		Year          string
		Tag           string
		Artist        string
		Physical      string
		Releases      []Release
		Template      string
		Title         string
		Wanted        bool
		NeedScrape    bool
		OrderBy        string
		OrderDirection string
		SortingFields  []map[string]string
		Filters        map[string]string
	}{
		Year:          year,
		Tag:           tag,
		Artist:        artist,
		Physical:      physical,
		Releases:      releases,
		Template:      "releases",
		Title:         constructTitle(title, len(releases)),
		Wanted:        false,
		NeedScrape:    false,
		OrderBy:       sortingData["OrderBy"].(string),
		OrderDirection: sortingData["OrderDirection"].(string),
		SortingFields: sortingData["SortingFields"].([]map[string]string),
		Filters:       sortingData["Filters"].(map[string]string),
	}

	if err := Templates.ExecuteTemplate(w, "base.html", data); err != nil {
		log.Printf("Error rendering template: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func updateReleaseHandler(id string, r *http.Request) error {
	// Parse the multipart form data
	if err := r.ParseMultipartForm(32 << 20); err != nil { // 32MB max
		log.Printf("Error parsing multipart form: %v", err)
		return err
	}

	// Get the file from the form
	file, header, err := r.FormFile("cover")
	if err != nil && err != http.ErrMissingFile {
		log.Printf("Error getting form file: %v", err)
		return err
	}

	var coverImage string
	if file != nil {
		defer file.Close()

		// Create the covers directory if it doesn't exist
		coverDir := "web/static/covers"
		if err := os.MkdirAll(coverDir, 0755); err != nil {
			log.Printf("Error creating covers directory: %v", err)
			return err
		}

		// First get the release_id from the database
		releaseID, err := fetchReleaseIDByID(id)
		if err != nil {
			log.Printf("Error getting release_id: %v", err)
			return err
		}

		// Get the file extension from the original filename
		ext := filepath.Ext(header.Filename)
		// Create the new filename using the release_id
		coverImage = releaseID + ext
		filePath := filepath.Join(coverDir, coverImage)

		// Create the destination file
		dst, err := os.Create(filePath)
		if err != nil {
			log.Printf("Error creating destination file: %v", err)
			return err
		}
		defer dst.Close()

		// Copy the uploaded file to the destination
		if _, err := io.Copy(dst, file); err != nil {
			log.Printf("Error copying file: %v", err)
			return err
		}
	}

	artist := r.FormValue("artist")
	oldArtist := r.FormValue("old_artist")
	updateAll := r.FormValue("update_all_artist_occurrencies") == "on"
	convertToOwned := r.FormValue("convert_to_owned") == "on"

	// Update the database with the new information including the cover image
	err = updateReleaseInDB(id, r.FormValue("title"), artist, r.FormValue("year"), coverImage, convertToOwned)
	if err != nil {
		return err
	}

	if updateAll && artist != oldArtist {
		err = updateAllArtistOccurrences(oldArtist, artist)
		if err != nil {
			log.Printf("Error updating all artist occurrences: %v", err)
			return err
		}
	}

	return nil
}

func uploadHandler(w http.ResponseWriter, r *http.Request) {
	// Parse the multipart form data with a maximum size of 32MB
	err := r.ParseMultipartForm(32 << 20)
	if err != nil {

		http.Error(w, "Invalid form", http.StatusBadRequest)
		return
	}

	// Get the file from the form
	file, _, err := r.FormFile("file")
	if err != nil {

		http.Error(w, "Missing file", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Create a CSV reader from the file
	reader := csv.NewReader(file)
	reader.FieldsPerRecord = -1 // Allow variable number of fields

	var logMessages strings.Builder

	// Process the CSV data
	if err := processCSVData(reader, &logMessages, false); err != nil {
		http.Error(w, "Error importing CSV data: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Return log messages and success message
	w.Write([]byte(logMessages.String() + "CSV data imported successfully!"))
}

func uploadWantedHandler(w http.ResponseWriter, r *http.Request) {
	// Parse the multipart form data with a maximum size of 32MB
	err := r.ParseMultipartForm(32 << 20)
	if err != nil {

		http.Error(w, "Invalid form", http.StatusBadRequest)
		return
	}

	// Get the file from the form
	file, _, err := r.FormFile("file")
	if err != nil {

		http.Error(w, "Missing file", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Create a CSV reader from the file
	reader := csv.NewReader(file)
	reader.FieldsPerRecord = -1 // Allow variable number of fields

	var logMessages strings.Builder

	// Process the CSV data
	if err := processCSVData(reader, &logMessages, true); err != nil {
		http.Error(w, "Error importing CSV data: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Return log messages and success message
	w.Write([]byte(logMessages.String() + "CSV data imported successfully!"))
}

func adminHandler(w http.ResponseWriter, r *http.Request) {
	data := struct {
		Template string
		Title    string
	}{
		Template: "admin",
		Title:    "Admin Panel",
	}

	if err := Templates.ExecuteTemplate(w, "base.html", data); err != nil {
		log.Printf("Error rendering template: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
}
