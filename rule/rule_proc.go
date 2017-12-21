package rule

import (
	"errors"
	"fmt"
	"reflect"
)

// helper parses input JSON string map in fieldData, and collect
// <fieldName, fieldValue> pairs in fields.
func parseInputJSON(fields map[string]string, fieldPrefix string, fieldData map[string]interface{}) error {
	// process the collected fieldData
	for k, v := range fieldData {

		if reflect.ValueOf(v).Kind() == reflect.String {
			fieldName := fieldPrefix + k
			if _, exists := fields[fieldName]; exists {
				// there are duplicated field names
				return fmt.Errorf("parse input JSON: duplicated field name, %s", fieldName)
			} else {
				fields[fieldName] = v.(string)
			}
		} else if reflect.ValueOf(v).Kind() == reflect.Map {
			var prefix string
			if len(fieldPrefix) == 0 {
				prefix = k + "."
			} else {
				prefix = fieldPrefix + "." + k + "."
			}
			if e := parseInputJSON(fields, prefix, v.(map[string]interface{})); e != nil {
				return e
			}
		} else if reflect.ValueOf(v).Kind() == reflect.Slice {
			slc := v.([]interface{})
			var prefix string
			if len(fieldPrefix) == 0 {
				prefix = k + "."
			} else {
				prefix = fieldPrefix + "." + k + "."
			}
			for i := 0; i < len(slc); i++ {
				if reflect.ValueOf(slc[i]).Kind() == reflect.Map {
					if e := parseInputJSON(fields, prefix, slc[i].(map[string]interface{})); e != nil {
						return e
					}
				} else {
					// ignore
				}
			}
		} else {
			// unknown type
			return errors.New("parse input JSON: unknown field type")
		}
	}
	return nil
}

// validation processing
func ValidateInputJSONByRules(input interface{}) (*validationResult, error) {
	result := validationResult{}
	inputFields := make(map[string]string)

	// generate the collection <fieldName, fieldValue> into inputFields
	// from input, include the nested JSON block fields
	if err := parseInputJSON(inputFields, "", input.(map[string]interface{})); err != nil {
		return nil, err
	}

	// create the FieldEvalContext for each field which does have at least one rule defined
	// inputRuntimeContexts with all data to fine the rule validation
	inputRuntimeContexts := make([]FieldEvalContext, 0)
	RegRuleLock.RLock()  // register rule READ lock
	for k, v := range inputFields {
		if rules := AllRegisteredRules[k]; rules != nil {
			for name, rule := range rules {
				ctx := FieldEvalContext{RuleName: name, FieldValue: v, Rule: rule}
				inputRuntimeContexts = append(inputRuntimeContexts, ctx)
			}
		}
	}
	RegRuleLock.RUnlock() // READ unlock

	// run JSON field evaluation
	// all required validate fields are collected in inputRuntimeContexts, and
	// each FieldEvalContext has independent runtime data:
	//       <rule-name, field-value, Rule-func block(pointer)>
	// result is aggregated to collect in the loop in validationResult struct for
	// API response
	result.flag = true
	for i := 0; i < len(inputRuntimeContexts); i++ {
		operand := inputRuntimeContexts[i].Rule
		if res, err := operand.Evaluate(&inputRuntimeContexts[i]); err != nil {
			fmt.Println(err)
		} else {
			if !res.(bool) {
				result.flag = res.(bool)
				result.rules = append(result.rules, inputRuntimeContexts[i].RuleName)
			}
		}
	}
	return &result, nil
}
