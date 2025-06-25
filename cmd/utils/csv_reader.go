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
	PhoneNumber string
	Website string
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

	return slices.Delete(getRequiredColumns(records), 0, 1) // Deletes the header
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
	if websiteLink == "" {
		return false
	}

	var invalid_domains = []string{
		"facebook.com", "fb.com", "instagram.com", "twitter.com",
		"linkedin.com", "tiktok.com", "youtube.com",
		"facebook.co.uk", "fb.co.uk", "instagram.co.uk", "twitter.co.uk",
		"linkedin.co.uk", "tiktok.co.uk", "youtube.co.uk", "fresha.com",
		"fresha.co.uk",
	}

	
	for _, domain := range invalid_domains {
		if strings.Contains(strings.ToLower(websiteLink), domain) {
			return false
		}
	}

	return true
}
