package scrapers

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/PuerkitoBio/goquery"
	"mma-scheduler/internal/models"
)

const (
	wikiUFCEventsURL = "https://en.wikipedia.org/wiki/List_of_UFC_events"
)

type WikiEventScraper struct {
	client         *http.Client
	db             *sql.DB
	existingEvents []*models.Event
}

type RowspanData struct {
	Value    string
	RowsLeft int
}

func NewWikiEventScraper(db *sql.DB) *WikiEventScraper {
	return &WikiEventScraper{
		client: &http.Client{
			Timeout: 60 * time.Second,
		},
		db: db,
	}
}

func (s *WikiEventScraper) EnhanceEventsWithWikiData(ctx context.Context) (int, error) {
	// First, fetch all events from our database
	if err := s.loadExistingEvents(ctx); err != nil {
		return 0, fmt.Errorf("failed to load existing events: %w", err)
	}

	log.Printf("Loaded %d events from database", len(s.existingEvents))

	// Now fetch the Wikipedia UFC events page
	req, err := http.NewRequestWithContext(ctx, "GET", wikiUFCEventsURL, nil)
	if err != nil {
		return 0, fmt.Errorf("error creating request: %w", err)
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/123.0.0.0 Safari/537.36")

	resp, err := s.client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("error fetching wiki page: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("bad status code: %d", resp.StatusCode)
	}

	log.Printf("Successfully fetched Wikipedia page with status code: %d", resp.StatusCode)

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("error parsing HTML: %w", err)
	}

	// Process tables in the page
	totalUpdated := 0

	// Function to process tables
	processTable := func(table *goquery.Selection, tableType string) (int, error) {
		updatedCount, err := s.processTable(ctx, table)
		if err != nil {
			log.Printf("Error processing %s table: %v", tableType, err)
			return 0, err
		}
		log.Printf("Processed %s table: %d events updated", tableType, updatedCount)
		return updatedCount, nil
	}

	// Try to find tables by their content
	doc.Find("table.wikitable").Each(func(i int, table *goquery.Selection) {
		caption := strings.TrimSpace(table.Find("caption").Text())

		if strings.Contains(strings.ToLower(caption), "past") {
			count, _ := processTable(table, "Past Events")
			totalUpdated += count
		} else if strings.Contains(strings.ToLower(caption), "scheduled") ||
			strings.Contains(strings.ToLower(caption), "upcoming") {
			count, _ := processTable(table, "Scheduled Events")
			totalUpdated += count
		}
	})

	if totalUpdated == 0 {
		log.Printf("No tables with obvious captions found, trying direct processing of any wikitable")

		// Process all wikitables
		doc.Find("table.wikitable").Each(func(i int, table *goquery.Selection) {
			log.Printf("Processing wikitable #%d", i+1)
			count, _ := processTable(table, fmt.Sprintf("Table #%d", i+1))
			totalUpdated += count
		})
	}

	// After Wikipedia scraping is complete, process events that weren't updated
	nonUpdatedCount, err := s.updateNonMatchedEvents(ctx)
	if err != nil {
		log.Printf("Error updating non-matched events: %v", err)
	} else {
		log.Printf("Updated %d non-matched events with names from URLs", nonUpdatedCount)
		totalUpdated += nonUpdatedCount
	}

	return totalUpdated, nil
}

func (s *WikiEventScraper) updateNonMatchedEvents(ctx context.Context) (int, error) {
	updateCount := 0

	// Process each existing event
	for _, event := range s.existingEvents {
		// If the event has no name or has an empty name but has a UFC URL
		if (event.Name == "" || strings.TrimSpace(event.Name) == "") && event.UFCURL != "" {
			// Generate a name from the URL
			generatedName := generateNameFromURL(event.UFCURL)
			log.Printf("Generating name for event ID=%s from URL: %s -> %s", 
				event.ID, event.UFCURL, generatedName)

			// Update the event with the generated name
			event.Name = generatedName
			if err := s.updateEventInDB(ctx, event); err != nil {
				log.Printf("Error updating event %s with generated name: %v", event.ID, err)
				continue
			}

			updateCount++
			log.Printf("Updated event ID=%s with generated name: %s", event.ID, generatedName)
		}
	}

	return updateCount, nil
}

func (s *WikiEventScraper) loadExistingEvents(ctx context.Context) error {
	query := `
		SELECT id, name, event_date, venue, city, country, ufc_url, status, wiki_url, attendance
		FROM events
		ORDER BY event_date
	`

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to query events: %w", err)
	}
	defer rows.Close()

	s.existingEvents = []*models.Event{}

	for rows.Next() {
		event := &models.Event{}
		var wikiURL, status, attendance sql.NullString

		err := rows.Scan(
			&event.ID,
			&event.Name,
			&event.Date,
			&event.Venue,
			&event.City,
			&event.Country,
			&event.UFCURL,
			&status,
			&wikiURL,
			&attendance,
		)

		if err != nil {
			return fmt.Errorf("failed to scan event row: %w", err)
		}

		if status.Valid {
			event.Status = status.String
		}
		if wikiURL.Valid {
			event.WikiURL = wikiURL.String
		}
		if attendance.Valid {
			event.Attendance = attendance.String
		}

		s.existingEvents = append(s.existingEvents, event)
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("error iterating event rows: %w", err)
	}

	return nil
}

func (s *WikiEventScraper) processTable(ctx context.Context, tableEl *goquery.Selection) (int, error) {
	if tableEl.Length() == 0 {
		return 0, fmt.Errorf("table element is empty")
	}

	// Extract column positions for headers
	var eventCol, dateCol, venueCol, locationCol, attendanceCol int
	eventCol = -1
	dateCol = -1
	venueCol = -1
	locationCol = -1
	attendanceCol = -1

	// Get column positions which can vary between tables
	tableEl.Find("thead tr th").Each(func(i int, s *goquery.Selection) {
		text := strings.TrimSpace(s.Text())
		log.Printf("Column %d: '%s'", i, text)
		switch strings.ToLower(text) {
		case "event":
			eventCol = i
		case "date":
			dateCol = i
		case "venue":
			venueCol = i
		case "location":
			locationCol = i
		case "attendance":
			attendanceCol = i
		}
	})

	// If we still don't have column positions, try another selector
	if eventCol == -1 || dateCol == -1 {
		log.Printf("Couldn't find columns using thead, trying direct th search")
		tableEl.Find("tr th").Each(func(i int, s *goquery.Selection) {
			text := strings.TrimSpace(s.Text())
			log.Printf("Alt column %d: '%s'", i, text)
			switch strings.ToLower(text) {
			case "event":
				eventCol = i
			case "date":
				dateCol = i
			case "venue":
				venueCol = i
			case "location":
				locationCol = i
			case "attendance":
				attendanceCol = i
			}
		})
	}

	log.Printf("Column positions - Event: %d, Date: %d, Venue: %d, Location: %d, Attendance: %d",
		eventCol, dateCol, venueCol, locationCol, attendanceCol)

	// Abort if we can't find the essential columns
	if eventCol == -1 || dateCol == -1 {
		return 0, fmt.Errorf("couldn't find essential columns in table")
	}

	// Create a map to store rowspan data for each column
	rowspanMap := make(map[int]*RowspanData)

	rowCount := 0
	updatedCount := 0

	// Now process rows
	tableEl.Find("tbody tr").Each(func(i int, row *goquery.Selection) {
		rowCount++

		// Skip rows with "Canceled" in text
		if strings.Contains(strings.ToLower(row.Text()), "canceled") {
			log.Printf("Skipping canceled event row %d", i)
			return
		}

		// Get event details from this row
		var eventName, eventURL, dateText, venue, location, attendance string

		// Extract event name and URL
		if eventCol >= 0 && eventCol < row.Find("td").Length() {
			eventCell := row.Find("td").Eq(eventCol)
			eventLink := eventCell.Find("a")
			eventName = strings.TrimSpace(eventLink.Text())
			eventURL, _ = eventLink.Attr("href")
			if eventURL != "" {
				eventURL = "https://en.wikipedia.org" + eventURL
			}
		} else {
			log.Printf("Row %d: Event column out of range", i)
			return
		}

		// Extract date
		if dateCol >= 0 && dateCol < row.Find("td").Length() {
			dateCell := row.Find("td").Eq(dateCol)
			dateText = strings.TrimSpace(dateCell.Text())

			// Also try to parse the full date from span
			dateSpan := dateCell.Find("span")
			if dateSpan.Length() > 0 {
				dataSortValue, exists := dateSpan.Attr("data-sort-value")
				if exists && dataSortValue != "" {
					log.Printf("Found date sort value: %s", dataSortValue)

					// Extract actual date from format like "000000002021-07-31-0000"
					re := regexp.MustCompile(`\d{4}-\d{2}-\d{2}`)
					match := re.FindString(dataSortValue)
					if match != "" {
						dateText = match
					}
				}
			}
		} else {
			log.Printf("Row %d: Date column out of range", i)
			return
		}

		// Process venue - handle rowspan properly
		venue = s.getColumnValueWithRowspan(row, venueCol, rowspanMap, "venue")

		// Process location - handle rowspan properly
		location = s.getColumnValueWithRowspan(row, locationCol, rowspanMap, "location")

		// Process attendance - handle rowspan properly
		attendance = s.getColumnValueWithRowspan(row, attendanceCol, rowspanMap, "attendance")

		// Skip rows with attendance explicitly marked as canceled
		if strings.Contains(strings.ToLower(attendance), "cancel") {
			log.Printf("Skipping event with canceled attendance: %s", eventName)
			return
		}

		// Skip if we don't have enough data
		if eventName == "" || dateText == "" || venue == "" {
			log.Printf("Row %d: Skipping due to missing data - Event: '%s', Date: '%s', Venue: '%s'",
				i, eventName, dateText, venue)
			return
		}

		log.Printf("Row %d: Event: '%s', Date: '%s', Venue: '%s', Location: '%s', Attendance: '%s'",
			i, eventName, dateText, venue, location, attendance)

		// Look for a matching event
		matchedEvent := s.findMatchingEventByDate(eventName, dateText, venue, location)
		if matchedEvent == nil {
			log.Printf("No match found for event: %s on %s at %s", eventName, dateText, venue)
			return
		}

		// Set the wiki URL
		matchedEvent.WikiURL = eventURL

		// Handle name update logic
		if eventName != "" {
			// If Wikipedia has a name, use it
			matchedEvent.Name = eventName
		} else if matchedEvent.Name == "" || strings.TrimSpace(matchedEvent.Name) == "" {
			// If no name in DB and none from Wikipedia, generate from URL
			matchedEvent.Name = generateNameFromURL(matchedEvent.UFCURL)
			log.Printf("Generated name from URL: %s", matchedEvent.Name)
		}

		// Format the attendance properly
		formattedAttendance := formatAttendance(attendance)

		// Update the matched event with wiki data
		matchedEvent.WikiURL = eventURL
		matchedEvent.Name = eventName // Set name from Wikipedia
		matchedEvent.Attendance = formattedAttendance

		// Update the event in the database
		if err := s.updateEventInDB(ctx, matchedEvent); err != nil {
			log.Printf("Error updating event %s: %v", matchedEvent.ID, err)
		} else {
			log.Printf("Updated event: %s with wiki URL: %s, attendance: %s",
				matchedEvent.Name, matchedEvent.WikiURL, matchedEvent.Attendance)
			updatedCount++
		}
	})

	log.Printf("Processed %d rows from table, updated %d events", rowCount, updatedCount)

	return updatedCount, nil
}

func (s *WikiEventScraper) getColumnValueWithRowspan(row *goquery.Selection, colIndex int,
	rowspanMap map[int]*RowspanData, colName string) string {

	// Check if we have a carried-over rowspan value for this column
	if data, exists := rowspanMap[colIndex]; exists && data.RowsLeft > 0 {
		// Use the remembered value and decrement the rows left
		value := data.Value
		data.RowsLeft--

		// Remove from map if this was the last row using this value
		if data.RowsLeft == 0 {
			delete(rowspanMap, colIndex)
		}

		log.Printf("Using carried-over %s from rowspan: '%s' (rows left: %d)",
			colName, value, data.RowsLeft)
		return value
	}

	// Otherwise, get the value from the current row's cell
	if colIndex >= 0 && colIndex < row.Find("td").Length() {
		cell := row.Find("td").Eq(colIndex)
		value := strings.TrimSpace(cell.Text())

		// Check if this cell has a rowspan attribute
		if rowspanStr, hasRowspan := cell.Attr("rowspan"); hasRowspan {
			rowspan, _ := strconv.Atoi(rowspanStr)
			if rowspan > 1 {
				// Remember this value for future rows
				rowspanMap[colIndex] = &RowspanData{
					Value:    value,
					RowsLeft: rowspan - 1, // -1 because we're using it for current row
				}
				log.Printf("Found %s with rowspan %d: '%s'", colName, rowspan, value)
			}
		}

		return value
	}

	// Column doesn't exist in this row
	return ""
}

func formatAttendance(attendance string) string {
	if attendance == "" {
		return ""
	}

	// Remove common prefixes
	attendance = strings.TrimPrefix(attendance, "Attendance: ")

	// Remove notes/citations in brackets or parentheses
	re := regexp.MustCompile(`\[.*?\]|\(.*?\)`)
	attendance = re.ReplaceAllString(attendance, "")

	// Clean up whitespace
	attendance = strings.TrimSpace(attendance)

	// Handle "behind closed doors" and similar phrases
	if strings.Contains(strings.ToLower(attendance), "closed doors") ||
		strings.Contains(strings.ToLower(attendance), "no attendance") {
		return "0"
	}

	// Handle dash or em-dash
	if attendance == "—" || attendance == "-" {
		return ""
	}

	return attendance
}

func (s *WikiEventScraper) findMatchingEventByDate(eventName string, dateText string, venue string, location string) *models.Event {
	// First, try to match PPV events directly (like "UFC 72")
	ppvMatch := s.findPPVEventMatch(eventName)
	if ppvMatch != nil {
		log.Printf("Found PPV event match: %s for %s", ppvMatch.ID, eventName)
		return ppvMatch
	}

	// Parse the date from Wikipedia
	var wikiDate time.Time
	var err error

	// Try different date formats
	dateFormats := []string{
		"2006-01-02", // ISO format
		"January 2, 2006",
		"Jan 2, 2006",
	}

	for _, format := range dateFormats {
		wikiDate, err = time.Parse(format, dateText)
		if err == nil {
			log.Printf("Successfully parsed date '%s' with format '%s'", dateText, format)
			break
		}
	}

	if wikiDate.IsZero() {
		log.Printf("Failed to parse date from: %s", dateText)
		return nil
	}

	// Extract city and country from location
	wikiCity, wikiCountry := extractCityCountry(location)
	wikiCity = standardizeText(wikiCity)
	wikiCountry = standardizeText(wikiCountry)
	wikiVenue := standardizeText(venue)

	log.Printf("Looking for events on date: %s with standardized city: %s, country: %s, venue: %s",
		wikiDate.Format("2006-01-02"), wikiCity, wikiCountry, wikiVenue)

	// Track best match
	var bestMatch *models.Event
	bestMatchScore := 0

	// Try to match by date with a 3-day window for timezone differences
	for _, event := range s.existingEvents {
		// Convert both dates to their date parts only for comparison
		wikiDateOnly := time.Date(wikiDate.Year(), wikiDate.Month(), wikiDate.Day(), 0, 0, 0, 0, time.UTC)
		eventDateOnly := time.Date(event.Date.Year(), event.Date.Month(), event.Date.Day(), 0, 0, 0, 0, time.UTC)

		// Initialize match score
		matchScore := 0

		// Check date match with extended window
		dateMatch := false
		dayDiff := 0

		// Exact date match
		if eventDateOnly.Equal(wikiDateOnly) {
			dateMatch = true
			matchScore += 5 // Higher score for exact match
			dayDiff = 0
			log.Printf("Exact date match: %s for Wiki date %s",
				event.Date.Format("2006-01-02"), wikiDate.Format("2006-01-02"))
		} else {
			// Check days before and after (within +/- 2 days)
			for delta := -2; delta <= 2; delta++ {
				if delta == 0 {
					continue // Already checked exact match
				}

				checkDate := wikiDateOnly.AddDate(0, 0, delta)
				if eventDateOnly.Equal(checkDate) {
					dateMatch = true
					// Higher score for closer dates
					matchScore += 5 - abs(delta)
					dayDiff = delta
					log.Printf("Date match %+d day(s): %s for Wiki date %s",
						delta, event.Date.Format("2006-01-02"), wikiDate.Format("2006-01-02"))
					break
				}
			}
		}

		// Skip to next event if dates don't match within window
		if !dateMatch {
			continue
		}

		// Standardize database values for comparison
		dbCity := standardizeText(event.City)
		dbCountry := standardizeText(event.Country)
		dbVenue := standardizeText(event.Venue)

		// Check city match with standardized values
		if dbCity == wikiCity {
			matchScore += 4
			log.Printf("Exact city match: '%s'", event.City)
		} else if strings.Contains(dbCity, wikiCity) || strings.Contains(wikiCity, dbCity) {
			// Partial city match
			matchScore += 2
			log.Printf("Partial city match: DB='%s', Wiki='%s'", event.City, wikiCity)
		}

		// Check country match
		if dbCountry == wikiCountry {
			matchScore += 3
			log.Printf("Exact country match: '%s'", event.Country)
		} else if strings.Contains(dbCountry, wikiCountry) || strings.Contains(wikiCountry, dbCountry) {
			// Partial country match
			matchScore += 1
			log.Printf("Partial country match: DB='%s', Wiki='%s'", event.Country, wikiCountry)
		}

		// Check venue with standardized values (handle cases like "The Chelsea - The Cosmopolitan" vs "The Chelsea at The Cosmopolitan")
		if dbVenue == wikiVenue {
			matchScore += 3
			log.Printf("Exact venue match: '%s'", event.Venue)
		} else if strings.Contains(dbVenue, wikiVenue) || strings.Contains(wikiVenue, dbVenue) {
			// Partial venue match
			matchScore += 2
			log.Printf("Partial venue match: DB='%s', Wiki='%s'", event.Venue, venue)
		} else {
			// Try matching core parts of venue names
			dbVenueParts := strings.Fields(dbVenue)
			wikiVenueParts := strings.Fields(wikiVenue)

			commonWords := 0
			for _, dbPart := range dbVenueParts {
				if len(dbPart) < 3 {
					continue // Skip short words like "at", "the", etc.
				}

				for _, wikiPart := range wikiVenueParts {
					if len(wikiPart) < 3 {
						continue
					}

					if strings.Contains(dbPart, wikiPart) || strings.Contains(wikiPart, dbPart) {
						commonWords++
						break
					}
				}
			}

			if commonWords > 0 {
				matchScore += min(commonWords, 2) // Up to 2 points for common venue words
				log.Printf("Venue word match (%d common words): DB='%s', Wiki='%s'",
					commonWords, event.Venue, venue)
			}
		}

		// Check URL for location hints (e.g., "berlin" in URL for events in Berlin)
		// This helps with the Berlin/China case
		urlLower := strings.ToLower(event.UFCURL)
		if strings.Contains(urlLower, strings.ToLower(wikiCity)) {
			matchScore += 2
			log.Printf("URL contains city name '%s': %s", wikiCity, event.UFCURL)
		}

		// Log candidate match details
		log.Printf("Event ID=%s, Date=%s, City='%s', Country='%s', Venue='%s', Score=%d",
			event.ID, event.Date.Format("2006-01-02"), event.City, event.Country, event.Venue, matchScore)

		// Update best match if score is better
		if matchScore > bestMatchScore {
			bestMatchScore = matchScore
			bestMatch = event
			log.Printf("New best match: Event ID=%s, Score=%d", event.ID, matchScore)
		} else if matchScore == bestMatchScore && bestMatch != nil {
			// Tiebreaker: prefer the match with the closer date
			bestMatchDate := time.Date(bestMatch.Date.Year(), bestMatch.Date.Month(), bestMatch.Date.Day(), 0, 0, 0, 0, time.UTC)
			bestDayDiff := 0

			if bestMatchDate.Equal(wikiDateOnly) {
				bestDayDiff = 0
			} else {
				for delta := -2; delta <= 2; delta++ {
					checkDate := wikiDateOnly.AddDate(0, 0, delta)
					if bestMatchDate.Equal(checkDate) {
						bestDayDiff = abs(delta)
						break
					}
				}
			}

			if abs(dayDiff) < bestDayDiff {
				bestMatch = event
				log.Printf("Same score but closer date - new best match: %s", event.ID)
			} else if abs(dayDiff) == bestDayDiff {
				// Further tiebreaker: prefer match with city in URL
				if strings.Contains(urlLower, strings.ToLower(wikiCity)) {
					bestMatch = event
					log.Printf("Same score and date but URL matches city - new best match: %s", event.ID)
				}
			}
		}
	}

	// Consider it a match if score is at least 5
	if bestMatchScore >= 5 {
		log.Printf("Best match found: Event ID=%s with score %d", bestMatch.ID, bestMatchScore)
		return bestMatch
	}

	log.Printf("No matching event found for date %s in %s, %s (best score: %d)",
		wikiDate.Format("2006-01-02"), wikiCity, wikiCountry, bestMatchScore)
	return nil
}

func (s *WikiEventScraper) findPPVEventMatch(eventName string) *models.Event {
	// Check if it's a numbered UFC event
	re := regexp.MustCompile(`UFC\s+(\d+)`)
	matches := re.FindStringSubmatch(eventName)

	if len(matches) < 2 {
		// Not a numbered UFC event
		return nil
	}

	// Extract the UFC number
	ufcNumber := matches[1]
	log.Printf("Looking for PPV event UFC %s", ufcNumber)

	// Look for URL pattern match in our database
	expectedURL := fmt.Sprintf("https://www.ufc.com/event/ufc-%s", ufcNumber)

	for _, event := range s.existingEvents {
		// Check exact URL match
		if event.UFCURL == expectedURL {
			log.Printf("Found exact URL match for UFC %s: %s", ufcNumber, event.UFCURL)
			return event
		}

		// Also check for alternate URL formats
		alternateURL := fmt.Sprintf("https://www.ufc.com/event/ufc-%s-", ufcNumber)
		if strings.HasPrefix(event.UFCURL, alternateURL) {
			log.Printf("Found URL prefix match for UFC %s: %s", ufcNumber, event.UFCURL)
			return event
		}
	}

	// If no URL match, try regex to find the event number in the name or URL
	for _, event := range s.existingEvents {
		// Skip events without UFC URLs
		if event.UFCURL == "" {
			continue
		}

		if strings.Contains(event.UFCURL, "/ufc-"+ufcNumber) ||
			strings.Contains(event.UFCURL, "/ufc"+ufcNumber) {
			log.Printf("Found UFC %s in URL: %s", ufcNumber, event.UFCURL)
			return event
		}
	}

	log.Printf("No match found for UFC %s", ufcNumber)
	return nil
}

// Helper function to get absolute value of an int
func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// Helper function to extract city and country from location string
func extractCityCountry(location string) (string, string) {
	// Remove any notes/citations
	re := regexp.MustCompile(`\[.*?\]|\(.*?\)`)
	location = re.ReplaceAllString(location, "")
	location = strings.TrimSpace(location)

	// Fix common abbreviations
	if strings.Contains(location, "U.S.") {
		location = strings.Replace(location, "U.S.", "United States", -1)
	}
	if strings.Contains(location, "U.K.") {
		location = strings.Replace(location, "U.K.", "United Kingdom", -1)
	}

	// Split by comma
	parts := strings.Split(location, ",")

	if len(parts) == 0 {
		return "", ""
	} else if len(parts) == 1 {
		return parts[0], ""
	} else {
		// Last part is typically the country
		country := strings.TrimSpace(parts[len(parts)-1])

		// City is the first part
		city := strings.TrimSpace(parts[0])

		return city, country
	}
}

// updateEventInDB updates an event in the database with wiki data
func (s *WikiEventScraper) updateEventInDB(ctx context.Context, event *models.Event) error {
	query := `
		UPDATE events
		SET name = $1, wiki_url = $2, attendance = $3, updated_at = CURRENT_TIMESTAMP
		WHERE id = $4
	`

	_, err := s.db.ExecContext(
		ctx,
		query,
		event.Name,
		event.WikiURL,
		event.Attendance,
		event.ID,
	)

	if err != nil {
		return fmt.Errorf("failed to update event: %w", err)
	}

	return nil
}

// generateNameFromURL creates a formatted name from a UFC URL
func generateNameFromURL(ufcURL string) string {
	// Extract the last path component
	path := strings.TrimSuffix(ufcURL, "/")
	parts := strings.Split(path, "/")
	if len(parts) == 0 {
		return "UFC Event"
	}

	slug := parts[len(parts)-1]

	// Replace hyphens with spaces
	slug = strings.ReplaceAll(slug, "-", " ")

	// Split into words
	words := strings.Fields(slug)

	// Title case each word
	for i, word := range words {
		if len(word) == 0 {
			continue
		}

		// Special case for acronyms like "UFC", "PPV"
		if strings.ToUpper(word) == "UFC" || strings.ToUpper(word) == "PPV" {
			words[i] = strings.ToUpper(word)
		} else {
			// Title case: first letter uppercase, rest lowercase
			words[i] = strings.ToUpper(word[:1]) + strings.ToLower(word[1:])
		}
	}

	return strings.Join(words, " ")
}

// standardizeText standardizes text for comparison by removing diacritics and normalizing whitespace
func standardizeText(s string) string {
    if s == "" {
        return ""
    }

    // Convert to lowercase for case-insensitive comparison
    t := strings.ToLower(s)

    // Replace common location separators
    t = strings.ReplaceAll(t, " - ", " at ")
    t = strings.ReplaceAll(t, " @ ", " at ")

    // Create a new string builder to hold the result
    var result strings.Builder

    // Process each rune, handling diacritics
    for _, r := range t {
        if r < 128 {
            // ASCII character, just keep it
            result.WriteRune(r)
        } else {
            // For non-ASCII characters, check if it's a letter with diacritic
            if unicode.IsLetter(r) {
                // Get base character by removing diacritics
                for _, base := range []rune{'a', 'e', 'i', 'o', 'u', 'c', 'n'} {
                    if isVariantOf(r, base) {
                        result.WriteRune(base)
                        break
                    }
                }
                // If no variant found, just skip this character
            }
        }
    }

    // Normalize whitespace
    normalizedStr := result.String()
    spaceRegex := regexp.MustCompile(`\s+`)
    normalizedStr = spaceRegex.ReplaceAllString(normalizedStr, " ")

    return strings.TrimSpace(normalizedStr)
}

// isVariantOf checks if a unicode character is a variant (with diacritic) of a base character
func isVariantOf(r rune, base rune) bool {
    // Common diacritic variants for Latin alphabets
    variants := map[rune][]rune{
        'a': {'à', 'á', 'â', 'ã', 'ä', 'å', 'ā', 'ă', 'ą'},
        'e': {'è', 'é', 'ê', 'ë', 'ē', 'ĕ', 'ė', 'ę', 'ě'},
        'i': {'ì', 'í', 'î', 'ï', 'ĩ', 'ī', 'ĭ', 'į', 'ı'},
        'o': {'ò', 'ó', 'ô', 'õ', 'ö', 'ø', 'ō', 'ŏ', 'ő'},
        'u': {'ù', 'ú', 'û', 'ü', 'ũ', 'ū', 'ŭ', 'ů', 'ű', 'ų'},
        'c': {'ç', 'ć', 'č', 'ĉ'},
        'n': {'ñ', 'ń', 'ň'},
    }
    
    if variantList, exists := variants[base]; exists {
        for _, variant := range variantList {
            if r == variant {
                return true
            }
        }
    }
    
    return false
}