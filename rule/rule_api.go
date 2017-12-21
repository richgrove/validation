package rule

import (
	"fmt"
	"encoding/json"
	"io"
	"net/http"
	"github.com/go-chi/chi"
)

// rule API service route
// use https://github.com/go-chi/chi lightweight http router to build REST services
func Handlers() *chi.Mux {
	r := chi.NewRouter()

	// specify /api/validation route
	r.Post("/api/validation", ValidateJSONData)

	// rule manipulation service: only support CreateRule() and DeleteRule((
	r.Route("/admin/rule", func(r chi.Router) {
		// POST /admin/rule
		r.Post("/", CreateRule)
		// DELETE /admin/rule/password_length
		r.Route("/{ruleName}", func(r chi.Router) {
			r.Delete("/", DeleteRule)
		})
	})

	return r
}

// validationResult collects a JSON processing result
type validationResult struct {
	flag  bool      // succ/fail
	rules []string  // violated rule names
}

const (
	ValidationStatusSucc  = "success"
	ValidationStatusFail  = "failure"
	ValidationStatusError = "error"

	RuleMgmtError = "error"
	RuleMgmtSucc  = "success"
)

// define validation API service result messages
type ResponseMsg struct {
	Result string `json:"result"`
}
type FailResponseMsg struct {
	Result string   `json:"result"`
	Rules  []string `json:"rules"`
}
type ErrResponseMsg struct {
	Result   string `json:"result"`
	ErrorMsg string `json:"error-message"`
}

// POST /api/validation service implementation
func ValidateJSONData(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	decoder := json.NewDecoder(r.Body)
	defer r.Body.Close()

	var f map[string]interface{}
	err := decoder.Decode(&f)
	if err != nil {
		fmt.Errorf("API service data error, %s", err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		errMsg := ErrResponseMsg{Result: ValidationStatusError, ErrorMsg: err.Error()}
		result, _ := json.Marshal(errMsg)
		io.WriteString(w, string(result))
		return
	}
	// parse input JSON and run the validation
	if result, e := ValidateInputJSONByRules(f); e != nil {
		// internal error
		fmt.Errorf("API service internal error, %s", e.Error())
		w.WriteHeader(http.StatusInternalServerError)
		errMsg := ErrResponseMsg{Result: ValidationStatusError, ErrorMsg: e.Error()}
		result, _ := json.Marshal(errMsg)
		io.WriteString(w, string(result))
		return
	} else {
		// handle the validation result for the API response
		if result.flag {
			// succ
			w.WriteHeader(http.StatusOK)
			res := ResponseMsg{Result: ValidationStatusSucc}
			resStr, _ := json.Marshal(res)
			io.WriteString(w, string(resStr))

		} else {
			// fail
			w.WriteHeader(http.StatusBadRequest)
			fail := FailResponseMsg{Result: ValidationStatusFail, Rules: result.rules}
			resStr, _ := json.Marshal(fail)
			io.WriteString(w, string(resStr))
		}
	}
}

func generateCreateRuleErrorMessage(err error) string {
	fmt.Errorf("rule management service error, %s", err.Error())
	errMsg := ErrResponseMsg{Result: RuleMgmtError, ErrorMsg: err.Error()}
	result, _ := json.Marshal(errMsg)
	return string(result)
}
func CreateRule(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	decoder := json.NewDecoder(r.Body)
	defer r.Body.Close()

	rule := RuleNode{}

	if err := decoder.Decode(&rule); err != nil {
		// failed to decode a JSON block
		w.WriteHeader(http.StatusInternalServerError)
		io.WriteString(w, generateCreateRuleErrorMessage(err))
		return
	}

	// parse one rule in r
	fieldList := map[string]int{}
	if operd, e := ConstructOperandListHelper(&rule.RuleContent, fieldList); e != nil {
		w.WriteHeader(http.StatusInternalServerError)
		io.WriteString(w, generateCreateRuleErrorMessage(e))
		return
	} else {
		// prepare to add rule
		if err := SaveRuleToRegister(operd, rule.Name, fieldList); err != nil {
			// save failed
			w.WriteHeader(http.StatusInternalServerError)
			io.WriteString(w, generateCreateRuleErrorMessage(err))
		} else {
			// success
			w.WriteHeader(http.StatusOK)
			res := ResponseMsg{Result: RuleMgmtSucc}
			resStr, _ := json.Marshal(res)
			io.WriteString(w, string(resStr))
		}
	}
}

func DeleteRule(w http.ResponseWriter, r *http.Request) {
	// TBD
}
