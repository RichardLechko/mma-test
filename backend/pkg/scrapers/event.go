package scrapers

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

type EventScraper struct {
	*BaseScraper
	wikiURL string
}

func NewEventScraper(config ScraperConfig) *EventScraper {
	return &EventScraper{
		BaseScraper: NewBaseScraper(config),
		wikiURL:     "https://en.wikipedia.org/wiki/List_of_UFC_events",
	}
}

type WikiEvent struct {
	Name       string
	Date       time.Time
	Venue      string
	City       string
	Country    string
	Attendance string
	Status     string // Added field to track if event was canceled
	IsCanceled bool   // Flag to determine if this event should be filtered out
	WikiURL    string // Added field to store the event's Wikipedia URL
}

func (s *EventScraper) ScrapeEvents() ([]*WikiEvent, error) {
	var allEvents []*WikiEvent

	req, err := http.NewRequest("GET", s.wikiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %v", err)
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) Firefox/123.0")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error fetching page: %v", err)
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error parsing HTML: %v", err)
	}

	// Scrape scheduled events
	scheduledEvents, err := s.scrapeScheduledEvents(doc)
	if err != nil {
		log.Printf("Warning: %v", err)
	} else {
		allEvents = append(allEvents, scheduledEvents...)
	}

	// Scrape past events
	pastEvents, err := s.scrapePastEvents(doc)
	if err != nil {
		log.Printf("Warning: %v", err)
	} else {
		allEvents = append(allEvents, pastEvents...)
	}
	
	// Filter out events with background color indicating cancellation
	var filteredEvents []*WikiEvent
	canceledCount := 0
	for _, event := range allEvents {
		if !event.IsCanceled {
			filteredEvents = append(filteredEvents, event)
		} else {
			canceledCount++
		}
	}
	
	log.Printf("Total events found: %d, canceled events filtered out: %d, final count: %d", 
		len(allEvents), canceledCount, len(filteredEvents))
	
	return filteredEvents, nil
}

func (s *EventScraper) scrapeScheduledEvents(doc *goquery.Document) ([]*WikiEvent, error) {
	var events []*WikiEvent

	// Find the table with id "Scheduled_events"
	table := doc.Find("table#Scheduled_events")
	if table.Length() == 0 {
		return nil, fmt.Errorf("scheduled events table not found")
	}

	// Find all rows in the tbody
	rows := table.Find("tbody > tr")
	if rows.Length() == 0 {
		return nil, fmt.Errorf("no rows found in scheduled events table")
	}

	// Keep track of rowspan data
	type RowspanData struct {
		Value    string
		RowsLeft int
	}
	
	var currentVenue, currentLocation *RowspanData

	rows.Each(func(i int, row *goquery.Selection) {
		// Skip header rows or rows without enough cells
		cells := row.Find("td")
		if cells.Length() < 3 {
			return
		}

		// Extract event name from the first column
		nameCell := cells.Eq(0)
		name := strings.TrimSpace(nameCell.Text())
		if name == "" || !strings.Contains(name, "UFC") {
			return
		}

		var event WikiEvent
		event.Name = name
		
		// Extract Wiki URL from the name link
		nameLink := nameCell.Find("a[href]").First()
		if nameLink.Length() > 0 {
			if href, exists := nameLink.Attr("href"); exists && href != "" {
				// Make sure it's an absolute URL
				if strings.HasPrefix(href, "/wiki/") {
					event.WikiURL = "https://en.wikipedia.org" + href
				} else if strings.HasPrefix(href, "https://") {
					event.WikiURL = href
				}
			}
		}
		
		// Add debug logging to verify URL extraction
		if event.WikiURL == "" {
			log.Printf("Warning: Could not extract URL for event: %s", name)
		}
		
		// Check if row or cells have a gray background indicating cancellation
		isCanceled := false
		
		// Check row attribute
		bgColor, hasBgColor := row.Attr("bgcolor")
		if hasBgColor && (bgColor == "#D3D3D3" || bgColor == "#DEDEDE") {
			isCanceled = true
		}
		
		// Check each cell if needed
		if !isCanceled {
			cells.Each(func(i int, cell *goquery.Selection) {
				cellBg, hasCellBg := cell.Attr("bgcolor")
				if hasCellBg && (cellBg == "#D3D3D3" || cellBg == "#DEDEDE") {
					isCanceled = true
					return
				}
			})
		}
		
		event.IsCanceled = isCanceled
		
		if isCanceled {
			event.Status = "Canceled"
		} else {
			event.Status = "Scheduled"
		}

		// Extract date from the second column
		dateStr := strings.TrimSpace(cells.Eq(1).Text())
		parsedDate, err := extractDate(dateStr)
		if err != nil {
			log.Printf("Warning: Could not parse date for event %s: %v", name, err)
			// Still include the event but with zero date
		} else {
			event.Date = parsedDate
		}

		// Handle venue (which could be in a rowspan)
		var venue string
		venueIdx := 2
		
		// Check if we're in the middle of a venue rowspan
		if currentVenue != nil && currentVenue.RowsLeft > 0 {
			// Use the stored venue
			venue = currentVenue.Value
			currentVenue.RowsLeft--
		} else if cells.Length() > venueIdx {
			// Check if this cell has a rowspan
			venueCell := cells.Eq(venueIdx)
			venue = strings.TrimSpace(venueCell.Text())
			
			// If it has a rowspan attribute
			rowspanAttr, exists := venueCell.Attr("rowspan")
			if exists {
				rowspan := 1
				fmt.Sscanf(rowspanAttr, "%d", &rowspan)
				if rowspan > 1 {
					// Store this venue for future rows
					currentVenue = &RowspanData{
						Value:    venue,
						RowsLeft: rowspan - 1, // Subtract 1 because we're using it for this row
					}
				}
			}
		}
		
		if venue != "" && venue != "—" {
			event.Venue = venue
		}

		// Handle location (which could be in a rowspan)
		var location string
		locationIdx := 3
		
		// Check if we're in the middle of a location rowspan
		if currentLocation != nil && currentLocation.RowsLeft > 0 {
			// Use the stored location
			location = currentLocation.Value
			currentLocation.RowsLeft--
		} else if cells.Length() > locationIdx {
			// Check if this cell has a rowspan
			locationCell := cells.Eq(locationIdx)
			location = strings.TrimSpace(locationCell.Text())
			
			// If it has a rowspan attribute
			rowspanAttr, exists := locationCell.Attr("rowspan")
			if exists {
				rowspan := 1
				fmt.Sscanf(rowspanAttr, "%d", &rowspan)
				if rowspan > 1 {
					// Store this location for future rows
					currentLocation = &RowspanData{
						Value:    location,
						RowsLeft: rowspan - 1, // Subtract 1 because we're using it for this row
					}
				}
			}
		}
		
		// Process the location data
		if location != "" && location != "—" {
			locationParts := strings.Split(location, ", ")
			if len(locationParts) >= 2 {
				event.City = locationParts[0]
				event.Country = locationParts[len(locationParts)-1]
			} else if len(locationParts) == 1 {
				event.City = locationParts[0]
			}
		}

		// Extract attendance if available
		attendanceIdx := 4
		if cells.Length() > attendanceIdx {
			attendanceText := strings.TrimSpace(cells.Eq(attendanceIdx).Text())
			event.Attendance = attendanceText
			
			// If the attendance column contains "Canceled", mark the event as canceled
			if strings.Contains(strings.ToLower(attendanceText), "cancel") {
				event.Status = "Canceled"
				event.IsCanceled = true
			}
		}

		// Only add valid events with a name
		if event.Name != "" {
			events = append(events, &event)
		}
	})

	return events, nil
}

func (s *EventScraper) scrapePastEvents(doc *goquery.Document) ([]*WikiEvent, error) {
	var events []*WikiEvent
	
	// Find the table with id "Past_events"
	table := doc.Find("table#Past_events")
	if table.Length() == 0 {
		return nil, fmt.Errorf("past events table not found")
	}

	// Find all rows in the tbody
	rows := table.Find("tbody > tr")
	if rows.Length() == 0 {
		return nil, fmt.Errorf("no rows found in past events table")
	}
	
	log.Printf("Starting to scrape all past events from Wikipedia")
	
	var processedRows, includedRows int
	
	// Keep track of rowspan data
	type RowspanData struct {
		Value    string
		RowsLeft int
	}
	
	var currentVenue, currentLocation *RowspanData
	
	rows.Each(func(i int, row *goquery.Selection) {
		processedRows++
		
		// Skip rows without enough cells
		cells := row.Find("td")
		if cells.Length() < 3 {
			return
		}

		// Check if row or cells have a gray background indicating cancellation
		isCanceled := false
		
		// Check row attribute
		bgColor, hasBgColor := row.Attr("bgcolor")
		if hasBgColor && (bgColor == "#D3D3D3" || bgColor == "#DEDEDE") {
			isCanceled = true
		}
		
		// Check each cell if needed
		if !isCanceled {
			cells.Each(func(i int, cell *goquery.Selection) {
				cellBg, hasCellBg := cell.Attr("bgcolor")
				if hasCellBg && (cellBg == "#D3D3D3" || cellBg == "#DEDEDE") {
					isCanceled = true
					return
				}
			})
		}

		// Extract event name from the second column (index 1)
		nameCell := cells.Eq(1)
		name := strings.TrimSpace(nameCell.Text())
		if name == "" {
			return
		}
		
		// Create event
		var event WikiEvent
		event.Name = name
		event.IsCanceled = isCanceled
		
		// Extract Wiki URL from the name link
		nameLink := nameCell.Find("a[href]").First()
		if nameLink.Length() > 0 {
			if href, exists := nameLink.Attr("href"); exists && href != "" {
				// Make sure it's an absolute URL
				if strings.HasPrefix(href, "/wiki/") {
					event.WikiURL = "https://en.wikipedia.org" + href
				} else if strings.HasPrefix(href, "https://") {
					event.WikiURL = href
				}
			}
		}
		
		// Add debug logging to verify URL extraction
		if event.WikiURL == "" {
			log.Printf("Warning: Could not extract URL for past event: %s", name)
		}
		
		// Set status based on cancellation
		if isCanceled {
			event.Status = "Canceled"
		} else {
			event.Status = "Completed"
		}
		
		// Extract date - from the third column (index 2)
		dateStr := strings.TrimSpace(cells.Eq(2).Text())
		eventDate, err := extractDate(dateStr)
		if err != nil {
			log.Printf("Warning: Could not parse date for event %s: %v", name, err)
			// Still include the event but with zero date
		} else {
			event.Date = eventDate
		}
		
		// Handle venue (which could be in a rowspan)
		var venue string
		venueIdx := 3
		
		// Check if we're in the middle of a venue rowspan
		if currentVenue != nil && currentVenue.RowsLeft > 0 {
			// Use the stored venue
			venue = currentVenue.Value
			currentVenue.RowsLeft--
		} else if cells.Length() > venueIdx {
			// Check if this cell has a rowspan
			venueCell := cells.Eq(venueIdx)
			venue = strings.TrimSpace(venueCell.Text())
			
			// If it has a rowspan attribute
			rowspanAttr, exists := venueCell.Attr("rowspan")
			if exists {
				rowspan := 1
				fmt.Sscanf(rowspanAttr, "%d", &rowspan)
				if rowspan > 1 {
					// Store this venue for future rows
					currentVenue = &RowspanData{
						Value:    venue,
						RowsLeft: rowspan - 1, // Subtract 1 because we're using it for this row
					}
				}
			}
		}
		
		if venue != "" && venue != "—" {
			event.Venue = venue
		}
		
		// Handle location (which could be in a rowspan)
		var location string
		locationIdx := 4
		
		// Check if we're in the middle of a location rowspan
		if currentLocation != nil && currentLocation.RowsLeft > 0 {
			// Use the stored location
			location = currentLocation.Value
			currentLocation.RowsLeft--
		} else if cells.Length() > locationIdx {
			// Check if this cell has a rowspan
			locationCell := cells.Eq(locationIdx)
			location = strings.TrimSpace(locationCell.Text())
			
			// If it has a rowspan attribute
			rowspanAttr, exists := locationCell.Attr("rowspan")
			if exists {
				rowspan := 1
				fmt.Sscanf(rowspanAttr, "%d", &rowspan)
				if rowspan > 1 {
					// Store this location for future rows
					currentLocation = &RowspanData{
						Value:    location,
						RowsLeft: rowspan - 1, // Subtract 1 because we're using it for this row
					}
				}
			}
		}
		
		// Process the location data
		if location != "" && location != "—" {
			locationParts := strings.Split(location, ", ")
			if len(locationParts) >= 2 {
				event.City = locationParts[0]
				event.Country = locationParts[len(locationParts)-1]
			} else if len(locationParts) == 1 {
				event.City = locationParts[0]
			}
		}
		
		// Handle attendance
		attendanceIdx := 5
		if cells.Length() > attendanceIdx {
			attendanceText := strings.TrimSpace(cells.Eq(attendanceIdx).Text())
			event.Attendance = attendanceText
			
			// If the attendance column contains "Canceled", mark the event as canceled
			if strings.Contains(strings.ToLower(attendanceText), "cancel") {
				event.Status = "Canceled"
				event.IsCanceled = true
			}
		}
		
		// Only add valid events with a name
		if event.Name != "" {
			events = append(events, &event)
			includedRows++
		}
	})

	log.Printf("Past events: processed %d rows, included %d events", processedRows, includedRows)

	return events, nil
}

// Improved helper function to extract date from Wikipedia date format
func extractDate(dateStr string) (time.Time, error) {
	// Remove any HTML or extra characters
	dateStr = strings.TrimSpace(dateStr)
	
	// Extract date text from spans with data-sort-value
	re := regexp.MustCompile(`data-sort-value="[^"]*"[^>]*>([^<]+)</span>`)
	matches := re.FindStringSubmatch(dateStr)
	if len(matches) > 1 {
		dateStr = strings.TrimSpace(matches[1])
	}
	
	// Remove footnote references if present
	dateStr = regexp.MustCompile(`\[\d+\]`).ReplaceAllString(dateStr, "")
	dateStr = strings.TrimSpace(dateStr)
	
	// Try different date formats
	formats := []string{
		"Jan 2, 2006",
		"January 2, 2006",
		"Jan 2 2006",
		"January 2 2006",
		"Jan. 2, 2006",
		"January. 2, 2006",
	}
	
	for _, format := range formats {
		if parsedDate, err := time.Parse(format, dateStr); err == nil {
			return parsedDate, nil
		}
	}
	
	// If still not parsed, try to extract date with regex
	dateMatch := regexp.MustCompile(`\b(Jan|Feb|Mar|Apr|May|Jun|Jul|Aug|Sep|Oct|Nov|Dec)[\.a-z]*\s+\d{1,2},?\s+\d{4}\b`).FindString(dateStr)
	if dateMatch != "" {
		// Clean up the date string
		dateMatch = strings.ReplaceAll(dateMatch, ",", "")
		dateMatch = strings.ReplaceAll(dateMatch, ".", "")
		
		// Try parsing again
		for _, format := range []string{"Jan 2 2006", "January 2 2006"} {
			if parsedDate, err := time.Parse(format, dateMatch); err == nil {
				return parsedDate, nil
			}
		}
	}
	
	return time.Time{}, fmt.Errorf("could not parse date: %s", dateStr)
}

func (s *EventScraper) ScrapeUpcomingEvents(ctx context.Context) ([]*WikiEvent, error) {
	req, err := http.NewRequest("GET", s.wikiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %v", err)
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) Firefox/123.0")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error fetching page: %v", err)
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error parsing HTML: %v", err)
	}

	return s.scrapeScheduledEvents(doc)
}