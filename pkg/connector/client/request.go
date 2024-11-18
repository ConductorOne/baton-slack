package enterprise

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
	payload []byte,
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
		return nil, err
	}
	var ratelimitData v2.RateLimitDescription
	response, err := c.wrapper.Do(
		request,
		uhttp.WithRatelimitData(&ratelimitData),
	)
	if err != nil {
		return &ratelimitData, err
	}
	defer response.Body.Close()

	bodyBytes, err := io.ReadAll(response.Body)
	if err != nil {
		return &ratelimitData, err
	}

	if err := json.Unmarshal(bodyBytes, &target); err != nil {
		return nil, err
	}

	return &ratelimitData, nil
}
