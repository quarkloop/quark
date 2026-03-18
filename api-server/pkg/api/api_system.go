package api

import "context"

func (c *ClientApi) SystemInfo(ctx context.Context) (map[string]interface{}, error) {
	var info map[string]interface{}
	// Calling the updated Get method with context
	err := c.client.Get(ctx, "/api/v1/system/info", &info)
	return info, err
}
