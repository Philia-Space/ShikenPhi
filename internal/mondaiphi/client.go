package mondaiphi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	examd "github.com/philiaspace/phi-exam-domain/domain"
)

// Client is an HTTP client for calling MondaiPhi APIs.
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// NewClient creates a new MondaiPhi API client.
func NewClient(baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Question represents a sanitized question from MondaiPhi.
type Question struct {
	ID             string   `json:"id"`
	Level          string   `json:"level"`
	Section        string   `json:"section"`
	Prompt         string   `json:"prompt"`
	Context        string   `json:"context,omitempty"`
	PassageID      string   `json:"passage_id,omitempty"`
	SourceGroupKey string   `json:"source_group_key,omitempty"`
	Options        []Option `json:"options,omitempty"`
}

// Option represents an answer choice.
type Option struct {
	ID        string `json:"id"`
	Value     string `json:"value"`
	Label     string `json:"label"`
	SortOrder int    `json:"sort_order"`
}

// Passage represents a reading/listening passage.
type Passage struct {
	ID            string `json:"id"`
	PassageNumber int    `json:"passage_number"`
	Title         string `json:"title"`
	Content       string `json:"content"`
	Level         string `json:"level"`
	Section       string `json:"section"`
}

// PackageTemplate represents an exam blueprint.
type PackageTemplate struct {
	ID             string         `json:"id"`
	Name           string         `json:"name"`
	Level          string         `json:"level"`
	SectionCounts  map[string]int `json:"section_counts"`
	TotalQuestions int            `json:"total_questions"`
	IsDefault      bool           `json:"is_default"`
}

// transportResponse is the envelope used by phi-core/transport.
type transportResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data"`
	Error   *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// ListQuestions fetches questions from MondaiPhi.
func (c *Client) ListQuestions(ctx context.Context, level examd.JLPTLevel, section examd.Section, limit int) ([]Question, error) {
	u, err := url.Parse(c.baseURL + "/questions")
	if err != nil {
		return nil, err
	}
	q := u.Query()
	q.Set("level", string(level))
	q.Set("section", string(section))
	q.Set("limit", fmt.Sprintf("%d", limit))
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("mondaiphi returned %d", resp.StatusCode)
	}

	var envelope transportResponse
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return nil, err
	}

	if !envelope.Success {
		return nil, fmt.Errorf("mondaiphi error: %v", envelope.Error)
	}

	dataMap, ok := envelope.Data.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected data format")
	}

	questionsRaw, ok := dataMap["questions"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("questions not found in response")
	}

	var questions []Question
	for _, q := range questionsRaw {
		qBytes, _ := json.Marshal(q)
		var question Question
		if err := json.Unmarshal(qBytes, &question); err != nil {
			continue
		}
		questions = append(questions, question)
	}

	return questions, nil
}

// GetQuestion fetches a single question with options.
func (c *Client) GetQuestion(ctx context.Context, id string) (*Question, []Option, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/questions/"+id, nil)
	if err != nil {
		return nil, nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, nil, fmt.Errorf("mondaiphi returned %d", resp.StatusCode)
	}

	var envelope transportResponse
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return nil, nil, err
	}

	if !envelope.Success {
		return nil, nil, fmt.Errorf("mondaiphi error: %v", envelope.Error)
	}

	dataMap, ok := envelope.Data.(map[string]interface{})
	if !ok {
		return nil, nil, fmt.Errorf("unexpected data format")
	}

	qBytes, _ := json.Marshal(dataMap["question"])
	var question Question
	if err := json.Unmarshal(qBytes, &question); err != nil {
		return nil, nil, err
	}

	var options []Option
	if optsRaw, ok := dataMap["options"].([]interface{}); ok {
		for _, o := range optsRaw {
			oBytes, _ := json.Marshal(o)
			var option Option
			if err := json.Unmarshal(oBytes, &option); err == nil {
				options = append(options, option)
			}
		}
	}

	return &question, options, nil
}

// ListTemplates fetches package templates.
func (c *Client) ListTemplates(ctx context.Context, level examd.JLPTLevel) ([]PackageTemplate, error) {
	u, err := url.Parse(c.baseURL + "/templates")
	if err != nil {
		return nil, err
	}
	if level != "" {
		q := u.Query()
		q.Set("level", string(level))
		u.RawQuery = q.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("mondaiphi returned %d", resp.StatusCode)
	}

	var envelope transportResponse
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return nil, err
	}

	if !envelope.Success {
		return nil, fmt.Errorf("mondaiphi error: %v", envelope.Error)
	}

	dataMap, ok := envelope.Data.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected data format")
	}

	templatesRaw, ok := dataMap["templates"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("templates not found in response")
	}

	var templates []PackageTemplate
	for _, t := range templatesRaw {
		tBytes, _ := json.Marshal(t)
		var template PackageTemplate
		if err := json.Unmarshal(tBytes, &template); err != nil {
			continue
		}
		templates = append(templates, template)
	}

	return templates, nil
}

// GetPassage fetches a passage with its questions.
func (c *Client) GetPassage(ctx context.Context, id string) (*Passage, []Question, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/passages/"+id, nil)
	if err != nil {
		return nil, nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, nil, fmt.Errorf("mondaiphi returned %d", resp.StatusCode)
	}

	var envelope transportResponse
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return nil, nil, err
	}

	if !envelope.Success {
		return nil, nil, fmt.Errorf("mondaiphi error: %v", envelope.Error)
	}

	dataMap, ok := envelope.Data.(map[string]interface{})
	if !ok {
		return nil, nil, fmt.Errorf("unexpected data format")
	}

	pBytes, _ := json.Marshal(dataMap["passage"])
	var passage Passage
	if err := json.Unmarshal(pBytes, &passage); err != nil {
		return nil, nil, err
	}

	var questions []Question
	if qsRaw, ok := dataMap["questions"].([]interface{}); ok {
		for _, q := range qsRaw {
			qBytes, _ := json.Marshal(q)
			var question Question
			if err := json.Unmarshal(qBytes, &question); err == nil {
				questions = append(questions, question)
			}
		}
	}

	return &passage, questions, nil
}
