package main

import (
	"fmt"
	"log"
	"net/url"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/gocolly/colly"
)

func scrapeLastFM(logMessages *strings.Builder) (string, error) {
	var result strings.Builder

	releases, err := fetchReleases("", "")
		if err != nil {
			log.Fatalf("Error fetching releases: %v", err)
		}

	c := createCollector()
	releaseTags := make(map[int][]string)
	setupHandlers(c, &result, releaseTags, logMessages)

	for _, release := range releases {
	err := scrapeRelease(c, release, releaseTags, &result, logMessages)
		if err != nil {
			result.WriteString(fmt.Sprintf("<br>Error scraping release %d: %v\n", release.ReleaseID, err))
		}
	}

	return result.String(), nil
}

func createCollector() *colly.Collector {
	return colly.NewCollector(
		colly.AllowedDomains("www.last.fm", "lastfm.freetls.fastly.net"),
	)
}

func setupHandlers(c *colly.Collector, result *strings.Builder, releaseTags map[int][]string, logMessages *strings.Builder) {
	c.OnResponse(func(r *colly.Response) {
		handleImageResponse(r)
	})

	c.OnHTML("a.cover-art img", func(e *colly.HTMLElement) {
		handleCoverArt(e, c)
	})

	c.OnHTML(".catalogue-metadata-description", func(e *colly.HTMLElement) {
		handleMetadata(e, result, releaseTags)
	})

	c.OnHTML("a[href*='/tag/']", func(e *colly.HTMLElement) {
		handleTag(e, result, releaseTags)
	})

	// c.OnRequest(func(r *colly.Request) {
	// 	log.Printf("Visiting release URL: %s", r.URL.String())
	// })

	c.OnError(func(r *colly.Response, err error) {
		log.Printf("Error visiting %s: %v", r.Request.URL, err)
		result.WriteString(fmt.Sprintf("<br>Error fetching %s: %v\n", r.Request.URL, err))
	})

	// c.OnResponse(func(r *colly.Response) {
	// 	logMessages.WriteString(fmt.Sprintf("Response from %s (status: %d)\n", r.Request.URL, r.StatusCode))
	// })
}

func scrapeRelease(c *colly.Collector, release Release, releaseTags map[int][]string, result *strings.Builder, logMessages *strings.Builder) error {
	if release.CoverImage != "" && len(release.Tags) > 0 && release.Year != 0 {
		// log.Printf("Skipping scrape for release %s as cover_image, tags, and year are already populated.", release.Title)
		return nil
	}

	url := buildLastFMURL(release)

	ctx := colly.NewContext()
	ctx.Put("releaseID", release.ReleaseID)

	err := c.Request("GET", url, nil, ctx, nil)
	if err != nil {
		return fmt.Errorf("error visiting %s: %v", url, err)
	}

	// logMessages.WriteString(fmt.Sprintf("Successfully scraped: %s ", url))

	if tags, ok := releaseTags[release.ReleaseID]; ok {
		err := updateReleaseFromScraping(release, tags, result)
		if err != nil {
			return fmt.Errorf("error updating database for release %d: %v", release.ReleaseID, err)
		}
	}

	return nil
}

func buildLastFMURL(release Release) string {
	artist := strings.TrimSpace(release.Artist)
	artist = strings.ReplaceAll(artist, " ", "+")
	title := strings.TrimSpace(release.Title)
	title = strings.ReplaceAll(title, " ", "+")
	artist = strings.TrimRight(artist, "+")
	title = strings.TrimRight(title, "+")
	return fmt.Sprintf("https://www.last.fm/music/%s/%s", artist, title)
}

func handleImageResponse(r *colly.Response) {
	if strings.HasPrefix(r.Headers.Get("Content-Type"), "image/") {
		releaseID, ok := r.Ctx.GetAny("releaseID").(int)
		if !ok {
			log.Printf("Error: could not get release ID from context for image")
			return
		}

		if err := os.MkdirAll("web/static/covers", 0755); err != nil {
			log.Printf("Error creating covers directory: %v", err)
			return
		}

		fsPath := fmt.Sprintf("web/static/covers/%d.jpg", releaseID)
		if err := os.WriteFile(fsPath, r.Body, 0644); err != nil {
			log.Printf("Error saving image for release %d: %v", releaseID, err)
			return
		}

		urlPath := fmt.Sprintf("%d.jpg", releaseID)
		_, err := db.Exec("UPDATE releases SET cover_image = $1 WHERE release_id = $2", urlPath, releaseID)
		if err != nil {
			log.Printf("Error updating cover image for release %d: %v", releaseID, err)
		}
	}
}

func handleCoverArt(e *colly.HTMLElement, c *colly.Collector) {
	releaseID, ok := e.Request.Ctx.GetAny("releaseID").(int)
	if !ok {
		log.Printf("Error: could not get release ID from context")
		return
	}

	imgSrc := e.Attr("src")
	if imgSrc != "" {
		ctx := colly.NewContext()
		ctx.Put("releaseID", releaseID)

		parsedURL, err := url.Parse(imgSrc)
		if err != nil {
			log.Printf("Error parsing image URL: %v", err)
			return
		}
		absoluteURL := ""
		if parsedURL.IsAbs() {
			absoluteURL = parsedURL.String()
		} else {
			baseURL, _ := url.Parse("https://www.last.fm")
			absoluteURL = baseURL.ResolveReference(parsedURL).String()
		}

		err = c.Request("GET", absoluteURL, nil, ctx, nil)
		if err != nil {
			log.Printf("Error downloading image for release %d: %v", releaseID, err)
		}
	}
}

func handleMetadata(e *colly.HTMLElement, result *strings.Builder, releaseTags map[int][]string) {
	releaseID, ok := e.Request.Ctx.GetAny("releaseID").(int)
	if !ok {
		log.Printf("Error: could not get release ID from context")
		result.WriteString("<br>Error: could not get release ID from context\n")
		return
	}

	dateStr := strings.TrimSpace(e.Text)
	if dateStr != "" {
		if strings.Contains(dateStr, " ") {
			parts := strings.Split(dateStr, " ")
			if len(parts) > 0 {
				year, err := strconv.Atoi(parts[len(parts)-1])
				if err == nil {
					releaseTags[releaseID] = append(releaseTags[releaseID], fmt.Sprintf("year:%d", year))
				}
			}
		} else {
			year, err := strconv.Atoi(dateStr)
			if err == nil {
				releaseTags[releaseID] = append(releaseTags[releaseID], fmt.Sprintf("year:%d", year))
			}
		}
	}
}

func handleTag(e *colly.HTMLElement, result *strings.Builder, releaseTags map[int][]string) {
	tag := strings.TrimSpace(e.Text)
	releaseID, ok := e.Request.Ctx.GetAny("releaseID").(int)
	if !ok {
		log.Printf("Error: could not get release ID from context")
		result.WriteString("<br>Error: could not get release ID from context\n")
		return
	}

	releaseTags[releaseID] = append(releaseTags[releaseID], tag)
}

func handleScrape(w http.ResponseWriter, r *http.Request) {
	var logMessages strings.Builder
	logMessages.WriteString("<br>Started data scraping from lastFM, please be patient...<br><br>\n")
	result, err := scrapeLastFM(&logMessages)
	if err != nil {
		http.Error(w, fmt.Sprintf("Scraping failed: %v", err), http.StatusInternalServerError)
		return
	}
	fmt.Fprintln(w, logMessages.String(), result)
	logMessages.WriteString("<br>Scraping completed<br>\n")
	log.Println("Scraping completed")
}
