package geocoder

import (
	"context"
	"crypto/hmac"
	"crypto/sha1" //nolint
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/time/rate"
)

type HttpRequester interface {
	Get(targetURL string) (*http.Response, error)
}

type RequestObserver interface {
	ObserveHTTPRequest(label string, duration time.Duration)
}

type Geocoder struct {
	// Google BusinessKey
	businessKey *BusinessKey
	// Geocoding URL, e.g. https://maps.googleapis.com/maps/api/geocode/json
	baseURL string
	// Set language to control output language of the geocoder. Leave empty to keep default behavior
	language string
	// HTTP Client
	client HttpRequester
	// Requests per second
	rps int
	// Sleep interval if OVER_QUERY_LIMIT status has been received
	overQuerySleepDuration time.Duration
	// Measures HTTP requests duration
	observer RequestObserver
	limiter  *rate.Limiter
}

// NewGeocoder creates new instance of Geocoder
func NewGeocoder(bkey *BusinessKey, baseURL, language string, client HttpRequester,
	requestPerSecond int, overQuerySleepDuration time.Duration, observer RequestObserver) (*Geocoder, error) {
	if bkey == nil {
		return nil, errors.New("empty BusinessKey")
	}
	if baseURL == "" {
		return nil, errors.New("empty baseURL, use https://maps.googleapis.com/maps/api/geocode/json")
	}
	if client == nil {
		return nil, errors.New("empty HTTPClient")
	}
	if requestPerSecond <= 0 {
		return nil, errors.New("requestPerSecond must be a positive number")
	}
	return &Geocoder{
			bkey,
			baseURL,
			language,
			client,
			requestPerSecond,
			overQuerySleepDuration,
			observer,
			rate.NewLimiter(rate.Limit(requestPerSecond), 1)},
		nil
}

// ReverseGeocode makes reverse geocoding against latitude, longitude and returns GoogleResponse.
// The number of requests per second is respected
func (g *Geocoder) ReverseGeocode(ctx context.Context, lat, lng float64) (*GoogleResponse, error) {
	err := g.limiter.Wait(ctx)
	if err != nil {
		return nil, err
	}
	ur, err := g.buildURL(lat, lng)
	if err != nil {
		return nil, err
	}

	t := time.Now()
	resp, err := g.client.Get(ur.String())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if g.observer != nil {
		g.observer.ObserveHTTPRequest("google", time.Since(t))
	}

	var res *GoogleResponse
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, err
	}

	if res.Status == GRS_OVER_QUERY_LIMIT {
		g.limiter.SetLimit(rate.Limit(0))
		time.Sleep(g.overQuerySleepDuration)
		g.limiter.SetLimit(rate.Limit(g.rps))
	}

	return res, nil
}

// buildURL constructs url for further reverse geocode request
func (g *Geocoder) buildURL(lat, lng float64) (*url.URL, error) {
	ur, err := url.Parse(g.baseURL)
	if err != nil {
		return nil, err
	}

	query := url.Values{}
	query.Add("latlng", fmt.Sprintf("%.8f,%.8f", lat, lng))
	query.Add("sensor", "false")
	if g.language != "" {
		query.Add("language", g.language)
	}
	if g.businessKey != nil {
		query.Add("client", g.businessKey.ClientID)
		if g.businessKey.Channel != "" {
			query.Add("channel", g.businessKey.Channel)
		}
	}

	ur.RawQuery = query.Encode()

	signature, err := g.getSignature(ur.Path + "?" + ur.RawQuery)
	if err != nil {
		return nil, err
	}

	query.Add("signature", signature)
	ur.RawQuery = query.Encode()

	return ur, nil
}

// getSignature returns a signature of the targetURL using Google client's signing key
func (g *Geocoder) getSignature(targetURL string) (string, error) {
	sKey := strings.ReplaceAll(g.businessKey.SigningKey, "-", "+")
	sKey = strings.ReplaceAll(sKey, "_", "/")

	signingKeyBytes, err := base64.StdEncoding.DecodeString(sKey)
	if err != nil {
		return "", err
	}

	h := hmac.New(sha1.New, signingKeyBytes)
	_, err = h.Write([]byte(targetURL))
	if err != nil {
		return "", err
	}

	hash := base64.StdEncoding.EncodeToString(h.Sum(nil))
	hash = strings.ReplaceAll(hash, "+", "-")
	hash = strings.ReplaceAll(hash, "/", "_")

	return hash, nil
}
