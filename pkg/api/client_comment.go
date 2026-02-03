package api

import "fmt"

func (c *Client) CreateComment(postID, content string) error {
	_, err := c.request("POST", fmt.Sprintf("/posts/%s/comments", postID), map[string]string{
		"content": content,
	}, nil)
	return err
}
