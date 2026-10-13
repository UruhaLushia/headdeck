package headscale

func (c *Client) APIKey() (string, error) {
	if c.key == "" {
		return "", ErrNoAPIKey
	}
	return c.key, nil
}
