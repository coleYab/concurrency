package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"sync"
	"time"
)

// Constants
const (
	concurrencyLimit = 500  // Reduced concurrency limit
	maxRetries       = 3    // Maximum retries for failed requests
	requestTimeout   = 30 * time.Second // Increased timeout
)

// Pokemon represents the structure of the Pokémon API response
type Pokemon struct {
	Name   string `json:"name"`
	Height int    `json:"height"`
	Weight int    `json:"weight"`
}

// fetchPokemon fetches data for a specific Pokémon by ID
func fetchPokemon(id int, wg *sync.WaitGroup, semaphore chan struct{}, results chan<- Pokemon, activeRequests *int, mutex *sync.Mutex, client *http.Client) {
	defer wg.Done()

	// Acquire a slot in the semaphore (limits concurrency)
	semaphore <- struct{}{}

	// Increment the active request counter
	mutex.Lock()
	*activeRequests++
	log.Printf("Making request for Pokémon ID: %d (Active requests: %d)", id, *activeRequests)
	mutex.Unlock()

	defer func() {
		// Decrement the active request counter
		mutex.Lock()
		*activeRequests--
		log.Printf("Finished request for Pokémon ID: %d (Active requests: %d)", id, *activeRequests)
		mutex.Unlock()

		// Release the semaphore slot
		<-semaphore
	}()

	// Retry logic with exponential backoff
	for retry := 0; retry <= maxRetries; retry++ {
		url := fmt.Sprintf("https://pokeapi.co/api/v2/pokemon/%d", id)
		resp, err := client.Get(url)
		if err != nil {
			log.Printf("Error fetching Pokémon %d (attempt %d): %v", id, retry+1, err)
			if retry == maxRetries {
				log.Printf("Max retries reached for Pokémon ID: %d", id)
				return
			}
			backoff := time.Duration(retry+1) * time.Second // Exponential backoff
			time.Sleep(backoff)
			continue
		}
		defer resp.Body.Close()

		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			log.Printf("Error reading response for Pokémon %d (attempt %d): %v", id, retry+1, err)
			if retry == maxRetries {
				log.Printf("Max retries reached for Pokémon ID: %d", id)
				return
			}
			backoff := time.Duration(retry+1) * time.Second // Exponential backoff
			time.Sleep(backoff)
			continue
		}

		var pokemon Pokemon
		if err := json.Unmarshal(body, &pokemon); err != nil {
			log.Printf("Error decoding JSON for Pokémon %d (attempt %d): %v", id, retry+1, err)
			if retry == maxRetries {
				log.Printf("Max retries reached for Pokémon ID: %d", id)
				return
			}
			backoff := time.Duration(retry+1) * time.Second // Exponential backoff
			time.Sleep(backoff)
			continue
		}

		// Send the result to the results channel
		results <- pokemon
		log.Printf("Successfully fetched Pokémon ID: %d", id)
		return
	}
}

func main() {
    start := time.Now()
	// Generate a list of 1000 Pokémon IDs (1 to 1000)
	pokemonIDs := make([]int, 1000)
	for i := 0; i < 1000; i++ {
		pokemonIDs[i] = i + 1
	}

	// Channel to collect results
	results := make(chan Pokemon, len(pokemonIDs))

	// Semaphore to limit concurrency
	semaphore := make(chan struct{}, concurrencyLimit)

	// WaitGroup to wait for all goroutines to finish
	var wg sync.WaitGroup

	// Track active requests
	activeRequests := 0
	var mutex sync.Mutex

	// Custom HTTP client with connection pooling and timeouts
	transport := &http.Transport{
		MaxIdleConns:        concurrencyLimit,
		MaxIdleConnsPerHost: concurrencyLimit,
	}
	client := &http.Client{
		Transport: transport,
		Timeout:   requestTimeout,
	}

	// Rate limiter to avoid overwhelming the API
	rateLimiter := time.Tick(10 * time.Millisecond) // Allow 10 requests per second

	// Start a goroutine for each Pokémon ID
	for _, id := range pokemonIDs {
		wg.Add(1)
		<-rateLimiter // Wait for the rate limiter
		go fetchPokemon(id, &wg, semaphore, results, &activeRequests, &mutex, client)
	}

	// Wait for all goroutines to finish
	wg.Wait()
	close(results) // Close the results channel

	// Collect and print the results
    i := 0
	for pokemon := range results {
		fmt.Printf("Name: %s, Height: %d, Weight: %d\n", pokemon.Name, pokemon.Height, pokemon.Weight)
        i += 1
	}

    fmt.Printf("Total recived data was: %v, in %v seconds", i + 1, time.Since(start))
}
