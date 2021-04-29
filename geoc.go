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

type HTTPClient interface {
	Get(targetURL string) (*http.Response, error)
}

type Geocoder struct {
	// Google BusinessKey
	businessKey *BusinessKey
	// Geocoding URL, e.g. https://maps.googleapis.com/maps/api/geocode/json
	baseURL string
	// Set language to control output language of the geocoder. Leave empty to keep default behavior
	language string
	// HTTP Client
	client HTTPClient
	// Requests per second
	rps int
	// Sleep interval if OVER_QUERY_LIMIT status has been received
	overQuerySleepDuration time.Duration
	limiter                *rate.Limiter
}

// NewGeocoder creates new instance of Geocoder
func NewGeocoder(bkey *BusinessKey, baseURL, language string, client HTTPClient,
	requestPerSecond int, overQuerySleepDuration time.Duration) (*Geocoder, error) {
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
	targetURL := g.buildURL(lat, lng)

	ur, err := url.Parse(targetURL)
	if err != nil {
		return nil, err
	}

	signature, err := g.getSignature(ur.Path + "?" + ur.RawQuery)
	if err != nil {
		return nil, err
	}
	targetURL += "&signature=" + signature

	resp, err := g.client.Get(targetURL)
	if err != nil {
		return nil, err
	}

	var res *GoogleResponse
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if res.Status == GRS_OVER_QUERY_LIMIT {
		g.limiter.SetLimit(rate.Limit(0))
		time.Sleep(g.overQuerySleepDuration)
		g.limiter.SetLimit(rate.Limit(g.rps))
	}

	return res, nil
}

func (g *Geocoder) buildURL(lat, lng float64) string {
	result := g.baseURL + "?latlng=" + fmt.Sprintf("%.8f,%.8f", lat, lng) + "&sensor=false"

	if g.language != "" {
		result += "&language=" + url.QueryEscape(g.language)
	}

	if g.businessKey != nil {
		result += "&client=" + url.QueryEscape(g.businessKey.ClientID)
		if g.businessKey.Channel != "" {
			result += "&channel=" + url.QueryEscape(g.businessKey.Channel)
		}
	}
	return result
}

func (g *Geocoder) getSignature(targetURL string) (string, error) {
	sKey := strings.ReplaceAll(g.businessKey.SigningKey, "-", "+")
	sKey = strings.ReplaceAll(sKey, "_", "/")

	signingKeyBytes, err := base64.StdEncoding.DecodeString(sKey)
	if err != nil {
		return "", err
	}

	h := hmac.New(sha1.New, signingKeyBytes)
	_, err = h.Write([]byte(targetURL))

	hash := base64.StdEncoding.EncodeToString(h.Sum(nil))
	hash = strings.ReplaceAll(hash, "+", "-")
	hash = strings.ReplaceAll(hash, "/", "_")

	return hash, err
}
