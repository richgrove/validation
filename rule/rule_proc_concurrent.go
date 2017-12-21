package rule

import (
	"fmt"
	"github.com/richgrove/validation/util"
)

type ValidatorState struct {
	flag  bool
	rules []string
}

// Task executor uses CombineResult() to aggregate all results generated
// by each concurrent validators in the reducer step
func (s ValidatorState) CombineResult(state util.ExecutorResult) util.ExecutorResult {
	r := state.(ValidatorState)
	if s.flag {
		// keep the "false" pass
		s.flag = r.flag
	}
	s.rules = append(s.rules, r.rules...)
	return s
}

// createValidatorExecutor() helper creates a executor by FieldEvalContext
func createValidatorExecutor(ctx *FieldEvalContext) util.Executor {
	return func(data interface{}) util.ExecutorResult {
		ret := ValidatorState{}
		operand := ctx.Rule
		//fmt.Printf("rule name: %s\n", ctx.RuleName)
		if res, err := operand.Evaluate(ctx); err != nil {
			fmt.Errorf("validator executor evaluation error, %s", err.Error())
		} else {
			ret.flag = res.(bool)
			if !ret.flag {
				ret.rules = append(ret.rules, ctx.RuleName)
			}
		}
		return ret
	}
}

type ValidationTask struct {
	inputRuntimeContexts []FieldEvalContext
}

func (v *ValidationTask) GetTaskData() interface{} {
	return 1 // ignore task data
}
func (v *ValidationTask) GetMaxTimeToCompleteInSecond() int {
	return -1 // run for completeness
}
func (v *ValidationTask) GetAllExecutors() []util.Executor {
	// assemble executorList from inputRuntimeContexts, and
	// createValidatorExecutor() helper creates a executor by FieldEvalContext
	var executorList = []util.Executor{}
	for i := 0; i < len(v.inputRuntimeContexts); i++ {
		executorList = append(executorList, createValidatorExecutor(&v.inputRuntimeContexts[i]))
	}
	return executorList
}

// validation processing in concurrency mode, used AppTaskExecutor pipeline in fan-out
func ValidateInputJSONByRules2(input interface{}) (*validationResult, error) {
	result := validationResult{}
	inputFields := make(map[string]string)

	// generate the collection <fieldName, fieldValue> into inputFields
	// from input, include the nested JSON block fields
	if err := parseInputJSON(inputFields, "", input.(map[string]interface{})); err != nil {
		return nil, err
	}

	// create the FieldEvalContext for each field which does have at least one rule defined.
	// all required validate fields are collected in inputRuntimeContexts, and
	// each FieldEvalContext has independent runtime data:
	//       <rule-name, field-value, Rule-func block(pointer)>
	// and pack to task
	task := ValidationTask{}
	RegRuleLock.RLock()  // register rule READ lock
	for k, v := range inputFields {
		if rules := AllRegisteredRules[k]; rules != nil {
			for name, rule := range rules {
				ctx := FieldEvalContext{RuleName: name, FieldValue: v, Rule: rule}
				task.inputRuntimeContexts = append(task.inputRuntimeContexts, ctx)
			}
		}
	}
	RegRuleLock.RUnlock()  // READ unlock

	// run JSON field evaluation
	// ExecuteAppTask() runs them concurrently, and its reducer collects them
	// results into state (includes flag, and failed rule names.
	if state, e := util.ExecutAppTask(&task, ValidatorState{flag: true}); e != nil {
		return nil, e
	} else {
		// convert to validationResult for the API response
		result.flag = state.(ValidatorState).flag
		result.rules = state.(ValidatorState).rules
		return &result, nil
	}
}
