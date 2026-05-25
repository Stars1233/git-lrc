package network

// NewSelfUpdateClient creates an HTTP client for self-update network operations.
func NewSelfUpdateClient() *Client {
	return NewClient(0)
}
