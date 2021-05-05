package geocoder

import (
	"bytes"
	"context"
	"errors"
	"io/ioutil"
	"net/http"
	"reflect"
	"sync"
	"testing"
	"time"
)

type FakeHTTPClient struct {
	responseBodyJSON string
	err              error
}

func (c *FakeHTTPClient) Get(targetURL string) (*http.Response, error) {
	r := ioutil.NopCloser(bytes.NewReader([]byte(c.responseBodyJSON)))
	return &http.Response{Body: r}, c.err
}

type FakeRequestObserver struct{}

func (c *FakeRequestObserver) ObserveHTTPRequest(label string, duration time.Duration) {}

func Test_SignRequest(t *testing.T) {
	tests := []struct {
		name              string
		BusinessKey       *BusinessKey
		URL               string
		Language          string
		client            *FakeHTTPClient
		expectedSignature string
		expectedError     error
	}{
		{
			"Test Signing",
			&BusinessKey{ClientID: "my_test_client", SigningKey: "bXlfdGVzdF9rZXk=", Channel: "grg-local"},
			"/maps/api/geocode/xml?latlng=49.17584440,7.30196070&sensor=false&client=my_test_client&channel=grg-local",
			"en",
			&FakeHTTPClient{},
			"fGNFKf3Yt6Syb9dRF42E7vm1FwM=",
			nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Log(tt.name)

			geocoder, _ := NewGeocoder(tt.BusinessKey, tt.URL, tt.Language, tt.client, 10, time.Second, &FakeRequestObserver{})
			res, err := geocoder.getSignature(tt.URL)

			if res != tt.expectedSignature {
				t.Errorf("test for %v Failed - results not match\nGot:\n%v\nExpected:\n%v", tt.name, res, tt.expectedSignature)
			}

			if err != nil && tt.expectedError != nil && tt.expectedError.Error() != err.Error() {
				t.Errorf("test for %v Failed - results not match\nGot:\n%v\nExpected:\n%v", tt.name, err, tt.expectedError)
			}
		})
	}
}

func Test_ReverseGeocode(t *testing.T) {
	tests := []struct {
		name                   string
		BusinessKey            *BusinessKey
		URL                    string
		Language               string
		client                 *FakeHTTPClient
		overQueryLimitDuration time.Duration
		expectedResponse       *GoogleResponse
		expectedError          error
	}{
		{
			"Should return error on parsing failure",
			&BusinessKey{ClientID: "my_test_client", SigningKey: "bXlfdGVzdF9rZXk=", Channel: "grg-local"},
			"https://maps.googleapis.com/maps/api/geocode/json",
			"en",
			&FakeHTTPClient{responseBodyJSON: "", err: errors.New("failed")},
			time.Nanosecond,
			nil,
			errors.New("failed"),
		},
		{
			"Should return OVER_QUERY_LIMIT",
			&BusinessKey{ClientID: "my_test_client", SigningKey: "bXlfdGVzdF9rZXk=", Channel: "grg-local"},
			"https://maps.googleapis.com/maps/api/geocode/json",
			"en",
			&FakeHTTPClient{responseBodyJSON: `{"status":"OVER_QUERY_LIMIT"}`},
			time.Millisecond * 100,
			&GoogleResponse{Status: GRS_OVER_QUERY_LIMIT},
			nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Log(tt.name)

			geocoder, _ := NewGeocoder(tt.BusinessKey, tt.URL, tt.Language, tt.client, 5, tt.overQueryLimitDuration, nil)
			var wg sync.WaitGroup

			for i := 0; i < 5; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					now := time.Now()
					res, err := geocoder.ReverseGeocode(context.TODO(), 49.17584440, 7.30196070)

					if tt.overQueryLimitDuration > time.Nanosecond && -time.Until(now) <= tt.overQueryLimitDuration {
						t.Errorf("test for %v Failed - test took less time than expected\nGot:\n%v\nExpected:\n%v", tt.name, -time.Until(now), tt.overQueryLimitDuration)
					}
					if !reflect.DeepEqual(res, tt.expectedResponse) {
						t.Errorf("test for %v Failed - results not match\nGot:\n%v\nExpected:\n%v", tt.name, res, tt.expectedResponse)
					}

					if err != nil && tt.expectedError != nil && tt.expectedError.Error() != err.Error() {
						t.Errorf("test for %v Failed - results not match\nGot:\n%v\nExpected:\n%v", tt.name, err, tt.expectedError)
					}
				}()
			}
			wg.Wait()
		})
	}
}
