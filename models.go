package main

import "github.com/lib/pq"

type Release struct {
	ID                        int
	CatalogNumber             string
	Artist                    string
	Title                     string
	Label                     string
	Format                    string
	Rating                    string
	Released                  string
	ReleaseID                 int
	CollectionFolder          string
	DateAdded                 string
	CollectionMediaCondition  string
	CollectionSleeveCondition string
	CollectionNotes           string
	Tags                      pq.StringArray
	Year                      int
	CoverImage                string
	Wanted                    bool
	Physical                  string
}
