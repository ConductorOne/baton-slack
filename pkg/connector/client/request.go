package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"

	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/uhttp"
	"github.com/grpc-ecosystem/go-grpc-middleware/logging/zap/ctxzap"
	"go.uber.org/zap"
)

func toValues(queryParameters map[string]interface{}) string {
	params := url.Values{}
	for key, valueAny := range queryParameters {
		switch value := valueAny.(type) {
		case string:
			params.Add(key, value)
		case int:
			params.Add(key, strconv.Itoa(value))
		case bool:
			params.Add(key, strconv.FormatBool(value))
		case []string:
			// Handle string arrays for Slack API
			for _, str := range value {
				params.Add(key, str)
			}
		default:
			continue
		}
	}
	return params.Encode()
}

func (c *Client) getUrl(
	path string,
	queryParameters map[string]interface{},
	useScim bool,
) *url.URL {
	var output *url.URL
	if useScim {
		output = c.baseScimUrl.JoinPath(path)
	} else {
		output = c.baseUrl.JoinPath(path)
	}

	output.RawQuery = toValues(queryParameters)
	return output
}

// WithBearerToken - TODO(marcos): move this function to `baton-sdk`.
func WithBearerToken(token string) uhttp.RequestOption {
	return uhttp.WithHeader("Authorization", fmt.Sprintf("Bearer %s", token))
}

func (c *Client) post(
	ctx context.Context,
	path string,
	target interface{},
	payload map[string]interface{},
	useBotToken bool,
) (
	*v2.RateLimitDescription,
	error,
) {
	token := c.token
	if useBotToken {
		token = c.botToken
	}

	return c.doRequest(
		ctx,
		http.MethodPost,
		c.getUrl(path, nil, false),
		target,
		WithBearerToken(token),
		uhttp.WithFormBody(toValues(payload)),
	)
}

func (c *Client) getScim(
	ctx context.Context,
	path string,
	target interface{},
	queryParameters map[string]interface{},
) (
	*v2.RateLimitDescription,
	error,
) {
	return c.doRequest(
		ctx,
		http.MethodGet,
		c.getUrl(path, queryParameters, true),
		&target,
		WithBearerToken(c.token),
	)
}

func (c *Client) patchScim(
	ctx context.Context,
	path string,
	target interface{},
	payload map[string]any,
) (
	*v2.RateLimitDescription,
	error,
) {
	return c.doRequest(
		ctx,
		http.MethodPatch,
		c.getUrl(path, nil, true),
		&target,
		WithBearerToken(c.token),
		uhttp.WithJSONBody(payload),
	)
}

func (c *Client) doRequest(
	ctx context.Context,
	method string,
	url *url.URL,
	target interface{},
	options ...uhttp.RequestOption,
) (
	*v2.RateLimitDescription,
	error,
) {
	logger := ctxzap.Extract(ctx)
	logger.Debug(
		"making request",
		zap.String("method", method),
		zap.String("url", url.String()),
	)

	options = append(
		options,
		uhttp.WithAcceptJSONHeader(),
	)

	request, err := c.wrapper.NewRequest(
		ctx,
		method,
		url,
		options...,
	)
	if err != nil {
		return nil, fmt.Errorf("creating HTTP request: %w", err)
	}

	var ratelimitData v2.RateLimitDescription
	response, err := c.wrapper.Do(
		request,
		uhttp.WithRatelimitData(&ratelimitData),
	)
	if err != nil {
		logBody(ctx, response)
		return &ratelimitData, err
	}
	defer response.Body.Close()

	bodyBytes, err := io.ReadAll(response.Body)
	if err != nil {
		return &ratelimitData, fmt.Errorf("reading response body: %w", err)
	}

	if response.StatusCode != http.StatusNoContent && len(bodyBytes) > 0 {
		if err := json.Unmarshal(bodyBytes, &target); err != nil {
			logBody(ctx, response)
			return nil, fmt.Errorf("unmarshaling response: %w", err)
		}
	}

	return &ratelimitData, nil
}

func (c *Client) doScimRequest(
	ctx context.Context,
	method string,
	path string,
	target interface{},
	payload interface{},
) (
	*v2.RateLimitDescription,
	error,
) {
	options := []uhttp.RequestOption{
		WithBearerToken(c.token),
	}

	if payload != nil {
		options = append(options, uhttp.WithJSONBody(payload))
	}

	return c.doRequest(
		ctx,
		method,
		c.getUrl(path, nil, true),
		target,
		options...,
	)
}

func (c *Client) deleteScim(
	ctx context.Context,
	path string,
) (
	*v2.RateLimitDescription,
	error,
) {
	logger := ctxzap.Extract(ctx)
	logger.Debug(
		"making request",
		zap.String("method", http.MethodDelete),
		zap.String("url", c.getUrl(path, nil, true).String()),
	)

	request, err := c.wrapper.NewRequest(
		ctx,
		http.MethodDelete,
		c.getUrl(path, nil, true),
		WithBearerToken(c.token),
		uhttp.WithAcceptJSONHeader(),
	)
	if err != nil {
		return nil, fmt.Errorf("creating SCIM delete request: %w", err)
	}

	var ratelimitData v2.RateLimitDescription
	response, err := c.wrapper.Do(
		request,
		uhttp.WithRatelimitData(&ratelimitData),
	)
	if err != nil {
		logBody(ctx, response)
		return &ratelimitData, err
	}
	defer response.Body.Close()

	if response.StatusCode == http.StatusNoContent {
		return &ratelimitData, nil
	}

	bodyBytes, err := io.ReadAll(response.Body)
	if err != nil {
		logBody(ctx, response)
		return &ratelimitData, fmt.Errorf("reading SCIM error response body: %w", err)
	}

	if len(bodyBytes) > 0 {
		var errorResponse map[string]interface{}
		if err := json.Unmarshal(bodyBytes, &errorResponse); err != nil {
			return &ratelimitData, fmt.Errorf("parsing SCIM error response: %w", err)
		}
		if detail, ok := errorResponse["detail"].(string); ok {
			return &ratelimitData, fmt.Errorf("SCIM API error: %s", detail)
		}
	}

	return &ratelimitData, nil
}
