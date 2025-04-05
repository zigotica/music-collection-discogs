package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/lib/pq"
)


func getDBConnStr() string {
	// For Docker environment, always use the service name
	host := "db"
	if os.Getenv("DOCKER_ENV") != "true" {
		host = getEnvWithDefault("DB_HOST", "localhost")
	}

	// Add debug logging
	log.Printf("Using database connection: postgres://%s:****@%s:%s/%s?sslmode=%s",
		getEnvWithDefault("DB_USER", "user"),
		host,
		getEnvWithDefault("DB_PORT", "5432"),
		getEnvWithDefault("DB_NAME", "music_collection"),
		getEnvWithDefault("DB_SSLMODE", "disable"))

	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s",
		getEnvWithDefault("DB_USER", "user"),
		getEnvWithDefault("DB_PASSWORD", "password"),
		host,
		getEnvWithDefault("DB_PORT", "5432"),
		getEnvWithDefault("DB_NAME", "music_collection"),
		getEnvWithDefault("DB_SSLMODE", "disable"))
}

// Helper function to get environment variable with default value
func getEnvWithDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

var db *sql.DB

func searchReleases(query string) ([]Release, error) {
	query = "%" + query + "%"
	sqlQuery := `
		SELECT id, catalog_number, artist, title, label, format, rating, released, release_id, 
		       collection_folder, date_added, collection_media_condition, collection_sleeve_condition, 
		       collection_notes, tags, year, cover_image, wanted, physical 
		FROM releases 
		WHERE unaccent(title) ILIKE unaccent($1) 
		   OR unaccent(artist) ILIKE unaccent($1) 
		   OR year::text ILIKE $1 
		   OR unaccent(physical) ILIKE unaccent($1)
		ORDER BY title ASC`

	rows, err := db.Query(sqlQuery, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var releases []Release
	for rows.Next() {
		var r Release
		var coverImage sql.NullString
		if err := rows.Scan(&r.ID, &r.CatalogNumber, &r.Artist, &r.Title, &r.Label, &r.Format, &r.Rating, &r.Released, &r.ReleaseID, &r.CollectionFolder, &r.DateAdded, &r.CollectionMediaCondition, &r.CollectionSleeveCondition, &r.CollectionNotes, &r.Tags, &r.Year, &coverImage, &r.Wanted, &r.Physical); err != nil {
			log.Printf("Error scanning row: %v", err)
			continue
		}
		r.CoverImage = coverImage.String
		releases = append(releases, r)
	}
	return releases, rows.Err()
}

func fetchReleaseByID(id string, orderBy string, orderDirection string) (*Release, error) {
	var release Release
	var coverImage sql.NullString
	query := "SELECT id, title, year, artist, tags, release_id, cover_image, wanted, physical FROM releases WHERE id = $1"

	if orderBy != "" {
		query += " ORDER BY " + orderBy

		if orderDirection != "" {
			query += " " + orderDirection
		}
	}

	err := db.QueryRow(query, id).Scan(
		&release.ID,
		&release.Title,
		&release.Year,
		&release.Artist,
		&release.Tags,
		&release.ReleaseID,
		&coverImage,
		&release.Wanted,
		&release.Physical,
	)
	if err != nil {
		return nil, err
	}

	release.CoverImage = coverImage.String
	return &release, nil
}

func fetchReleasesByYear(year string, orderBy string, orderDirection string) ([]Release, error) {
	query := "SELECT id, catalog_number, artist, title, label, format, rating, released, release_id, collection_folder, date_added, collection_media_condition, collection_sleeve_condition, collection_notes, tags, year, cover_image, wanted, physical FROM releases WHERE year = $1"

	if orderBy != "" {
		query += " ORDER BY " + orderBy

		if orderDirection != "" {
			query += " " + orderDirection
		}
	}

	rows, err := db.Query(query, year)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var releases []Release
  for rows.Next() {
		var r Release
		var coverImage sql.NullString
		if err := rows.Scan(&r.ID, &r.CatalogNumber, &r.Artist, &r.Title, &r.Label, &r.Format, &r.Rating, &r.Released, &r.ReleaseID, &r.CollectionFolder, &r.DateAdded, &r.CollectionMediaCondition, &r.CollectionSleeveCondition, &r.CollectionNotes, &r.Tags, &r.Year, &coverImage, &r.Wanted, &r.Physical); err != nil {
			log.Printf("Error scanning row: %v", err)
			continue
		}
		r.CoverImage = coverImage.String
		releases = append(releases, r)
	}
	return releases, rows.Err()
}

func fetchReleasesByTag(tag string, orderBy string, orderDirection string) ([]Release, error) {
	query := "SELECT id, catalog_number, artist, title, label, format, rating, released, release_id, collection_folder, date_added, collection_media_condition, collection_sleeve_condition, collection_notes, tags, year, cover_image, wanted, physical FROM releases WHERE $1 = ANY(tags)"

	if orderBy != "" {
		query += " ORDER BY " + orderBy

		if orderDirection != "" {
			query += " " + orderDirection
		}
	}

	rows, err := db.Query(query, tag)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var releases []Release
	for rows.Next() {
		var r Release
		var coverImage sql.NullString
		if err := rows.Scan(&r.ID, &r.CatalogNumber, &r.Artist, &r.Title, &r.Label, &r.Format, &r.Rating, &r.Released, &r.ReleaseID, &r.CollectionFolder, &r.DateAdded, &r.CollectionMediaCondition, &r.CollectionSleeveCondition, &r.CollectionNotes, &r.Tags, &r.Year, &coverImage, &r.Wanted, &r.Physical); err != nil {
			log.Printf("Error scanning row: %v", err)
			continue
		}
		r.CoverImage = coverImage.String
		releases = append(releases, r)
	}
	return releases, rows.Err()
}

func fetchReleasesByArtist(artist string, orderBy string, orderDirection string) ([]Release, error) {
	// account for artists with / in the name (i.e. Lennon/Ono)
	artists := strings.Split(artist, "/")
	var conditions []string
	var args []interface{}
	for i, a := range artists {
		conditions = append(conditions, fmt.Sprintf("artist LIKE $%d", i+1))
		args = append(args, "%"+a+"%")
	}
	whereClause := strings.Join(conditions, " OR ")

	query := "SELECT id, catalog_number, artist, title, label, format, rating, released, release_id, collection_folder, date_added, collection_media_condition, collection_sleeve_condition, collection_notes, tags, year, cover_image, wanted, physical FROM releases WHERE " + whereClause

	if orderBy != "" {
		query += " ORDER BY " + orderBy

		if orderDirection != "" {
			query += " " + orderDirection
		}
	}

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var releases []Release
	for rows.Next() {
		var r Release
		var coverImage sql.NullString
		if err := rows.Scan(&r.ID, &r.CatalogNumber, &r.Artist, &r.Title, &r.Label, &r.Format, &r.Rating, &r.Released, &r.ReleaseID, &r.CollectionFolder, &r.DateAdded, &r.CollectionMediaCondition, &r.CollectionSleeveCondition, &r.CollectionNotes, &r.Tags, &r.Year, &coverImage, &r.Wanted, &r.Physical); err != nil {
			log.Printf("Error scanning row: %v", err)
			continue
		}
		r.CoverImage = coverImage.String
		releases = append(releases, r)
	}
	return releases, rows.Err()
}

func fetchReleasesByPhysical(physical string, orderBy string, orderDirection string) ([]Release, error) {
	query := "SELECT id, catalog_number, artist, title, label, format, rating, released, release_id, collection_folder, date_added, collection_media_condition, collection_sleeve_condition, collection_notes, tags, year, cover_image, wanted, physical FROM releases WHERE physical = $1"

	if orderBy != "" {
		query += " ORDER BY " + orderBy

		if orderDirection != "" {
			query += " " + orderDirection
		}
	}

	rows, err := db.Query(query, physical)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var releases []Release
	for rows.Next() {
		var r Release
		var coverImage sql.NullString
		if err := rows.Scan(&r.ID, &r.CatalogNumber, &r.Artist, &r.Title, &r.Label, &r.Format, &r.Rating, &r.Released, &r.ReleaseID, &r.CollectionFolder, &r.DateAdded, &r.CollectionMediaCondition, &r.CollectionSleeveCondition, &r.CollectionNotes, &r.Tags, &r.Year, &coverImage, &r.Wanted, &r.Physical); err != nil {
			log.Printf("Error scanning row: %v", err)
			continue
		}
		r.CoverImage = coverImage.String
		releases = append(releases, r)
	}
	return releases, rows.Err()
}

func fetchReleases(orderBy string, orderDirection string) ([]Release, error) {
	var releases []Release
	query := "SELECT id, catalog_number, artist, title, label, format, rating, released, release_id, collection_folder, date_added, collection_media_condition, collection_sleeve_condition, collection_notes, tags, year, cover_image, wanted, physical FROM releases"

	if orderBy != "" {
		query += " ORDER BY " + orderBy

		if orderDirection != "" {
			query += " " + orderDirection
		}
	}

	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var r Release
		var coverImage sql.NullString
		if err := rows.Scan(&r.ID, &r.CatalogNumber, &r.Artist, &r.Title, &r.Label, &r.Format, &r.Rating, &r.Released, &r.ReleaseID, &r.CollectionFolder, &r.DateAdded, &r.CollectionMediaCondition, &r.CollectionSleeveCondition, &r.CollectionNotes, &r.Tags, &r.Year, &coverImage, &r.Wanted, &r.Physical); err != nil {
			log.Printf("Error scanning row: %v", err)
			continue
		}
		r.CoverImage = coverImage.String
		releases = append(releases, r)
	}

	return releases, rows.Err()
}

func fetchWantedReleases(orderBy string, orderDirection string) ([]Release, error) {
	var releases []Release
	query := "SELECT id, catalog_number, artist, title, label, format, rating, released, release_id, collection_folder, date_added, collection_media_condition, collection_sleeve_condition, collection_notes, tags, year, cover_image, wanted, physical FROM releases WHERE wanted = TRUE"

	if orderBy != "" {
		query += " ORDER BY " + orderBy

		if orderDirection != "" {
			query += " " + orderDirection
		}
	}

	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var r Release
		var coverImage sql.NullString
		if err := rows.Scan(&r.ID, &r.CatalogNumber, &r.Artist, &r.Title, &r.Label, &r.Format, &r.Rating, &r.Released, &r.ReleaseID, &r.CollectionFolder, &r.DateAdded, &r.CollectionMediaCondition, &r.CollectionSleeveCondition, &r.CollectionNotes, &r.Tags, &r.Year, &coverImage, &r.Wanted, &r.Physical); err != nil {
			log.Printf("Error scanning row: %v", err)
			continue
		}
		r.CoverImage = coverImage.String
		releases = append(releases, r)
	}

	return releases, rows.Err()
}

func fetchNeedScrapingReleases(orderBy string, orderDirection string) ([]Release, error) {
	var releases []Release

	// Default ordering
	if orderBy == "" {
		orderBy = "title"
	}
	if orderDirection == "" {
		orderDirection = "asc"
	}

	// Sanitize orderBy and orderDirection
	switch orderBy {
	case "title", "artist", "year":
		// Valid order by values
	default:
		orderBy = "title" // Default to title if invalid
	}

	switch orderDirection {
	case "asc", "desc":
		// Valid order direction values
	default:
		orderDirection = "asc" // Default to ascending if invalid
	}

	// Query for releases with year=0 OR empty/null tags OR empty/null cover_image
	query := fmt.Sprintf(`
		SELECT id, catalog_number, artist, title, label, format, rating, released, release_id, 
		       collection_folder, date_added, collection_media_condition, collection_sleeve_condition, 
		       collection_notes, tags, year, cover_image, wanted, physical
		FROM releases 
		WHERE year = 0 OR tags IS NULL OR array_length(tags, 1) IS NULL OR cover_image IS NULL OR cover_image = '' 
		ORDER BY %s %s
	`, orderBy, orderDirection)

	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var r Release
		var coverImage sql.NullString
		if err := rows.Scan(&r.ID, &r.CatalogNumber, &r.Artist, &r.Title, &r.Label, &r.Format, &r.Rating, &r.Released, &r.ReleaseID, &r.CollectionFolder, &r.DateAdded, &r.CollectionMediaCondition, &r.CollectionSleeveCondition, &r.CollectionNotes, &r.Tags, &r.Year, &coverImage, &r.Wanted, &r.Physical); err != nil {
			log.Printf("Error scanning row: %v", err)
			continue
		}
		r.CoverImage = coverImage.String
		releases = append(releases, r)
	}

	return releases, rows.Err()
}

func setWantedStatus(id string, wanted bool) error {
	_, err := db.Exec("UPDATE releases SET wanted = $1 WHERE id = $2", wanted, id)
	if err != nil {
		log.Printf("Error updating wanted status for release ID: %s: %v", id, err)
	}
	return err
}


// Helper function to get release_id as string from id
func fetchReleaseIDByID(id string) (string, error) {
	var releaseID int
	err := db.QueryRow("SELECT release_id FROM releases WHERE id = $1", id).Scan(&releaseID)
	if err != nil {
		return "", err
	}
	return strconv.Itoa(releaseID), nil
}

func updateReleaseInDB(id, title, artist, year, coverImage string, convertToOwned bool) error {
	query := `
		UPDATE releases 
		SET title = $1, artist = $2, year = $3`
	args := []interface{}{title, artist, year}

	// Only update wanted status if converting from wanted to owned
	if convertToOwned {
		query += `, wanted = $4`
		args = append(args, false)
	}

	if coverImage != "" {
		query += `, cover_image = $` + strconv.Itoa(len(args)+1)
		args = append(args, coverImage)
	}

	query += ` WHERE id = $` + strconv.Itoa(len(args)+1)
	args = append(args, id)

	_, err := db.Exec(query, args...)
	if err != nil {
		log.Printf("Error updating release in database: %v", err)
	}
	return err
}

func updateAllArtistOccurrences(oldArtist, newArtist string) error {
	_, err := db.Exec("UPDATE releases SET artist = $1 WHERE artist = $2", newArtist, oldArtist)
	if err != nil {
		log.Printf("Error updating all artist occurrences from '%s' to '%s': %v", oldArtist, newArtist, err)
		return err
	}
	log.Printf("Successfully updated all artist occurrences from '%s' to '%s'", oldArtist, newArtist)
	return nil
}

func updateReleaseFromScraping(release Release, tags []string, result *strings.Builder) error {
	uniqueTags := make(map[string]bool)
	var dedupedTags []string
	for _, tag := range tags {
		if !uniqueTags[tag] {
			uniqueTags[tag] = true
			dedupedTags = append(dedupedTags, tag)
		}
	}

	year := 0
	for _, tag := range tags {
		if strings.HasPrefix(tag, "year:") {
			if y, err := strconv.Atoi(strings.TrimPrefix(tag, "year:")); err == nil {
				year = y
				break
			}
		}
	}

	var filteredTags []string
	for _, tag := range dedupedTags {
		if !strings.HasPrefix(tag, "year:") {
			filteredTags = append(filteredTags, tag)
		}
	}
	filteredTagsArray := pq.StringArray(filteredTags)

	// Update the release in the database
	res, err := db.Exec("UPDATE releases SET tags = $1, year = $2 WHERE release_id = $3", filteredTagsArray, year, release.ReleaseID)
	if err != nil {
		log.Printf("Error updating tags for release ID: %d, ReleaseID: %d: %v", release.ID, release.ReleaseID, err)
		result.WriteString(fmt.Sprintf("<br>Error updating tags for release ID: %d, ReleaseID: %d: %v\n", release.ID, release.ReleaseID, err))
		return err
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		log.Printf("Error getting rows affected: %v", err)
	}
	if rowsAffected > 0 {
		// log.Printf("Successfully updated year for ReleaseID: %d to %d", release.ReleaseID, year)
		// result.WriteString(fmt.Sprintf("<br>Updated Album %s - year: %d tags: %s\n", release.Title, year, filteredTags))
	} else {
		log.Printf("No rows affected for ReleaseID: %d - check if release_id exists in database", release.ReleaseID)
	}

	return nil
}

func addTagToRelease(id string, tag string) error {
	_, err := db.Exec(`
		UPDATE releases 
		SET tags = array_append(COALESCE(tags, ARRAY[]::TEXT[]), $1) 
		WHERE id = $2 AND (tags IS NULL OR NOT ($1 = ANY(tags)))`,
		tag, id)
	return err
}

func removeTagFromRelease(id string, tag string) error {
	_, err := db.Exec(`
		UPDATE releases 
		SET tags = array_remove(tags, $1) 
		WHERE id = $2`,
		tag, id)
	return err
}

func initDB() {
	var err error
	db, err = sql.Open("postgres", getDBConnStr())
	if err != nil {
		log.Fatal(err)
	}

	// Create unaccent extension if it doesn't exist
	_, err = db.Exec("CREATE EXTENSION IF NOT EXISTS unaccent")
	if err != nil {
		log.Fatal(err)
	}

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS releases (
        id SERIAL PRIMARY KEY,
        catalog_number TEXT,
        artist TEXT,
        title TEXT,
        label TEXT,
        format TEXT,
        rating TEXT,
        released TEXT,
        release_id INT UNIQUE,
        collection_folder TEXT,
        date_added TEXT,
        collection_media_condition TEXT,
        collection_sleeve_condition TEXT,
        collection_notes TEXT,
        tags TEXT[],
        year INT,
        cover_image TEXT,
				wanted BOOLEAN DEFAULT FALSE,
        physical TEXT
    );`)

	if err != nil {
		log.Fatal(err)
	}
}
