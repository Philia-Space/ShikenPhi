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
	baseURL       string
	serviceSecret string
	httpClient    *http.Client
}

// NewClient creates a new MondaiPhi API client.
func NewClient(baseURL string, serviceSecret ...string) *Client {
	secret := ""
	if len(serviceSecret) > 0 {
		secret = serviceSecret[0]
	}
	return &Client{
		baseURL:       baseURL,
		serviceSecret: secret,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (c *Client) GetQuestionForScoring(ctx context.Context, id string) (*Question, []Option, []Asset, error) {
	return c.getQuestion(ctx, id, true)
}

type Question struct {
	ID             string   `json:"id"`
	Level          string   `json:"level"`
	Section        string   `json:"section"`
	Prompt         string   `json:"prompt"`
	Context        string   `json:"context,omitempty"`
	AnswerValue    string   `json:"answer_value,omitempty"`
	PassageID      string   `json:"passage_id,omitempty"`
	SourceGroupKey string   `json:"source_group_key,omitempty"`
	Options        []Option `json:"options,omitempty"`
}

type Option struct {
	ID        string `json:"id"`
	Value     string `json:"value"`
	Label     string `json:"label"`
	SortOrder int    `json:"sort_order"`
}

type Asset struct {
	ID   string `json:"id"`
	Type string `json:"type"`
}

type Passage struct {
	ID            string `json:"id"`
	PassageNumber int    `json:"passage_number"`
	Title         string `json:"title"`
	Content       string `json:"content"`
	Level         string `json:"level"`
	Section       string `json:"section"`
}

type PackageTemplate struct {
	ID             string         `json:"id"`
	Name           string         `json:"name"`
	Level          string         `json:"level"`
	SectionCounts  map[string]int `json:"section_counts"`
	TotalQuestions int            `json:"total_questions"`
	IsDefault      bool           `json:"is_default"`
}

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

// GetQuestion fetches a single question with options and assets (sanitized).
func (c *Client) GetQuestion(ctx context.Context, id string) (*Question, []Option, []Asset, error) {
	return c.getQuestion(ctx, id, false)
}

func (c *Client) getQuestion(ctx context.Context, id string, includeAnswer bool) (*Question, []Option, []Asset, error) {
	path := "/questions/" + id
	if includeAnswer {
		path = "/internal/questions/" + id
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return nil, nil, nil, err
	}

	// Add service secret for internal endpoints
	if includeAnswer && c.serviceSecret != "" {
		req.Header.Set("X-Service-Secret", c.serviceSecret)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, nil, nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, nil, nil, fmt.Errorf("mondaiphi returned %d", resp.StatusCode)
	}

	var envelope transportResponse
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return nil, nil, nil, err
	}

	if !envelope.Success {
		return nil, nil, nil, fmt.Errorf("mondaiphi error: %v", envelope.Error)
	}

	dataMap, ok := envelope.Data.(map[string]interface{})
	if !ok {
		return nil, nil, nil, fmt.Errorf("unexpected data format")
	}

	qBytes, _ := json.Marshal(dataMap["question"])
	var question Question
	if err := json.Unmarshal(qBytes, &question); err != nil {
		return nil, nil, nil, err
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

	var assets []Asset
	if assetsRaw, ok := dataMap["assets"].([]interface{}); ok {
		for _, a := range assetsRaw {
			aBytes, _ := json.Marshal(a)
			var asset Asset
			if err := json.Unmarshal(aBytes, &asset); err == nil {
				assets = append(assets, asset)
			}
		}
	}

	return &question, options, assets, nil
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

// GetAssetURL follows the MondaiPhi asset redirect to get the final S3 presigned URL.
func (c *Client) GetAssetURL(ctx context.Context, assetID string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/assets/"+assetID, nil)
	if err != nil {
		return "", err
	}

	transportClient := &http.Client{
		Timeout: 10 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	resp, err := transportClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusTemporaryRedirect || resp.StatusCode == http.StatusFound {
		return resp.Header.Get("Location"), nil
	}

	return "", fmt.Errorf("asset %s returned status %d", assetID, resp.StatusCode)
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
