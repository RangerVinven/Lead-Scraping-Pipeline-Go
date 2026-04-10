package utils

import (
	"encoding/csv"
	"log"
	"os"
	"slices"
	"strings"
)

type Lead struct {
	CompanyName string
	FirstName string
	Email string
	Website string
	PhoneNumber string
	Icebreaker string
}

func ReadCSV(fileName string) []Lead {
	scraperOutput, err := os.Open(fileName)
	if err != nil {
		log.Fatal("Couldn't read the output.csv file. Recieved error: %w", err)
	}
	defer scraperOutput.Close()

	csvReader := csv.NewReader(scraperOutput)
	records, err := csvReader.ReadAll()
	if err != nil {
		log.Fatal("Couldn't parse the output.csv file. Recieved error: %w", err)
	}

	leads := getRequiredColumns(records)
	if len(leads) == 0 {
		return leads
	}
	
	return slices.Delete(leads, 0, 1) // Deletes the header
}

// Gets the company's name, phone number, and website (only returns results with websites)
func getRequiredColumns(scraperResults [][]string) []Lead {
	var leads []Lead

	// Loops over the scraperResults (from the csv file)
	for _, lead := range scraperResults {
		if hasValidWebsite(lead[7]) {
			leads = append(leads, Lead{
				CompanyName: lead[2],
				PhoneNumber: lead[8],
				Website: lead[7],
			})
		}
	}

	return leads
}

func hasValidWebsite(websiteLink string) bool {
	if websiteLink == "{}" {
		return false
	}

	var invalid_domains = []string{
		// Social Media
		"facebook.com", "fb.com", "instagram.com", "twitter.com", "x.com",
		"linkedin.com", "tiktok.com", "youtube.com", "pinterest.com",
		"facebook.co.uk", "fb.co.uk", "instagram.co.uk", "twitter.co.uk",
		"linkedin.co.uk", "tiktok.co.uk", "youtube.co.uk",

		// Major Business Directories
		"yell.com", "yelp.com", "yelp.co.uk", "thomsonlocal.com",
		"cylex-uk.co.uk", "cylex.com", "192.com", "freeindex.co.uk",
		"scoot.co.uk", "centralindex.com", "businessmagnet.co.uk",
		"lacartes.com", "showmelocal.com", "brownbook.net",
		"hotfrog.co.uk", "hotfrog.com", "europages.co.uk",
		"kompass.com", "bark.com", "gumtree.com", "misterwhat.co.uk",
		"approvedbusiness.co.uk", "finder.com", "localdatacompany.com",
		"near.co.uk", "118118.com", "localstore.co.uk", "allbiz.co.uk",
		"nearest.com", "local-life.co.uk",

		// Trade/Niche Directories
		"checkatrade.com", "trustatrader.com", "mybuilder.com",
		"ratedpeople.com", "houzz.co.uk", "houzz.com", "directory.thesun.co.uk",
		"directory.independent.co.uk", "directory.mirror.co.uk",

		// Booking/Review Portals
		"fresha.com", "fresha.co.uk", "phorest.com", "treatwell.co.uk",
		"tripadvisor.com", "tripadvisor.co.uk", "booking.com", "expedia.co.uk",
		"zomato.com", "just-eat.co.uk", "deliveroo.co.uk", "ubereats.com",
		"opentable.co.uk",
	}

	
	for _, domain := range invalid_domains {
		if strings.Contains(strings.ToLower(websiteLink), domain) {
			return false
		}
	}

	return true
}

// In package utils

// LoadWebsitesFromRecoveryCSV reads a CSV file and returns a slice containing
// the data from the 4th column (index 3), which is expected to be the website URL.
func LoadWebsitesFromRecoveryCSV(fileName string) ([]string, error) {
	// Attempt to open the specified recovery file.
	file, err := os.Open(fileName)
	if err != nil {
		// If the file simply doesn't exist, it's not an error.
		// It just means we have no state to recover, so we return an empty slice.
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		// For any other error (e.g., permissions), return the error.
		return nil, err
	}
	defer file.Close()

	// Read all the records from the CSV.
	csvReader := csv.NewReader(file)
	records, err := csvReader.ReadAll()
	if err != nil {
		return nil, err
	}

	var websites []string
	if len(records) < 2 {
		// If the file is empty or only has a header, there's nothing to load.
		return websites, nil
	}

	// Loop over the records, skipping the header row (index 0).
	for _, record := range records[1:] {
		// Ensure the row has enough columns to prevent a crash. The 4th column is index 3.
		if len(record) > 3 {
			website := record[3]
			// Add the website to our slice, as long as it's not an empty string.
			if website != "" {
				websites = append(websites, website)
			}
		}
	}

	return websites, nil
}
