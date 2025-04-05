package main

import (
	"encoding/csv"
	"fmt"
	"io"
	"log"
	"strconv"
	"strings"
)

func processCSVData(reader *csv.Reader, logMessages *strings.Builder, wanted bool) error {
	logMessages.WriteString("<br>Starting CSV processing...<br>\n")

	header, err := reader.Read()
	if err != nil {
		return fmt.Errorf("error reading CSV header: %v", err)
	}

	// Map column names to indices
	colMap := make(map[string]int)
	for i, col := range header {
		colMap[col] = i
	}

	var totalRecords, validRecords, skippedRecords int

	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Printf("Error reading record: %v", err)
			continue
		}
		totalRecords++

		// log.Printf("\n--- Processing Record %d ---", totalRecords)
		// log.Printf("Raw record data: %v", record)

		// Get required fields
		artist := getField(record, colMap, "Artist")
		title := getField(record, colMap, "Title")
		releaseID := getField(record, colMap, "release_id")

		if artist == "" || title == "" || releaseID == "" {
			skippedRecords++
			// log.Printf("SKIPPED: Missing required data (Artist: '%s', Title: '%s', Release ID: '%s')", artist, title, releaseID)
			continue
		}

		// Convert release_id to integer once
		releaseIDInt, err := strconv.Atoi(releaseID)
		if err != nil {
			log.Printf("Error converting release_id '%s' to int: %v", releaseID, err)
			skippedRecords++
			continue
		}

		// Debug: Show the actual query we're about to run
		// log.Printf("Checking if release_id %d exists in database", releaseIDInt)

		// Check if release_id exists in database
		var exists bool
		err = db.QueryRow("SELECT EXISTS(SELECT 1 FROM releases WHERE release_id = $1)", releaseIDInt).Scan(&exists)
		if err != nil {
			log.Printf("Error checking if release exists: %v", err)
			skippedRecords++
			continue
		}

		// Debug: Show all release_ids in database
		// rows, err := db.Query("SELECT release_id FROM releases ORDER BY release_id")
		// if err != nil {
		// 	log.Printf("Error querying all release_ids: %v", err)
		// } else {

		// 	var ids []int
		// 	for rows.Next() {
		// 		var id int
		// 		if err := rows.Scan(&id); err != nil {
		// 			log.Printf("Error scanning release_id: %v", err)
		// 			continue
		// 		}
		// 		ids = append(ids, id)
		// 	}
		// 	rows.Close()
		// }

		if exists {
			// log.Printf("SKIPPED: Release ID %d already exists in database", releaseIDInt)
			skippedRecords++
			continue
		}

		// Get optional fields
		catalogNum := getField(record, colMap, "Catalog#")
		label := getField(record, colMap, "Label")
		format := getField(record, colMap, "Format")
		rating := getField(record, colMap, "Rating")
		released := getField(record, colMap, "Released")
		collectionFolder := getField(record, colMap, "CollectionFolder")
		dateAdded := getField(record, colMap, "Date Added")
		mediaCondition := getField(record, colMap, "Collection Media Condition")
		sleeveCondition := getField(record, colMap, "Collection Sleeve Condition")
		notes := getField(record, colMap, "Collection Notes")

		// Determine physical format
		physical := determinePhysicalFormat(format)

		// Insert the new release into the database
		_, err = db.Exec(`
                       INSERT INTO releases (artist, title, release_id, catalog_number, label, format, rating, released, collection_folder, date_added, collection_media_condition, collection_sleeve_condition, collection_notes, year, wanted, physical)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
		`, artist, title, releaseIDInt, catalogNum, label, format, rating, released, collectionFolder, dateAdded, mediaCondition, sleeveCondition, notes, 0, wanted, physical)

		if err != nil {
			log.Printf("Error inserting release into database: %v", err)
			skippedRecords++
			continue
		}

		// This releaseIDInt conversion is redundant since we already did it above
		validRecords++
	}

	log.Printf("\n=== Import Summary ===")
	log.Printf("Total records processed: %d", totalRecords)
	log.Printf("Valid records: %d", validRecords)
	log.Printf("Skipped records: %d", skippedRecords)
	logMessages.WriteString("<br>Import Summary<br>\n")
	logMessages.WriteString(fmt.Sprintf("Total records processed: %d<br>\n", totalRecords))
	logMessages.WriteString(fmt.Sprintf("New records: %d<br>\n", validRecords))
	logMessages.WriteString(fmt.Sprintf("Already added records: %d<br><br>\n", skippedRecords))

	// Diagnostic query to check actual database content
	var totalInDB int
	err = db.QueryRow("SELECT COUNT(*) FROM releases").Scan(&totalInDB)
	if err != nil {
		log.Printf("Error counting records in database: %v", err)
	} else {
		log.Printf("\n=== Database Status ===")
		log.Printf("Total records in database: %d", totalInDB)

		// Get a sample of records to verify data
		rows, err := db.Query("SELECT release_id, artist, title FROM releases ORDER BY release_id LIMIT 5")
		if err != nil {
			log.Printf("Error querying sample records: %v", err)
		} else {
			rows.Close()
		}
	}

	return nil
}

func determinePhysicalFormat(format string) string {
	formatLower := strings.ToLower(format)
	if strings.Contains(formatLower, "7\"") ||
		strings.Contains(formatLower, "ep") ||
		strings.Contains(formatLower, "maxi") ||
		strings.Contains(formatLower, "single") {
		return "EP - Single"
	} else if strings.Contains(formatLower, "lp") ||
		strings.Contains(formatLower, "7\"") ||
		strings.Contains(formatLower, "12\"") {
		return "Vinyl"
	} else if strings.Contains(formatLower, "cd") {
		return "CD"
	} else if strings.Contains(formatLower, "dvd") {
		return "DVD"
	} else if strings.Contains(formatLower, "ray") {
		return "Blu-ray"
	} else if strings.Contains(formatLower, "cass") {
		return "Tape"
	}
	return ""
}

// Helper function to safely get field value from CSV record
func getField(record []string, colMap map[string]int, fieldName string) string {
	if idx, ok := colMap[fieldName]; ok && idx < len(record) {
		return record[idx]
	}
	return ""
}

