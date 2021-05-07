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

type fakeHttpRequester struct {
	responseBodyJSON string
	err              error
}

func (c *fakeHttpRequester) Get(targetURL string) (*http.Response, error) {
	r := ioutil.NopCloser(bytes.NewReader([]byte(c.responseBodyJSON)))
	return &http.Response{Body: r}, c.err
}

type fakeRequestObserver struct{}

func (c *fakeRequestObserver) ObserveHTTPRequest(label string, duration time.Duration) {}

func Test_SignRequest(t *testing.T) {
	tests := []struct {
		name              string
		BusinessKey       *BusinessKey
		URL               string
		Language          string
		client            *fakeHttpRequester
		expectedSignature string
		expectedError     error
	}{
		{
			"Test Signing",
			&BusinessKey{ClientID: "my_test_client", SigningKey: "bXlfdGVzdF9rZXk=", Channel: "grg-local"},
			"/maps/api/geocode/xml?latlng=49.17584440,7.30196070&sensor=false&client=my_test_client&channel=grg-local",
			"en",
			&fakeHttpRequester{},
			"fGNFKf3Yt6Syb9dRF42E7vm1FwM=",
			nil,
		},
		{
			"Test Signing 1",
			&BusinessKey{ClientID: "my_test_client", SigningKey: "bXlfdGVzdF9rZXk=", Channel: "grg-local"},
			"/maps/api/geocode/json?channel=grg-local&client=my_test_client&language=en&latlng=45.32000000%2C12.67000000&sensor=false",
			"en",
			&fakeHttpRequester{},
			"bdwh-bmlibC2w2N_A2tgt7pSuAE=",
			nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Log(tt.name)

			geocoder, _ := NewGeocoder(tt.BusinessKey, tt.URL, tt.Language, tt.client, 10, time.Second, &fakeRequestObserver{})
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
		client                 *fakeHttpRequester
		overQueryLimitDuration time.Duration
		expectedResponse       *GoogleResponse
		expectedError          error
	}{
		{
			"Should return error on parsing failure",
			&BusinessKey{ClientID: "my_test_client", SigningKey: "bXlfdGVzdF9rZXk=", Channel: "grg-local"},
			"https://maps.googleapis.com/maps/api/geocode/json",
			"en",
			&fakeHttpRequester{responseBodyJSON: "", err: errors.New("failed")},
			time.Nanosecond,
			nil,
			errors.New("failed"),
		},
		{
			"Should return OVER_QUERY_LIMIT",
			&BusinessKey{ClientID: "my_test_client", SigningKey: "bXlfdGVzdF9rZXk=", Channel: "grg-local"},
			"https://maps.googleapis.com/maps/api/geocode/json",
			"en",
			&fakeHttpRequester{responseBodyJSON: `{"status":"OVER_QUERY_LIMIT"}`},
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

func Test_buildURL(t *testing.T) {
	tests := []struct {
		name          string
		BusinessKey   *BusinessKey
		URL           string
		Language      string
		client        *fakeHttpRequester
		lat           float64
		lng           float64
		expectedURL   string
		expectedError error
	}{
		{
			"Should build url",
			&BusinessKey{ClientID: "my_test_client", SigningKey: "bXlfdGVzdF9rZXk=", Channel: "grg-local"},
			"https://maps.googleapis.com/maps/api/geocode/json",
			"en",
			&fakeHttpRequester{},
			45.32,
			12.67,
			"https://maps.googleapis.com/maps/api/geocode/json?channel=grg-local&client=my_test_client&language=en&latlng=45.32000000%2C12.67000000&sensor=false&signature=bdwh-bmlibC2w2N_A2tgt7pSuAE%3D",
			nil,
		},
		{
			"Should build escaped url",
			&BusinessKey{ClientID: "my&test&client", SigningKey: "bXlfdGVzdF9rZXk=", Channel: "grg-local!@#$%^&*() "},
			"https://maps.googleapis.com/maps/api/geocode/json",
			"en",
			&fakeHttpRequester{},
			45.32,
			12.67,
			"https://maps.googleapis.com/maps/api/geocode/json?channel=grg-local%21%40%23%24%25%5E%26%2A%28%29+&client=my%26test%26client&language=en&latlng=45.32000000%2C12.67000000&sensor=false&signature=Ui0NkXF9aJEZHtjQ-H1-V333LUk%3D",
			nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Log(tt.name)

			geocoder, _ := NewGeocoder(tt.BusinessKey, tt.URL, tt.Language, tt.client, 10, time.Second, &fakeRequestObserver{})
			res, err := geocoder.buildURL(tt.lat, tt.lng)

			if res.String() != tt.expectedURL {
				t.Errorf("test for %v Failed - results not match\nGot:\n%v\nExpected:\n%v", tt.name, res.String(), tt.expectedURL)
			}

			if err != nil && tt.expectedError != nil && tt.expectedError.Error() != err.Error() {
				t.Errorf("test for %v Failed - results not match\nGot:\n%v\nExpected:\n%v", tt.name, err, tt.expectedError)
			}
		})
	}
}
