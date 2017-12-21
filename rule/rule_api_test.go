package rule

import (
	"net/http"
	//"net/http/httptest"
	"strings"
	"testing"
	"io"
	"io/ioutil"
)

var testCases = []struct {
	description string
	jsonDataRequest string
	expected string
	statusCode int
}{
	// test case 1:  validation success
	{
		description: "test case 1:  validation success",
		jsonDataRequest: `
			{
				"username": "bwillis",
				"password": "",
				"first_name": "Bruce",
				"last_name": "Willis",
				"date_of_birth": "03/19/1955",
				"email": "bruce@willis.com",
				"phone": "424-288-2000",
				"address": {
					"street": "2000 Avenue Of The Stars",
					"city": "Los Angeles",
					"state": "CA",
					"zip_code": "90067"
				}
			}`,
		expected: `{"result":"success"}`,
		statusCode: http.StatusOK,
	},

	// test case 2: validation failure with phone_pattern
	{
		description: "test case 2: validation failure with phone_pattern",
		jsonDataRequest: `
			{
				"username": "bwillis",
				"password": "",
				"first_name": "Bruce",
				"last_name": "Willis",
				"date_of_birth": "03/19/1955",
				"email": "bruce@willis.com",
				"phone": "42R-288-2000",
				"address": {
					"street": "2000 Avenue Of The Stars",
					"city": "Los Angeles",
					"state": "CA",
					"zip_code": "90067"
				}
			}`,
		expected: `{"result":"failure","rules":["phone_pattern"]}`,
		statusCode: http.StatusBadRequest,
	},

	// test case 3: validation failure with zip_code_pattern, password_length
	{
		description: "test case 3: validation failure with zip_code_pattern, password_length",
		jsonDataRequest: `
			{
				"username": "bwillis",
				"password": "tesTer",
				"first_name": "Bruce",
				"last_name": "Willis",
				"date_of_birth": "03/19/1955",
				"email": "bruce@willis.com",
				"phone": "424-288-2000",
				"address": {
					"street": "2000 Avenue Of The Stars",
					"city": "Los Angeles",
					"state": "CA",
					"zip_code": "9o067"
				}
			}`,
		expected: `{"result":"failure","rules":["zip_code_pattern","password_length"]}`,
		statusCode: http.StatusBadRequest,
	},

	// test case 4: validation failure with username_length
	{
		description: "test case 4: validation failure with username_length",
		jsonDataRequest: `
			{
				"username": "bill",
				"password": "",
				"first_name": "Bruce",
				"last_name": "Willis",
				"date_of_birth": "03/19/1955",
				"email": "bruce@willis.com",
				"phone": "424-288-2000",
				"address": {
					"street": "2000 Avenue Of The Stars",
					"city": "Los Angeles",
					"state": "CA",
					"zip_code": "90067"
				}
			}`,
		expected: `{"result":"failure","rules":["username_length"]}`,
		statusCode: http.StatusBadRequest,
	},
}

var (
	//server *httptest.Server
	reader io.Reader
)

func TestRuleApiService(t *testing.T) {
	//server = httptest.NewServer(Handlers())
	//defer server.Close()

	// run all test cases from testCases[]
	for _, tc := range testCases {
		reader = strings.NewReader(tc.jsonDataRequest)

		request, err := http.NewRequest("POST", "http://localhost:8000/api/validation", reader)
		res, err := http.DefaultClient.Do(request)
		if err != nil {
			t.Error(err)
		}

		t.Log(tc.description)
		if res.StatusCode != tc.statusCode {
			t.Errorf("\tFAILED: HTTP Response status code not expected %d", tc.statusCode)
		} else {
			t.Logf("\tPASS: HTTP Response status code expected %d", res.StatusCode)
			if bodyByte, e := ioutil.ReadAll(res.Body); e == nil {
				t.Logf("\t\t%s", string(bodyByte))
				res.Body.Close()
			}
		}
	}
}