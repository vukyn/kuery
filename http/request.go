package http

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

// MakeRequest make an outbound request and handle retry
//
//	Params:
//		ctx: context
//		httpClient: http client
//		req: http request
//		maxRetry: max retry times
//		waitRetry: wait time between retry (in second)
//		condRetry: retry condition
//	Return:
//		[]byte: response body
//		error: error
func MakeRequest(ctx context.Context, httpClient *http.Client, req *http.Request,
	maxRetry int, waitRetry int, condRetry func(*http.Response) bool) ([]byte, error) {
	return makeRequestRecursive(ctx, httpClient, req, 1, maxRetry, waitRetry, condRetry)
}

func makeRequestRecursive(
	ctx context.Context,
	httpClient *http.Client,
	req *http.Request,
	retryCount int,
	maxRetry int,
	waitRetry int,
	condRetry func(*http.Response) bool) ([]byte, error) {

	var reqBody io.ReadCloser
	if req.Body != nil {
		reqBody, _ = req.GetBody() // keep this body for future retry
	}

	// set body to req
	if reqBody != nil {
		req.Body = reqBody
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return []byte{}, err
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return []byte{}, err
	}
	defer resp.Body.Close()

	retry := func() ([]byte, error) {
		if retryCount >= maxRetry {
			return body, errors.New(fmt.Sprintf("response status code is %d", resp.StatusCode))
		}
		time.Sleep(time.Duration(waitRetry) * time.Second)
		if reqBody != nil {
			// reset body, as the old one has been read
			req.Body = io.NopCloser(reqBody)
		}
		return makeRequestRecursive(ctx, httpClient, req, retryCount+1, maxRetry, waitRetry, condRetry)
	}

	// Run retry if status code is not 2xx or condRetry is true
	if resp.StatusCode < 200 || resp.StatusCode >= 300 || (condRetry != nil && condRetry(resp)) {
		body, err := retry()
		if err != nil {
			return nil, err
		}
		return body, nil
	}

	return body, nil
}
