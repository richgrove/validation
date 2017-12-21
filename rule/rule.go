package rule

import (
	"encoding/json"
	"errors"
)

var ParseRuleOperatorError = errors.New("rule parser: incorrect operands")
var ParseRuleJsonDecodingError = errors.New("rule parser: JSON unmarshal invalid object value")
var ParseRuleUnknownOperatorError = errors.New("rule parser: JSON unmarshal unknown operator")

// evaluation context: keep the run-time state
type EvalContext interface {
	GetFieldValue() interface{}
}

// run-time field evaluation context
type FieldEvalContext struct {
	RuleName   string
	FieldValue string
	Rule       Operand
}

func (context *FieldEvalContext) GetFieldValue() interface{} {
	return context.FieldValue
}

// rule operator functor, evaluates []interface{} data to
// generate a single value, interface{}
type OperatorFn func([]interface{}) (interface{}, error)

// system built-in operator in OperatorType
type OperatorType string

const (
	LengthOperator      OperatorType = "LENGTH"
	EqualToOperator     OperatorType = "EQUAL_TO"
	GreaterThanOperator OperatorType = "GREATER_THAN"
	OrOperator          OperatorType = "OR"
	AndOperator         OperatorType = "AND"
	RegexMatchOperator  OperatorType = "REGEX_MATCH"
)

// Operand has the capability to be evaluated by Evaluate() function,
// it can be either a terminated operand like FieldOperand/ValueOperand
// or recursive defined operand like TermOperand.
type Operand interface {
	GetOperator() *OperatorFn
	GetOperands() []Operand
	Evaluate(EvalContext) (interface{}, error)
}

// FieldOperand defines an operand to return field value
// when evaluate it.  Given field value is defined in EvalContext.
// JSON block like,
//     { "field", _field_name_ }
type FieldOperand struct {
	Name string `json:"field"`
}

func (*FieldOperand) GetOperator() *OperatorFn {
	return nil
}
func (*FieldOperand) GetOperands() []Operand {
	return nil
}
func (*FieldOperand) Evaluate(cx EvalContext) (interface{}, error) {
	return cx.GetFieldValue(), nil
}

// ValueOperand defines an operand to evaluate the value literal,
// which is recorded when parse JSON block like,
//       { "value": _value_literal_ }
type ValueOperand struct {
	Value string `json:"value"`
}

func (*ValueOperand) GetOperator() *OperatorFn {
	return nil
}
func (*ValueOperand) GetOperands() []Operand {
	return nil
}
func (v *ValueOperand) Evaluate(cx EvalContext) (interface{}, error) {
	return v.Value, nil
}

// TermOperand as a function definition, OperatorFn( OperandList ).
// When TermOperand is parsed, the JSON block like,
//    { "operator": OperatorType, "operands":  [ _operand_, ...]
// JSON unmarshalJSON records the parsed result in []Term slice.
// Evaluate() is executed when all OperandList items are evaluated.
type TermOperand struct {
	ParseOperator string `json:"operator"`
	ParseOperands []Term `json:"operands"`
	OperatorFn    *OperatorFn
	OperandList   []Operand
}

func (t *TermOperand) GetOperator() *OperatorFn {
	return t.OperatorFn
}
func (t *TermOperand) GetOperands() []Operand {
	return t.OperandList
}
func (t *TermOperand) Evaluate(cx EvalContext) (interface{}, error) {
	length := len(t.GetOperands())
	if length == 0 {
		// no operands evaluated
		return nil, nil
	}

	evalResult := make([]interface{}, length)
	for i, ops := range t.GetOperands() {
		if v, e := ops.Evaluate(cx); e != nil {
			// ops evaluate failed w/ e
			return nil, e
		} else {
			evalResult[i] = v
		}
	}

	return (*(t.GetOperator()))(evalResult)
}

// Term is used to record the UnmarshalJSON temporary result.
// It may need to re-visit to consider the performance.
// Current implementation re-used the Golang encoding/json
// Unmarshal mechanism.
type Term struct {
	Value interface{}
}

// RuleNode is used to parse one validation rule with "name" and "rule" content
type RuleNode struct {
	Name        string `json:"name"`
	RuleContent Term   `json:"rule"`
}

// Customized Term decoding to handle,
//   FieldOperand,  { "field": ... }
//   ValueOperand,  { "value": ... }
//   TermOperand,   { "operator": ..., "operands": [ ... ] }
func (t *Term) UnmarshalJSON(data []byte) error {
	var f interface{}
	json.Unmarshal(data, &f)
	m := f.(map[string]interface{})

	if _, ok := m["field"]; ok {
		// parse field operand,
		// { "field": _field_name_ }
		field := FieldOperand{}
		if err := json.Unmarshal(data, &field); err != nil {
			// failed to parse "field"
			return ParseRuleJsonDecodingError
		}
		t.Value = field
		return nil
	}

	if _, ok := m["value"]; ok {
		// parse value operand,
		// { "value": _value_literal_ }
		value := ValueOperand{}
		if err := json.Unmarshal(data, &value); err != nil {
			// failed to parse "value"
			return ParseRuleJsonDecodingError
		}
		t.Value = value
		return nil
	}

	if _, ok := m["operator"]; ok {
		// parse term operand,
		// { "operator":  _operator_literal_, "operands": [ _operand_, ...] }
		term := TermOperand{}
		if err := json.Unmarshal(data, &term); err != nil {
			// failed to parse "operator"
			return ParseRuleJsonDecodingError
		}
		// check the _operator_literal_ registered or not
		if fn, ok := RegisteredOperators[OperatorType(term.ParseOperator)]; ok {
			term.OperatorFn = &fn
			t.Value = term
			return nil
		} else {
			return ParseRuleUnknownOperatorError
		}
	}

	// unknown JSON block
	return ParseRuleJsonDecodingError
}
