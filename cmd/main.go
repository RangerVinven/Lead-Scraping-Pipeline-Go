package main

import (
	"Lead-Automation-Pipeline/cmd/utils"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"sync"
	"time"
	"regexp"

	"golang.org/x/net/html"
	htmltomarkdown "github.com/JohannesKaufmann/html-to-markdown/v2"
)

type SafeSeenLeadsWebsites struct {
	mu sync.Mutex
	websites []string
}

func main() {
	// The websites of all the leads whos been seen so far
	var seenLeadsWebsites = SafeSeenLeadsWebsites{}

	// Runs the google maps scraper

	// Reads the email, phone number, and website from the results
	var leads = utils.ReadCSV("output.csv")

	// Checks whether any of the clients have already been scraped
	for i := 0; i < len(leads); i++ {
		if slices.Contains(seenLeadsWebsites.websites, leads[i].Website) {
			leads = append(leads[:i], leads[i+1:]...)
			i--
		}
	}

	var wg sync.WaitGroup
	var numOfGoroutines = 5
	client := http.Client {
		Timeout: 10 * time.Second,
	}

	// Splits the leads into chunks, one chunk per goroutine
	var leadChunks = splitLeadsUp(leads, numOfGoroutines)

	// Starts a goroutine for each chunk
	for _, chunk := range leadChunks {
		wg.Add(1)

		// This anonymous function stops you from needing
		// to loop over the chunk inside the processLead() function
		go func(c []utils.Lead) {
			defer wg.Done()

			for _, lead := range chunk {
				processLead(lead, &seenLeadsWebsites, &client)
			}
		}(chunk)
	}

	wg.Wait()

	// Saves the final results in the output file specified by the user
}

func processLead(lead utils.Lead, seenLeadsWebsites *SafeSeenLeadsWebsites, client *http.Client) {
	// Scrapes the website for emails, and copies the HTML
	// emails, markdown, err := getEmailsAndMarkdown(lead.Website, client)
	getEmailsAndMarkdown(lead.Website, client)
	// if err != nil {
	// 	fmt.Println("Error getting the emails and markdown. Recieved error: %w", err)
	// }

	// Calls OpenAI to write a summarisation of the website's content

	// Calls OpenAI to write an email icebreaker

	// Save the email to the array used to check if a lead has been scraped before
	seenLeadsWebsites.mu.Lock()
	seenLeadsWebsites.websites = append(seenLeadsWebsites.websites, lead.Website)
	seenLeadsWebsites.mu.Unlock()
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
		fmt.Errorf("Failed to get homepage's HTML: %w", err)
	}

	emails = append(emails, getEmailsFromHTML(homepageHTML, website)...)
	markdown = append(markdown, convertHTMLToMarkdown(homepageHTML))

	// Gets 10 internal links from the website (exluding blog and post pages)
	links, err := getInternalLinks(website, homepageHTML)
	if err != err {
		fmt.Errorf("Failed to get internal links: %w", err)
	}

	for _, link := range links {
		var pageHTML, err = getWebsiteHTML(link, client); if err != nil {
			fmt.Errorf("Failed to get page to %w. Recieved error: %w", link, err)
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
		fmt.Errorf("Invalid base URL: %w", err)
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
		fmt.Errorf("Invalid base URL: %w", err)
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
	markdown, err := htmltomarkdown.ConvertString(html)
	if err != nil {
		fmt.Errorf("Couldn't convert HTML to markdown. Recieved error: %w", err)
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

		// fmt.Println("%w is not in %w", provider, email)
	}

	return false
}
