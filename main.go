package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
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
	Data    interface{}
	APIName string
	Error   error
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

	// Start goroutines to fetch data from both APIs
	go fetchBrasilAPI(ctx, cep, resultChan)
	go fetchViaCEP(ctx, cep, resultChan)

	// Wait for the first response or timeout
	select {
	case result := <-resultChan:
		if result.Error != nil {
			fmt.Printf("Erro na API %s: %v\n", result.APIName, result.Error)
			return
		}

		fmt.Printf("Resposta mais rápida da API: %s\n\n", result.APIName)

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
	}
}

func fetchBrasilAPI(ctx context.Context, cep string, resultChan chan<- Response) {
	url := fmt.Sprintf("https://brasilapi.com.br/api/cep/v1/%s", cep)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		resultChan <- Response{APIName: "BrasilAPI", Error: err}
		return
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		resultChan <- Response{APIName: "BrasilAPI", Error: err}
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		resultChan <- Response{APIName: "BrasilAPI", Error: fmt.Errorf("status code: %d", resp.StatusCode)}
		return
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		resultChan <- Response{APIName: "BrasilAPI", Error: err}
		return
	}

	var data BrasilAPICEP
	if err := json.Unmarshal(body, &data); err != nil {
		resultChan <- Response{APIName: "BrasilAPI", Error: err}
		return
	}

	resultChan <- Response{APIName: "BrasilAPI", Data: data}
}

func fetchViaCEP(ctx context.Context, cep string, resultChan chan<- Response) {
	url := fmt.Sprintf("http://viacep.com.br/ws/%s/json/", cep)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		resultChan <- Response{APIName: "ViaCEP", Error: err}
		return
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		resultChan <- Response{APIName: "ViaCEP", Error: err}
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		resultChan <- Response{APIName: "ViaCEP", Error: fmt.Errorf("status code: %d", resp.StatusCode)}
		return
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		resultChan <- Response{APIName: "ViaCEP", Error: err}
		return
	}

	var data ViaCEP
	if err := json.Unmarshal(body, &data); err != nil {
		resultChan <- Response{APIName: "ViaCEP", Error: err}
		return
	}

	resultChan <- Response{APIName: "ViaCEP", Data: data}
}
