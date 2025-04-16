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

// fetchTagsForReleaseID retrieves the current tags for a given release ID.
func fetchTagsForReleaseID(id string) ([]string, error) {
	var tags pq.StringArray
	err := db.QueryRow("SELECT tags FROM releases WHERE id = $1", id).Scan(&tags)
	if err != nil {
		if err == sql.ErrNoRows {
			return []string{}, nil // Return empty slice if no tags found or release doesn't exist
		}
		log.Printf("Error fetching tags for release ID %s: %v", id, err)
		return nil, err
	}
	// Handle potential NULL tags column by returning an empty slice
	if tags == nil {
		return []string{}, nil
	}
	return tags, nil
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

// updateReleaseInDB updates the core fields of a release and ensures the decade tag is correct.
func updateReleaseInDB(id, title, artist, yearStr, coverImage string, convertToOwned bool) error {
	// Fetch current tags
	currentTags, err := fetchTagsForReleaseID(id)
	if err != nil {
		// Log the error but proceed, assuming empty tags if fetch failed
		log.Printf("Warning: Could not fetch current tags for release ID %s: %v", id, err)
		currentTags = []string{}
	}

	// Prepare new tags list, filtering out old decade tags
	newTags := []string{}
	for _, tag := range currentTags {
		// Check if the tag matches the decade format "YYYYs"
		if len(tag) >= 5 && strings.HasSuffix(tag, "s") {
			if _, err := strconv.Atoi(tag[:len(tag)-1]); err == nil && len(tag[:len(tag)-1]) == 4 {
				// It's a decade tag, skip it
				continue
			}
		}
		newTags = append(newTags, tag)
	}

	// Parse year and add new decade tag
	yearInt, err := strconv.Atoi(yearStr)
	if err == nil && yearInt > 0 {
		decade := (yearInt / 10) * 10
		decadeTag := fmt.Sprintf("%ds", decade)
		// Add the new decade tag if it's not already present (shouldn't be after filtering, but check just in case)
		tagExists := false
		for _, tag := range newTags {
			if tag == decadeTag {
				tagExists = true
				break
			}
		}
		if !tagExists {
			newTags = append(newTags, decadeTag)
		}
	} else if err != nil {
		log.Printf("Warning: Invalid year '%s' provided for release ID %s. Cannot determine decade tag.", yearStr, id)
		// Keep year as 0 or invalid value in DB if conversion fails
		yearStr = "0"
	}


	// Build the query dynamically
	query := `UPDATE releases SET title = $1, artist = $2, year = $3, tags = $4`
	args := []interface{}{title, artist, yearStr, pq.StringArray(newTags)}
	argCounter := 5 // Start counting args from 5

	// Only update wanted status if converting from wanted to owned
	if convertToOwned {
		query += fmt.Sprintf(", wanted = $%d", argCounter)
		args = append(args, false)
		argCounter++
	}

	if coverImage != "" {
		query += fmt.Sprintf(", cover_image = $%d", argCounter)
		args = append(args, coverImage)
		argCounter++
	}

	query += fmt.Sprintf(" WHERE id = $%d", argCounter)
	args = append(args, id)

	_, err = db.Exec(query, args...)
	if err != nil {
		log.Printf("Error updating release in database (ID: %s): %v", id, err)
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

// --- Statistics Functions ---

type StatItem struct {
	Label string
	Count int
}

// fetchStatsByDecade counts owned releases grouped by decade.
func fetchStatsByDecade() ([]StatItem, error) {
	query := `
		SELECT (year / 10) * 10 AS decade, COUNT(*) as count
		FROM releases
		WHERE wanted = FALSE AND year > 0  -- Exclude wanted and releases with year 0
		GROUP BY decade
		ORDER BY decade ASC;
	`
	rows, err := db.Query(query)
	if err != nil {
		log.Printf("Error fetching stats by decade: %v", err)
		return nil, err
	}
	defer rows.Close()

	var stats []StatItem
	for rows.Next() {
		var decade int
		var count int
		if err := rows.Scan(&decade, &count); err != nil {
			log.Printf("Error scanning decade stat row: %v", err)
			continue
		}
		stats = append(stats, StatItem{Label: fmt.Sprintf("%ds", decade), Count: count})
	}
	return stats, rows.Err()
}

// fetchStatsByFormat counts owned releases grouped by physical format.
func fetchStatsByFormat() ([]StatItem, error) {
	query := `
		SELECT COALESCE(physical, 'Unknown') as format, COUNT(*) as count
		FROM releases
		WHERE wanted = FALSE
		GROUP BY COALESCE(physical, 'Unknown') -- Use the expression instead of the alias
		ORDER BY count DESC;
	`
	rows, err := db.Query(query)
	if err != nil {
		log.Printf("Error fetching stats by format: %v", err)
		return nil, err
	}
	defer rows.Close()

	var stats []StatItem
	for rows.Next() {
		var item StatItem
		if err := rows.Scan(&item.Label, &item.Count); err != nil {
			log.Printf("Error scanning format stat row: %v", err)
			continue
		}
		stats = append(stats, item)
	}
	return stats, rows.Err()
}

// fetchStatsTopArtists gets the top 10 artists by owned release count.
func fetchStatsTopArtists() ([]StatItem, error) {
	query := `
		SELECT artist, COUNT(*) as count
		FROM releases
		WHERE wanted = FALSE AND artist IS NOT NULL AND artist != ''
		GROUP BY artist
		ORDER BY count DESC
		LIMIT 20;
	`
	rows, err := db.Query(query)
	if err != nil {
		log.Printf("Error fetching top artists stats: %v", err)
		return nil, err
	}
	defer rows.Close()

	var stats []StatItem
	for rows.Next() {
		var item StatItem
		if err := rows.Scan(&item.Label, &item.Count); err != nil {
			log.Printf("Error scanning top artist stat row: %v", err)
			continue
		}
		stats = append(stats, item)
	}
	return stats, rows.Err()
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

	// Determine year from "year:YYYY" tags
	year := 0
	for _, tag := range dedupedTags { // Use dedupedTags here
		if strings.HasPrefix(tag, "year:") {
			if y, err := strconv.Atoi(strings.TrimPrefix(tag, "year:")); err == nil {
				year = y
				break // Found the year, no need to check further
			}
		}
	}

	// Filter tags: remove "year:YYYY" and existing decade tags ("YYYYs")
	finalTags := []string{}
	uniqueCheck := make(map[string]bool) // To ensure final tags are unique

	for _, tag := range dedupedTags {
		// Skip year tags
		if strings.HasPrefix(tag, "year:") {
			continue
		}
		// Skip existing decade tags
		if len(tag) >= 5 && strings.HasSuffix(tag, "s") {
			if _, err := strconv.Atoi(tag[:len(tag)-1]); err == nil && len(tag[:len(tag)-1]) == 4 {
				continue // Skip decade tag
			}
		}
		// Add other tags if not already added
		if !uniqueCheck[tag] {
			finalTags = append(finalTags, tag)
			uniqueCheck[tag] = true
		}
	}


	// Add the correct decade tag if year is valid
	if year > 0 {
		decade := (year / 10) * 10
		decadeTag := fmt.Sprintf("%ds", decade)
		if !uniqueCheck[decadeTag] { // Add only if not already present
			finalTags = append(finalTags, decadeTag)
			uniqueCheck[decadeTag] = true
		}
	}

	finalTagsArray := pq.StringArray(finalTags)

	// Update the release in the database with filtered tags and determined year
	res, err := db.Exec("UPDATE releases SET tags = $1, year = $2 WHERE release_id = $3", finalTagsArray, year, release.ReleaseID)
	if err != nil {
		log.Printf("Error updating tags/year for release ID: %d, ReleaseID: %d: %v", release.ID, release.ReleaseID, err)
		result.WriteString(fmt.Sprintf("<br>Error updating tags/year for release ID: %d, ReleaseID: %d: %v\n", release.ID, release.ReleaseID, err))
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
