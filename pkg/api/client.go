package api

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-resty/resty/v2"
)

const BaseURL = "https://www.moltbook.com/api/v1"

type Client struct {
	restClient *resty.Client
	APIKey     string
}

func NewClient(apiKey string) *Client {
	c := resty.New()
	c.SetBaseURL(BaseURL)
	c.SetTimeout(60 * time.Second)
	// Add retry logic for stability
	c.SetRetryCount(3)
	c.SetRetryWaitTime(1 * time.Second)
	c.SetRetryMaxWaitTime(5 * time.Second)
	
	c.SetHeader("User-Agent", "moltbook-go-client/1.0")
	
	if apiKey != "" {
		c.SetAuthToken(apiKey)
		// Fallback for some older platform versions
		c.SetHeader("X-API-Key", apiKey)
	}
	
	return &Client{
		restClient: c,
		APIKey:     apiKey,
	}
}

// FlexibleResponse handles both wrapped and unwrapped JSON structures
type FlexibleResponse struct {
	Success bool            `json:"success"`
	Data    json.RawMessage `json:"data"`
	Error   string          `json:"error"`
	Hint    string          `json:"hint"`
	// Additional root-level fields some endpoints use
	Agent       *Agent  `json:"agent"`
	Posts       []Post  `json:"posts"`
	Results     []Post  `json:"results"`
	Comments    []Comment `json:"comments"`
	Status      string    `json:"status"`
	RecentPosts []Post    `json:"recentPosts"`
}

type Agent struct {
	Name             string `json:"name"`
	Description      string `json:"description"`
	Karma            int    `json:"karma"`
	FollowerCount    int    `json:"follower_count"`
	FollowingCount   int    `json:"following_count"`
	IsClaimed        bool   `json:"is_claimed"`
	APIKey           string `json:"api_key,omitempty"`
	ClaimURL         string `json:"claim_url,omitempty"`
	VerificationCode string `json:"verification_code,omitempty"`
}

type Post struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"`
	Title     string    `json:"title"`
	Content   string    `json:"content"`
	URL       string    `json:"url,omitempty"`
	Upvotes   int       `json:"upvotes"`
	Downvotes int       `json:"downvotes"`
	CreatedAt time.Time `json:"created_at"`
	Author    struct {
		Name string `json:"name"`
	} `json:"author"`
	Submolt struct {
		Name        string `json:"name"`
		DisplayName string `json:"display_name"`
	} `json:"submolt"`
	Similarity float64 `json:"similarity,omitempty"`
}

type Comment struct {
	ID        string    `json:"id"`
	Content   string    `json:"content"`
	Author    struct {
		Name string `json:"name"`
	} `json:"author"`
	Upvotes   int       `json:"upvotes"`
	Downvotes int       `json:"downvotes"`
	CreatedAt time.Time `json:"created_at"`
}

func (c *Client) request(method, path string, body interface{}, params map[string]string) (*FlexibleResponse, error) {
	req := c.restClient.R()
	if body != nil {
		req.SetBody(body)
	}
	if params != nil {
		req.SetQueryParams(params)
	}
	if c.APIKey != "" {
		req.SetAuthToken(c.APIKey)
		req.SetHeader("X-API-Key", c.APIKey)
	}

	var res FlexibleResponse
	resp, err := req.Execute(method, path)
	if err != nil {
		return nil, fmt.Errorf("network error: %w", err)
	}

	// Handle Rate Limiting explicitly
	if resp.StatusCode() == 429 {
		var rateRes struct {
			Error string `json:"error"`
			Hint  string `json:"hint"`
			RetryAfterSeconds int `json:"retry_after_seconds"`
			RetryAfterMinutes int `json:"retry_after_minutes"`
		}
		json.Unmarshal(resp.Body(), &rateRes)
		msg := "Rate limit exceeded"
		if rateRes.Hint != "" {
			msg += ": " + rateRes.Hint
		} else if rateRes.Error != "" {
			msg += ": " + rateRes.Error
		}
		return nil, fmt.Errorf("%s (Retry after %d seconds)", msg, rateRes.RetryAfterSeconds)
	}

	// Try to parse the response as a FlexibleResponse
	if err := json.Unmarshal(resp.Body(), &res); err != nil {
		if !resp.IsSuccess() {
			return nil, fmt.Errorf("API error (%d): %s", resp.StatusCode(), resp.String())
		}
		return nil, fmt.Errorf("failed to parse JSON (%d): %v", resp.StatusCode(), err)
	}

	if !res.Success {
		if res.Error != "" {
			msg := res.Error
			if res.Hint != "" {
				msg += fmt.Sprintf(" (Hint: %s)", res.Hint)
			}
			return &res, fmt.Errorf("%s", msg)
		}
		if !resp.IsSuccess() {
			return &res, fmt.Errorf("request failed: %d", resp.StatusCode())
		}
	}

	return &res, nil
}

func (c *Client) Register(name, description string) (*Agent, error) {
	res, err := c.request("POST", "/agents/register", map[string]string{
		"name":        name,
		"description": description,
	}, nil)
	if err != nil {
		return nil, err
	}

	if res.Agent != nil {
		return res.Agent, nil
	}

	var data struct {
		Agent Agent `json:"agent"`
	}
	if err := json.Unmarshal(res.Data, &data); err == nil && data.Agent.Name != "" {
		return &data.Agent, nil
	}

	return nil, fmt.Errorf("failed to find agent in response")
}

func (c *Client) GetFeed(sort string, limit, offset int) ([]Post, error) {
	params := map[string]string{
		"sort":   sort,
		"limit":  fmt.Sprintf("%d", limit),
		"offset": fmt.Sprintf("%d", offset),
	}
	
	res, err := c.request("GET", "/posts", nil, params)
	if err != nil {
		return nil, err 
	}

	if len(res.Posts) > 0 {
		return res.Posts, nil
	}
	
	var data struct {
		Posts []Post `json:"posts"`
	}
	if err := json.Unmarshal(res.Data, &data); err == nil && len(data.Posts) > 0 {
		return data.Posts, nil
	}

	return res.Posts, nil
}

// Added this new method support
func (c *Client) GetSubmoltFeed(submolt, sort string, limit int) ([]Post, error) {
	if limit == 0 { limit = 20 }
	path := fmt.Sprintf("/submolts/%s/feed", submolt)
	res, err := c.request("GET", path, nil, map[string]string{
		"sort":  sort,
		"limit": fmt.Sprintf("%d", limit),
	})
	if err != nil {
		// Fallback to query param if convenience endpoint fails
		res, err = c.request("GET", "/posts", nil, map[string]string{
			"submolt": submolt,
			"sort":    sort,
			"limit":   fmt.Sprintf("%d", limit),
		})
		if err != nil {
			return nil, err
		}
	}

	if len(res.Posts) > 0 {
		return res.Posts, nil
	}
	
	var data struct {
		Posts []Post `json:"posts"`
	}
	if err := json.Unmarshal(res.Data, &data); err == nil {
		return data.Posts, nil
	}

	return []Post{}, nil
}

func (c *Client) GetPersonalizedFeed(sort string, limit, offset int) ([]Post, error) {
	if limit == 0 { limit = 20 }
	res, err := c.request("GET", "/feed", nil, map[string]string{
		"sort":   sort,
		"limit":  fmt.Sprintf("%d", limit),
		"offset": fmt.Sprintf("%d", offset),
	})
	if err != nil {
		return nil, err
	}

	if len(res.Posts) > 0 {
		return res.Posts, nil
	}

	var data struct {
		Posts []Post `json:"posts"`
	}
	if err := json.Unmarshal(res.Data, &data); err == nil && len(data.Posts) > 0 {
		return data.Posts, nil
	}

	return res.Posts, nil
}

func (c *Client) Search(query string, searchType string) ([]Post, error) {
	res, err := c.request("GET", "/search", nil, map[string]string{
		"q":    query,
		"type": searchType,
	})
	if err != nil {
		return nil, err
	}

	if len(res.Results) > 0 {
		return res.Results, nil
	}

	var data struct {
		Results []Post `json:"results"`
	}
	if err := json.Unmarshal(res.Data, &data); err == nil && len(data.Results) > 0 {
		return data.Results, nil
	}

	return res.Results, nil
}

func (c *Client) CreatePost(submolt, title, content string) error {
	_, err := c.request("POST", "/posts", map[string]string{
		"submolt": submolt,
		"title":   title,
		"content": content,
	}, nil)
	return err
}

func (c *Client) DeletePost(postID string) error {
	_, err := c.request("DELETE", fmt.Sprintf("/posts/%s", postID), nil, nil)
	return err
}

func (c *Client) UpvotePost(postID string) error {
	_, err := c.request("POST", fmt.Sprintf("/posts/%s/upvote", postID), map[string]string{}, nil)
	return err
}

func (c *Client) GetComments(postID string) ([]Comment, error) {
	res, err := c.request("GET", fmt.Sprintf("/posts/%s/comments", postID), nil, nil)
	if err != nil {
		return nil, err
	}

	if len(res.Comments) > 0 {
		return res.Comments, nil
	}

	var data struct {
		Comments []Comment `json:"comments"`
	}
	if err := json.Unmarshal(res.Data, &data); err == nil {
		return data.Comments, nil
	}

	return nil, fmt.Errorf("could not find comments in response")
}

func (c *Client) GetMe() (*Agent, error) {
	res, err := c.request("GET", "/agents/me", nil, nil)
	if err != nil {
		return nil, err
	}

	if res.Agent != nil {
		return res.Agent, nil
	}

	var data struct {
		Agent Agent `json:"agent"`
	}
	if err := json.Unmarshal(res.Data, &data); err == nil && data.Agent.Name != "" {
		return &data.Agent, nil
	}

	return nil, fmt.Errorf("could not find agent in response")
}

func (c *Client) GetProfile(name string) (*Agent, []Post, error) {
	res, err := c.request("GET", "/agents/profile", nil, map[string]string{
		"name": name,
	})
	if err != nil {
		return nil, nil, err
	}

	agent := res.Agent
	posts := res.RecentPosts

	if agent == nil {
		var data struct {
			Agent Agent `json:"agent"`
		}
		if err := json.Unmarshal(res.Data, &data); err == nil {
			agent = &data.Agent
		}
	}

	if len(posts) == 0 {
		var data struct {
			RecentPosts []Post `json:"recentPosts"`
		}
		if err := json.Unmarshal(res.Data, &data); err == nil {
			posts = data.RecentPosts
		}
	}

	if agent == nil {
		return nil, nil, fmt.Errorf("could not find profile data")
	}

	return agent, posts, nil
}

func (c *Client) GetStatus() (string, error) {
	res, err := c.request("GET", "/agents/status", nil, nil)
	if err != nil {
		return "", err
	}

	if res.Status != "" {
		return res.Status, nil
	}

	var data struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(res.Data, &data); err == nil {
		return data.Status, nil
	}

	return "", fmt.Errorf("could not find status in response")
}

func (c *Client) Follow(name string) error {
	_, err := c.request("POST", fmt.Sprintf("/agents/%s/follow", name), nil, nil)
	return err
}

func (c *Client) Unfollow(name string) error {
	_, err := c.request("DELETE", fmt.Sprintf("/agents/%s/follow", name), nil, nil)
	return err
}

func (c *Client) Subscribe(submolt string) error {
	_, err := c.request("POST", fmt.Sprintf("/submolts/%s/subscribe", submolt), nil, nil)
	return err
}

func (c *Client) Unsubscribe(submolt string) error {
	_, err := c.request("DELETE", fmt.Sprintf("/submolts/%s/subscribe", submolt), nil, nil)
	return err
}

func (c *Client) UpdateProfile(description string) error {
	_, err := c.request("PATCH", "/agents/me", map[string]string{
		"description": description,
	}, nil)
	return err
}
