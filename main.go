package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"

	"github.com/PuerkitoBio/goquery"
)

// TestCase sample input/output
type Sample struct {
	Input  string
	Output string
}

func FetchSampleCases(contestID, problemID string) ([]Sample, error) {
	url := fmt.Sprintf("https://atcoder.jp/contests/%s/tasks/%s_%s", contestID, contestID, problemID)

	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch problem page: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return nil, fmt.Errorf("problem %s not found (404)", problemID)
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("unexpected status code %d", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML: %w", err)
	}

	var samples []Sample
	var current Sample

	doc.Find("section").Each(func(i int, s *goquery.Selection) {
		heading := s.Find("h3").Text()
		text := s.Find("pre").Text()

		if strings.HasPrefix(heading, "Sample Input") {
			current = Sample{Input: text}
		} else if strings.HasPrefix(heading, "Sample Output") {
			current.Output = text
			samples = append(samples, current)
		}
	})

	return samples, nil
}

func SaveSamples(samples []Sample, folder string) error {
	if err := os.MkdirAll(folder, 0755); err != nil {
		return fmt.Errorf("failed to create folder %s: %w", folder, err)
	}

	for i, sample := range samples {
		inFile := fmt.Sprintf("%s/sample%d.in.txt", folder, i+1)
		outFile := fmt.Sprintf("%s/sample%d.out.txt", folder, i+1)

		if err := os.WriteFile(inFile, []byte(sample.Input), 0644); err != nil {
			return err
		}
		if err := os.WriteFile(outFile, []byte(sample.Output), 0644); err != nil {
			return err
		}
	}

	return nil
}

func GetMostRecentContestID() (string, error) {
	// Fetch the AtCoder Contest Archive page
	resp, err := http.Get("https://atcoder.jp/contests/archive")
	if err != nil {
		return "", fmt.Errorf("failed to fetch contests archive page: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to fetch contests archive, status: %d", resp.StatusCode)
	}

	// Parse the HTML document
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to parse contests archive page: %w", err)
	}

	// Iterate over each row in the contests table
	var contestID string
	doc.Find("table tbody tr").EachWithBreak(func(i int, s *goquery.Selection) bool {
		// Find the link to the contest
		link := s.Find("a").Eq(1).AttrOr("href", "")
		fmt.Println(link)
		if strings.HasPrefix(link, "/contests/") {
			id := strings.TrimPrefix(link, "/contests/")
			if strings.HasPrefix(id, "abc") {
				contestID = id
				return false
			}
		}
		return true
	})

	if contestID == "" {
		return "", fmt.Errorf("no AtCoder Beginner Contest found in the archive")
	}

	return contestID, nil
}

func main() {
	contestID := flag.String("contest", "", "AtCoder contest ID (e.g., abc349)")
	problems := flag.String("range", "a-g", "Problem range (e.g., a-d)")
	concurrency := flag.Int("concurrency", 4, "Max number of concurrent fetches")
	flag.Parse()

	if *contestID == "" {
		fmt.Println("🕵️  Detecting the most recent AtCoder Beginner Contest (ABC)...")
		id, err := GetMostRecentContestID()
		if err != nil {
			log.Fatalf("Failed to detect the most recent ABC contest: %v", err)
		}
		*contestID = id
		fmt.Printf("📦 Using the most recent ABC contest: %s\n", *contestID)
	}

	start := rune((*problems)[0])
	end := rune((*problems)[len(*problems)-1])

	var wg sync.WaitGroup
	sem := make(chan struct{}, *concurrency)

	for ch := start; ch <= end; ch++ {
		problemID := string(ch)
		wg.Add(1)
		go func(problemID string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			fmt.Printf("🔍 Fetching problem %s_%s...\n", *contestID, problemID)

			samples, err := FetchSampleCases(*contestID, problemID)
			if err != nil {
				fmt.Printf("⚠️  Skipping %s: %v\n", problemID, err)
				return
			}

			outputFolder := fmt.Sprintf("testcases/%s", problemID)
			if err := SaveSamples(samples, outputFolder); err != nil {
				fmt.Printf("❌ Failed to save %s: %v\n", problemID, err)
			} else {
				fmt.Printf("✅ Saved %d samples to %s/\n", len(samples), outputFolder)
			}
		}(problemID)
	}

	wg.Wait()
	fmt.Println("🎉 All test cases fetched.")
}
