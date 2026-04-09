package main

import (
	"Lead-Automation-Pipeline/cmd/utils"
	"bufio"
	"encoding/csv"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	htmltomarkdown "github.com/JohannesKaufmann/html-to-markdown/v2"
	"github.com/joho/godotenv"
	"golang.org/x/net/html"
)

type SafeSeenLeadsWebsites struct {
	mu sync.Mutex
	websites []string
}

type SafeAllProcessedLeads struct {
	mu sync.Mutex
	leads []utils.Lead
}

// func main() {
// 	godotenv.Load()
// 	var start = time.Now()
//
// 	// Ensures the user passes in the queries file and the output file
// 	var args = os.Args
// 	if len(args) < 3 || len(args) > 4 {
// 		fmt.Errorf("Usage: go run . <keywords.txt> <output.csv> [optional_recovery_file.csv]")
// 	}
//
// 	// The websites of all the leads whos been seen so far
// 	var seenLeadsWebsites = SafeSeenLeadsWebsites{
// 		websites: []string{},
// 	}
// 	var allProcessedLeads = SafeAllProcessedLeads{}
//
// 	if len(args) == 4 {
// 		recoveryFileName := args[3]
// 		fmt.Println("Attempting to recover seen websites from:", recoveryFileName)
//
// 		recoveredWebsites, err := utils.LoadWebsitesFromRecoveryCSV(recoveryFileName)
// 		if err != nil {
// 			fmt.Errorf("Error loading recovery file: %v", err)
// 		}
//
// 		// Pre-populate the slice with the data from your old CSV.
// 		seenLeadsWebsites.websites = recoveredWebsites
// 		fmt.Printf("✅ Successfully recovered %d websites.\n", len(seenLeadsWebsites.websites))
// 	}
//
// 	fmt.Println("Splitting keywords into their own file...")
// 	// Split keywords into their own files
// 	var keywordFiles = splitKeywordsUp(args[1])
// 	err := os.Mkdir("temp/processedLeads/", 0755)
// 	if err != nil {
// 		fmt.Errorf("Failed to create the processedLeads directory. Recieved error:", err)
// 	}
//
// 	// Loop over the files, calling the runPipeline function()
// 	for _, keywordFile := range keywordFiles {
// 		var scraperOutputFile = strings.Replace(keywordFile, ".txt", ".csv", -1)
// 		var pipelineOutputFile = strings.Replace(scraperOutputFile, "temp/", "temp/processedLeads/", -1)
//
// 		fmt.Println("Running scraper on", keywordFile + "...")
// 		runScraper(keywordFile, scraperOutputFile)
//
// 		fmt.Println("Running the pipeline", keywordFile + "...")
// 		runPipeline(scraperOutputFile, pipelineOutputFile, &seenLeadsWebsites, &allProcessedLeads)
// 		fmt.Println("Finished the pipeline for", keywordFile + "!")
// 	}
//
// 	fmt.Println("Saving final results to", args[2] + "...")
// 	// Saves all the leads
// 	saveToCSVFile(allProcessedLeads.leads, args[2])
// 	fmt.Println("Complete!")
// 	var duration = time.Since(start)
// 	fmt.Println("Time elapsed:", duration)
// }

func main() {
	godotenv.Load()
	var start = time.Now()

	// 1. CHECK ARGUMENTS
	// We now expect two arguments: the input queries file and the output file.
	var args = os.Args
	if len(args) != 3 {
		fmt.Println("Usage: go run . <queries.txt> <output.csv>")
		return
	}
	queriesFileName := args[1]
	outputFileName := args[2]

	// Ensure the temp directory exists
	err := os.MkdirAll("temp", 0755)
	if err != nil {
		fmt.Println("Failed to create the temp directory. Received error:", err)
	}

	scraperOutputFileName := fmt.Sprintf("temp/temp_scraper_results_%d.csv", time.Now().Unix())

	fmt.Println("Running scraper on queries from:", queriesFileName)
	err = runScraper(queriesFileName, scraperOutputFileName)
	if err != nil {
		fmt.Println("Error running scraper:", err)
		return
	}

	fmt.Println("Reading leads from:", scraperOutputFileName)

	// The websites of all the leads who have been seen so far
	var seenLeadsWebsites = SafeSeenLeadsWebsites{
		websites: []string{},
	}
	var allProcessedLeads = SafeAllProcessedLeads{}

	// 2. READ LEADS FROM YOUR CSV
	// This function must correctly parse your CSV into a []utils.Lead slice.
	var leads = utils.ReadCSV(scraperOutputFileName)
	if len(leads) == 0 {
		fmt.Println("No leads found from the scraper. Exiting.")
		return
	}
	fmt.Printf("Found %d leads to process.\n", len(leads))


	// 3. PROCESS THE LEADS CONCURRENTLY
	var wg sync.WaitGroup
	var numOfGoroutines = 30
	client := http.Client{
		Timeout: 10 * time.Second,
	}

	var leadChunks = splitLeadsUp(leads, numOfGoroutines)

	fmt.Println("Starting lead processing...")
	// Starts a goroutine for each chunk
	for _, chunk := range leadChunks {
		wg.Add(1)

		go func(c []utils.Lead) {
			defer wg.Done()

			for _, lead := range c {
				processedLead, err := processLead(lead, &seenLeadsWebsites, &client)
				if err != nil {
					fmt.Printf("Error processing %s: %v\n", lead.CompanyName, err)
					continue
				}

				// Add the successfully processed lead to the final list
				if processedLead.Email != "" {
					allProcessedLeads.mu.Lock()
					allProcessedLeads.leads = append(allProcessedLeads.leads, processedLead)
					allProcessedLeads.mu.Unlock()
				}
			}
		}(chunk)
	}

	wg.Wait() // Wait for all goroutines to finish

	// 4. SAVE FINAL RESULTS
	fmt.Println("Saving final results to", outputFileName+"...")
	saveToCSVFile(allProcessedLeads.leads, outputFileName)

	fmt.Println("Complete!")
	var duration = time.Since(start)
	fmt.Println("Time elapsed:", duration)
}

func processLead(lead utils.Lead, seenLeadsWebsites *SafeSeenLeadsWebsites, client *http.Client) (utils.Lead, error) {
	var start = time.Now()

	// Checks if the lead has been processed before
	seenLeadsWebsites.mu.Lock()
	if slices.Contains(seenLeadsWebsites.websites, lead.Website) {
		seenLeadsWebsites.mu.Unlock()
		fmt.Println("Skipping website processed in a previous run:", lead.Website)
		return utils.Lead{}, nil
	}

	// Adds the lead to the seenLeadsWebsites slice
	seenLeadsWebsites.websites = append(seenLeadsWebsites.websites, lead.Website)
	seenLeadsWebsites.mu.Unlock()

	fmt.Println("Getting the emails and markdown for:", lead.CompanyName)
	// Scrapes the website for emails, and copies the HTML

	emails, markdown, err := getEmailsAndMarkdown(lead.Website, client)
	if err != nil {
		fmt.Println("Error getting the emails and markdown. Recieved error:", err)
	}

	if len(emails) == 0 {
		fmt.Println("Found no emails for:", lead.CompanyName)
		return utils.Lead{}, nil
	}

	fmt.Println("Got the emails and markdown for", lead.CompanyName)
	lead.Email = emails[0]

	// Calls OpenAI to write a summarisation of each of the websites' pages
	println("Generating abstracts for", lead.CompanyName)
	var abstracts = utils.GenerateAbstracts(markdown)

	println("Generating an icebreaker for", lead.CompanyName)
	// Calls OpenAI to write an email icebreaker
	var icebreaker = utils.GenerateIcebreaker(abstracts)
	lead.Icebreaker = icebreaker

	var duration = time.Since(start)
	fmt.Println(lead.CompanyName + " took: " + duration.String())
	return lead, nil
}

// Splits the lead array up into multiple chunks
func splitLeadsUp(leads []utils.Lead, numOfChunks int) [][]utils.Lead {
    var leadChunks [][]utils.Lead
    total := len(leads)

    if numOfChunks <= 0 {
        return nil // or return the whole slice as one chunk
    }
    if numOfChunks > total {
        numOfChunks = total
    }

    chunkSize := total / numOfChunks
    remainder := total % numOfChunks

    start := 0
    for i := 0; i < numOfChunks; i++ {
        // Add 1 to the chunk size for the first 'remainder' chunks to spread leftovers evenly
        size := chunkSize
        if i < remainder {
            size++
        }
        end := start + size
        leadChunks = append(leadChunks, leads[start:end])
        start = end
    }

    return leadChunks
}

func getEmailsAndMarkdown(website string, client *http.Client) ([]string, []string, error) {
	
	var emails = []string{}
	var markdown = []string{}

	// Gets the homepage's HTML
	var homepageHTML, err = getWebsiteHTML(website, client)
	if err != nil {
		fmt.Println("Failed to get homepage's HTML:", err)
	}

	emails = append(emails, getEmailsFromHTML(homepageHTML, website)...)
	markdown = append(markdown, convertHTMLToMarkdown(homepageHTML))

	// Gets 10 internal links from the website (exluding blog and post pages)
	links, err := getInternalLinks(website, homepageHTML)
	if err != err {
		fmt.Println("Failed to get internal links:", err)
	}

	for _, link := range links {
		var pageHTML, err = getWebsiteHTML(link, client); if err != nil {
			fmt.Printf("Failed to get page to %s. Recieved error: %v\n", link, err)
		}

		emails = append(emails, getEmailsFromHTML(pageHTML, link)...)
		markdown = append(markdown, convertHTMLToMarkdown(pageHTML))
	}

	return emails, markdown, nil
}

func getWebsiteHTML(url string, client *http.Client) (string, error) {
	// Sends the response to the website
	resp, err := client.Get(url)
	if err != nil {
		return "", fmt.Errorf("Failed to make request: %w", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Non 200 HTTP Response: %w", err)
	}

	// Gets the HTML from the response
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("Failed to read response to body: %w", err)
	}

	return string(bodyBytes), nil
}

func getInternalLinks(homepageURL string, htmlContent string) ([]string, error) {
	var links = []string{}

	baseURL, err := url.Parse(homepageURL)
	if err != nil {
		fmt.Println("Invalid base URL:", err)
	}

	doc, err := html.Parse(strings.NewReader(htmlContent))
	if err != nil {
		return nil, fmt.Errorf("HTML parse error: %w", err)
	}

	// Traverses the HTML
	var traverse func(*html.Node)
	traverse = func(node *html.Node) {
		if node.Type == html.ElementNode && node.Data == "a" {
			for _, attr := range node.Attr {
				if attr.Key == "href" {
					href := strings.TrimSpace(attr.Val)
					if href == "" || strings.HasPrefix(href, "mailto:") || strings.HasPrefix(href, "javascript:") || strings.HasPrefix(href, "tel:") {
						continue
					}

					parsed, err := url.Parse(href)
					if err != nil {
						continue
					}

					resolved := baseURL.ResolveReference(parsed)

					if resolved.Host == baseURL.Host {
						links = append(links, resolved.String())
					}
				}
			}
		}

		for child := node.FirstChild; child != nil; child = child.NextSibling {
			if len(links) <= 10 {
				traverse(child)
			} else {
				break
			}
		}
	}
	traverse(doc)
	return links, nil
}

func getEmailsFromHTML(html string, websiteURL string) []string {
	var emailRegex = `[a-z0-9.\-+]+@[a-z0-9.\-+]+\.[a-z]+`
	
	baseURL, err := url.Parse(websiteURL)
	if err != nil {
		fmt.Println("Invalid base URL:", err)
	}

	var domain = baseURL.Hostname()

	// Finds the emails
	re := regexp.MustCompile(emailRegex)
	matches := re.FindAllString(html, -1)

	// Removes duplicates
	unique := make(map[string]bool)
	var result []string

	for _, email := range matches {
		if !unique[email] {
			unique[email] = true

			// Ensures the email is either using the domain, or is part of a common mail provider
			if strings.Contains(email, strings.Trim(domain, "www.")) || isDomainFromCommonProvider(email) {
				result = append(result, email)
			}
		}
	}

	return result
}

func convertHTMLToMarkdown(html string) string {
	markdown,err := htmltomarkdown.ConvertString(html)
	if err != nil {
		fmt.Println("Couldn't convert HTML to markdown. Recieved error:", err)
	}

	// Limits the markdown to 10,000 characters to not use too many tokens
	if len(markdown) > 10000 {
		return markdown[:10000]
	}
	return markdown
}

func isDomainFromCommonProvider(email string) bool {
	var commonMailProviders = []string {
		"gmail.com", "yahoo.com", "outlook.com", "hotmail.com", "icloud.com",
		"btinternet.com", "btconnect.com", "gmail.co.uk", "yahoo.co.uk",
		"outlook.co.uk", "hotmail.co.uk", "icloud.co.uk", "btinternet.co.uk",
		"btconnect.co.uk", "live.co.uk", "live.com", "mail.com", "mail.co.uk",
		"aol.com", "aol.co.uk",
	}

	for _, provider := range commonMailProviders {
		if strings.Contains(email, provider) {
			return true
		}

	}

	return false
}

func saveToCSVFile(processedLeads []utils.Lead, outputFileName string) error {
	file, err := os.Create(outputFileName)
	if err != nil {
		fmt.Println("Couldn't create output file. Recieved error:", err)
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Writes the headers
	writer.Write([]string{"Company Name", "First Name", "Email", "Website", "Phone Number", "Icebreaker"})

	// Writes each lead as a row
	for _, lead := range processedLeads {
		var row = []string{lead.CompanyName, lead.FirstName, lead.Email, lead.Website, lead.PhoneNumber, lead.Icebreaker}
		writer.Write(row)
	}

	return nil
}

func splitKeywordsUp(keywordsFileName string) []string {
	// Gets the epoch time
	var epochTime = time.Now().Unix()
	var fileNames = []string{}

	err := os.Mkdir("temp", 0755)
	if err != nil {
		fmt.Println("Failed to create the temp directory. Recieved error:", err)
	}

	file, err := os.Open(keywordsFileName)
	if err != nil {
		fmt.Println("Failed to keywords file. Recieved error:", err)
	}
	defer file.Close()

	// Scans the file
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		filename := "temp/" + line + "_" + strconv.Itoa(int(epochTime)) + ".txt"
		filename = strings.Replace(filename, " ", "_", -1)

		err := os.WriteFile(filename, []byte(line), 0644)
		if err != nil {
			fmt.Println("Failed to create keyword file", filename + ". Recieved error:", err)
			continue
		}

		fileNames = append(fileNames, filename)
	}
	
	if err := scanner.Err(); err != nil {
		fmt.Println("Error when reading keywords file. Recieved error:", err)
	}

	return fileNames
}

func runScraper(keywordFileName string, outputFileName string) error {
	// output, err := exec.Command("google-maps-scraper", "-input", keywordFileName, "-results", outputFileName, "-c", "10", "-exit-on-inactivity", "3m").CombinedOutput()
	//
	// stdout, err := cmd.StdoutPipe()
	//
	//    if len(outputAsString) > 0 {
	//        fmt.Println("--- Scraper Live Output ---")
	//        fmt.Println(outputAsString)
	//        fmt.Println("--------------------------")
	//    }
	//
	// if err != nil {
	// 	fmt.Println(err.Error())
	// 	fmt.Errorf("scraper error: %v\nOutput:\n%s", err)
	// }
	//
	// return nil

	// 1. Create the command object with its arguments.
	cmd := exec.Command("google-maps-scraper",
		"-input", keywordFileName,
		"-results", outputFileName,
		"-c", "10",
		"-exit-on-inactivity", "1m",
	)

	// 2. Create pipes to capture the command's stdout and stderr.
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("error creating stdout pipe: %v", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("error creating stderr pipe: %v", err)
	}

	// 3. Start the command. This runs it in the background.
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("error starting command: %v", err)
	}

	fmt.Println("--- Scraper Live Output ---")

	// 4. Create a scanner to read the output from both pipes line by line.
	// We use a goroutine to avoid blocking on one pipe while the other has output.
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			fmt.Println(scanner.Text())
		}
	}()

	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			// Print errors to standard error for good practice.
			fmt.Fprintln(os.Stderr, scanner.Text())
		}
	}()

	// 5. Wait for the command to finish.
	// This will block until the scraper process exits.
	err = cmd.Wait()

	fmt.Println("--------------------------")

	if err != nil {
		// The error from cmd.Wait() will often be of type *exec.ExitError,
		// which can provide more details about the exit status.
		return fmt.Errorf("scraper command finished with error: %v", err)
	}

	fmt.Println("Scraper finished successfully.")
	return nil
}


func runPipeline(keywordFileName string, outputFileName string, seenLeadsWebsites *SafeSeenLeadsWebsites, allProcessedLeads *SafeAllProcessedLeads) {
	// Reads the email, phone number, and website from the results
	var leads = utils.ReadCSV(keywordFileName)
	if len(leads) == 0 {
		return
	}

	var wg sync.WaitGroup
	// var numOfGoroutines = 5
	var numOfGoroutines = 130
	client := http.Client {
		Timeout: 10 * time.Second,
	}

	// Splits the leads into chunks, one chunk per goroutine
	var leadChunks = splitLeadsUp(leads, numOfGoroutines)
	var keywordProcessedLeads SafeAllProcessedLeads

	// Starts a goroutine for each chunk
	for _, chunk := range leadChunks {
		wg.Add(1)

		// This anonymous function stops you from needing
		// to loop over the chunk inside the processLead() function
		go func(c []utils.Lead) {
			defer wg.Done()

			for _, lead := range chunk {
				processedLead, err := processLead(lead, seenLeadsWebsites, &client)
				if err != nil {
					fmt.Println("Error processing", lead.CompanyName, "Recieved Error:", err)
					continue
				} 

				if processedLead.Email != "" {
					allProcessedLeads.mu.Lock()
					allProcessedLeads.leads = append(allProcessedLeads.leads, processedLead)
					allProcessedLeads.mu.Unlock()

					keywordProcessedLeads.mu.Lock()
					keywordProcessedLeads.leads = append(keywordProcessedLeads.leads, processedLead)
					keywordProcessedLeads.mu.Unlock()
				}
			}
		}(chunk)
	}

	wg.Wait()

	fmt.Println("Saving results to the CSV...")
	// Saves the final results in the output file specified by the user
	saveToCSVFile(keywordProcessedLeads.leads, outputFileName)
	fmt.Println("Complete!")
}
