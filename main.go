package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"time"
)

// BrasilAPICEP represents the structure returned by BrasilAPI
type BrasilAPICEP struct {
	Cep          string `json:"cep"`
	State        string `json:"state"`
	City         string `json:"city"`
	Neighborhood string `json:"neighborhood"`
	Street       string `json:"street"`
	Service      string `json:"service"`
}

// ViaCEP represents the structure returned by ViaCEP API
type ViaCEP struct {
	Cep         string `json:"cep"`
	Logradouro  string `json:"logradouro"`
	Complemento string `json:"complemento"`
	Bairro      string `json:"bairro"`
	Localidade  string `json:"localidade"`
	Uf          string `json:"uf"`
	Ibge        string `json:"ibge"`
	Gia         string `json:"gia"`
	Ddd         string `json:"ddd"`
	Siafi       string `json:"siafi"`
}

// Response represents a generic API response with the API source
type Response struct {
	Data     interface{}
	APIName  string
	Error    error
	Duration time.Duration // Add duration field to track response time
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Por favor, forneça um CEP como argumento. Exemplo: go run main.go 01153000")
		return
	}

	cep := os.Args[1]
	fmt.Printf("Buscando informações para o CEP: %s\n", cep)

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	// Channel to receive responses
	resultChan := make(chan Response, 2)

	// Wait group to wait for both API calls to complete
	var wg sync.WaitGroup
	wg.Add(2)

	// Map to store timing results
	timingResults := make(map[string]time.Duration)
	var timingMutex sync.Mutex

	// Start goroutines to fetch data from both APIs
	go fetchBrasilAPI(ctx, cep, resultChan, &wg, &timingMutex, timingResults)
	go fetchViaCEP(ctx, cep, resultChan, &wg, &timingMutex, timingResults)

	// Wait for the first response or timeout
	select {
	case result := <-resultChan:
		if result.Error != nil {
			fmt.Printf("Erro na API %s: %v\n", result.APIName, result.Error)
			return
		}

		fmt.Printf("Resposta mais rápida da API: %s (%.3fs)\n\n", result.APIName, result.Duration.Seconds())

		switch data := result.Data.(type) {
		case BrasilAPICEP:
			fmt.Printf("CEP: %s\nEstado: %s\nCidade: %s\nBairro: %s\nRua: %s\n",
				data.Cep, data.State, data.City, data.Neighborhood, data.Street)
		case ViaCEP:
			fmt.Printf("CEP: %s\nEstado: %s\nCidade: %s\nBairro: %s\nRua: %s\n",
				data.Cep, data.Uf, data.Localidade, data.Bairro, data.Logradouro)
		}
	case <-ctx.Done():
		fmt.Println("Erro: Timeout após 1 segundo")
		return
	}

	// Start a goroutine to wait for all results and display comparative timing
	go func() {
		wg.Wait() // Wait for both API calls to complete or timeout

		// Print comparative timing results
		fmt.Println("\n=== Comparativo de Tempo de Resposta ===")
		timingMutex.Lock()
		defer timingMutex.Unlock()

		// Check if we have both results
		if len(timingResults) > 1 {
			// Find the fastest and slowest
			var fastest, slowest string
			var fastestTime, slowestTime time.Duration

			for api, duration := range timingResults {
				if fastest == "" || duration < fastestTime {
					fastest = api
					fastestTime = duration
				}
				if slowest == "" || duration > slowestTime {
					slowest = api
					slowestTime = duration
				}
			}

			// Print results
			fmt.Printf("API mais rápida: %s (%.3fs)\n", fastest, fastestTime.Seconds())
			fmt.Printf("API mais lenta: %s (%.3fs)\n", slowest, slowestTime.Seconds())
			fmt.Printf("Diferença: %.3fs\n", slowestTime.Seconds()-fastestTime.Seconds())

			for api, duration := range timingResults {
				fmt.Printf("%s: %.3fs\n", api, duration.Seconds())
			}
		} else {
			fmt.Println("Não foi possível obter resposta de ambas as APIs para comparação.")
		}
	}()

	// Read the second response to clear the channel
	select {
	case <-resultChan: // Discard the second response
	case <-time.After(100 * time.Millisecond): // Small timeout in case second response never arrives
	}

	// Give time for the timing comparison to be displayed
	time.Sleep(200 * time.Millisecond)
}

func fetchBrasilAPI(ctx context.Context, cep string, resultChan chan<- Response, wg *sync.WaitGroup, mu *sync.Mutex, results map[string]time.Duration) {
	defer wg.Done()
	startTime := time.Now()
	apiName := "BrasilAPI"

	url := fmt.Sprintf("https://brasilapi.com.br/api/cep/v1/%s", cep)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		resultChan <- Response{APIName: apiName, Error: err, Duration: time.Since(startTime)}
		return
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		resultChan <- Response{APIName: apiName, Error: err, Duration: time.Since(startTime)}
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		resultChan <- Response{APIName: apiName, Error: fmt.Errorf("status code: %d", resp.StatusCode), Duration: time.Since(startTime)}
		return
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		resultChan <- Response{APIName: apiName, Error: err, Duration: time.Since(startTime)}
		return
	}

	var data BrasilAPICEP
	if err := json.Unmarshal(body, &data); err != nil {
		resultChan <- Response{APIName: apiName, Error: err, Duration: time.Since(startTime)}
		return
	}

	duration := time.Since(startTime)

	// Store timing result
	mu.Lock()
	results[apiName] = duration
	mu.Unlock()

	resultChan <- Response{APIName: apiName, Data: data, Duration: duration}
}

func fetchViaCEP(ctx context.Context, cep string, resultChan chan<- Response, wg *sync.WaitGroup, mu *sync.Mutex, results map[string]time.Duration) {
	defer wg.Done()
	startTime := time.Now()
	apiName := "ViaCEP"

	url := fmt.Sprintf("http://viacep.com.br/ws/%s/json/", cep)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		resultChan <- Response{APIName: apiName, Error: err, Duration: time.Since(startTime)}
		return
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		resultChan <- Response{APIName: apiName, Error: err, Duration: time.Since(startTime)}
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		resultChan <- Response{APIName: apiName, Error: fmt.Errorf("status code: %d", resp.StatusCode), Duration: time.Since(startTime)}
		return
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		resultChan <- Response{APIName: apiName, Error: err, Duration: time.Since(startTime)}
		return
	}

	var data ViaCEP
	if err := json.Unmarshal(body, &data); err != nil {
		resultChan <- Response{APIName: apiName, Error: err, Duration: time.Since(startTime)}
		return
	}

	duration := time.Since(startTime)

	// Store timing result
	mu.Lock()
	results[apiName] = duration
	mu.Unlock()

	resultChan <- Response{APIName: apiName, Data: data, Duration: duration}
}
