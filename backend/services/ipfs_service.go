package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"time"
)

type IPFSClient struct {
	endpoint string
	timeout  time.Duration
	client   *http.Client
}

type IPFSResult struct {
	CID  string `json:"cid"`
	Hash string `json:"hash"`
	Size int    `json:"size"`
	URL  string `json:"url"`
}

func NewIPFSClient() *IPFSClient {
	endpoint := os.Getenv("IPFS_API_URL")
	if endpoint == "" {
		endpoint = "https://ipfs.infura.io:5001"
	}
	return &IPFSClient{
		endpoint: endpoint,
		timeout:  60 * time.Second,
		client: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

func (c *IPFSClient) UploadFile(fileContent []byte, filename string) (*IPFSResult, error) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return nil, fmt.Errorf("ipfs: failed to create form file: %w", err)
	}
	if _, err := part.Write(fileContent); err != nil {
		return nil, fmt.Errorf("ipfs: failed to write file content: %w", err)
	}
	writer.Close()

	req, err := http.NewRequest("POST", c.endpoint+"/api/v0/add", body)
	if err != nil {
		return nil, fmt.Errorf("ipfs: failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ipfs: upload request failed: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		Hash string `json:"Hash"`
		Size int    `json:"Size"`
		Name string `json:"Name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("ipfs: failed to decode response: %w", err)
	}

	gatewayURL := os.Getenv("IPFS_GATEWAY_URL")
	if gatewayURL == "" {
		gatewayURL = "https://ipfs.io/ipfs"
	}

	return &IPFSResult{
		CID:  result.Hash,
		Hash: result.Hash,
		Size: result.Size,
		URL:  fmt.Sprintf("%s/%s", gatewayURL, result.Hash),
	}, nil
}

func (c *IPFSClient) UploadJSON(data interface{}) (*IPFSResult, error) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("ipfs: failed to marshal JSON: %w", err)
	}

	return c.UploadFile(jsonData, "metadata.json")
}

func (c *IPFSClient) Cat(cid string) ([]byte, error) {
	req, err := http.NewRequest("POST", c.endpoint+"/api/v0/cat?arg="+cid, nil)
	if err != nil {
		return nil, fmt.Errorf("ipfs: failed to create cat request: %w", err)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ipfs: cat request failed: %w", err)
	}
	defer resp.Body.Close()

	return io.ReadAll(resp.Body)
}

func (c *IPFSClient) Pin(cid string) error {
	req, err := http.NewRequest("POST", c.endpoint+"/api/v0/pin/add?arg="+cid, nil)
	if err != nil {
		return fmt.Errorf("ipfs: failed to create pin request: %w", err)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("ipfs: pin request failed: %w", err)
	}
	defer resp.Body.Close()

	return nil
}
