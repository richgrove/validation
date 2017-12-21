package main

import (
	"net/http"
	"github.com/richgrove/validation/rule"
)

func main() {
	// serve at port 8000 for API services:
	//  POST /api/validation   validate a JSON
	//  POST /admin/rule                  create a rule
	//  DELETE /admin/rule/<rule-name>    delete a rule
	http.ListenAndServe(":8000", rule.Handlers())
}
