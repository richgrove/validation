package rule

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"sync"
)

const (
	ruleJsonDefinitionFileName = "./rules.json"
)

// registered rule is, ruleName => Operand
// ruleName is unique
type RegisteredRule map[string]Operand

// AllRegisteredRules is collection of (dataFieldName, [RegisteredRule, ...])
// given a data field name may be defined with multiple rules, e.g.
// "password" field:
//   rule1 - length is 0 OR length > 6
//   rule2 - contains letter, digital, one special character in a regex pattern
var AllRegisteredRules = map[string]RegisteredRule{}
// define registered rules RWMutex lock
var RegRuleLock = sync.RWMutex{}

// all registered operators in OperatorFn
var RegisteredOperators map[OperatorType]OperatorFn

func init() {

	// prepare built-in operators
	RegisteredOperators = map[OperatorType]OperatorFn{

		// calc the length of a string
		LengthOperator: func(operands []interface{}) (interface{}, error) {
			if len(operands) != 1 {
				return nil, ParseRuleOperatorError
			}
			switch v := operands[0].(type) {
			case string:
				return len(v), nil
			default:
				return nil, ParseRuleOperatorError
			}
		},

		// compare two values equal w/ the same type, such as string or int
		EqualToOperator: func(operands []interface{}) (interface{}, error) {
			if len(operands) != 2 {
				return nil, ParseRuleOperatorError
			}
			var v1, v2 string
			switch v := operands[0].(type) {
			case string:
				v1 = v
			case int:
				v1 = strconv.Itoa(v)
			}
			switch v := operands[1].(type) {
			case string:
				v2 = v
			case int:
				v2 = strconv.Itoa(v)
			}
			return strings.Compare(v1, v2) == 0, nil
		},

		// compare two number values in >
		GreaterThanOperator: func(operands []interface{}) (interface{}, error) {
			if len(operands) != 2 {
				return nil, ParseRuleOperatorError
			}
			var v1, v2 int
			var err error
			switch v := operands[0].(type) {
			case string:
				if v1, err = strconv.Atoi(v); err != nil {
					return nil, err
				}
			case int:
				v1 = v
			}
			switch v := operands[1].(type) {
			case string:
				if v2, err = strconv.Atoi(v); err != nil {
					return nil, err
				}
			case int:
				v2 = v
			}
			return v1 > v2, nil
		},

		// do the logic OR on two bool values
		OrOperator: func(operands []interface{}) (interface{}, error) {
			if len(operands) != 2 {
				return nil, ParseRuleOperatorError
			}
			if reflect.TypeOf(operands[0]) == reflect.TypeOf(operands[1]) {
				switch operands[0].(type) {
				case bool:
					return operands[0].(bool) || operands[1].(bool), nil
				}
			}
			return nil, ParseRuleOperatorError
		},

		// do the logic AND on two bool values
		AndOperator: func(operands []interface{}) (interface{}, error) {
			if len(operands) != 2 {
				return nil, ParseRuleOperatorError
			}
			if reflect.TypeOf(operands[0]) == reflect.TypeOf(operands[1]) {
				switch operands[0].(type) {
				case bool:
					return operands[0].(bool) && operands[1].(bool), nil
				}
			}
			return nil, ParseRuleOperatorError
		},

		// do the regex match on two parameters,
		RegexMatchOperator: func(operands []interface{}) (interface{}, error) {
			if len(operands) != 2 {
				return nil, ParseRuleOperatorError
			}
			if reflect.TypeOf(operands[0]) == reflect.TypeOf(operands[1]) {
				switch operands[0].(type) {
				case string:
					// regexp pattern operands[0] match check against string, operands[1]
					if match, err := regexp.MatchString(operands[0].(string), operands[1].(string)); err != nil {
						return nil, err
					} else {
						return match, nil
					}
				}
			}
			return nil, ParseRuleOperatorError
		},
	}

	if err := loadSystemRules(); err != nil {
		// panic
		log.Fatal(err)
		panic("system rule load: failed")
	}
}

// Helper function transforms the Unmarshal parsed temporary result, Term
// into OperandList []Operand, and record number of unique field names in the rule.
func ConstructOperandListHelper(t *Term, fieldList map[string]int) (Operand, error) {
	switch v := t.Value.(type) {
	case TermOperand:
		for _, o := range v.ParseOperands {
			if opernd, err := ConstructOperandListHelper(&o, fieldList); err == nil {
				v.OperandList = append(v.OperandList, opernd)
			} else {
				return nil, err
			}
		}
		return &v, nil

	case FieldOperand:
		fieldList[v.Name] = 1
		return &v, nil
	case ValueOperand:
		return &v, nil
	}
	return nil, fmt.Errorf("unknown rule operand, %v", t)
}

// sanity check the rule, then save to the rule register,  AllRegisteredRules
// maintain the RWLock as need
func SaveRuleToRegister(rule Operand, ruleName string, fieldList map[string]int) error {
	count := 0
	var fieldName string
	for k := range fieldList {
		count++
		fieldName = k
	}
	if count != 1 {
		// unique field name in a rule can only have one
		return fmt.Errorf("system rule load: rule name, %s, contains more than one unique field name", ruleName)
	}
	// save rule with ruleName
	RegRuleLock.RLock()    // READ lock
	rules, exists := AllRegisteredRules[fieldName]
	RegRuleLock.RUnlock()  // READ unlock

	if !exists {
		// create a new registered rule
		regRule := map[string]Operand{}
		regRule[ruleName] = rule
		RegRuleLock.Lock()   // WRITE lock
		AllRegisteredRules[fieldName] = regRule
		RegRuleLock.Unlock() // WRITE unlock
	} else {
		if _, exists := rules[ruleName]; exists {
			// duplicated rule name
			return fmt.Errorf("system rule load: rule name, %s, is duplicaed in the field name, %s", ruleName, fieldName)
		} else {
			RegRuleLock.Lock()   // WRITE lock
			rules[ruleName] = rule
			RegRuleLock.Unlock() // WRITE unlock
		}
	}
	return nil
}

// when the system starts up, it tries to load all rules defined in ruleJsonDefinitionFileName.
// AllRegisteredRules manipulation doesn't require to be locked
func loadSystemRules() error {
	jsonFile, err := os.Open(ruleJsonDefinitionFileName)
	if err != nil {
		return err
	}
	defer jsonFile.Close()

	decoder := json.NewDecoder(jsonFile)

	// at open bracket
	if _, err := decoder.Token(); err != nil {
		return err
	}

	// file stream read while the array contains values
	for decoder.More() {
		r := RuleNode{}
		// decode one rule block,
		//    { "name":  _rule_name_, "rule": { _rule_content_ ...} }
		// into a map[string]interface{}
		if err := decoder.Decode(&r); err != nil {
			// failed to decode a JSON block
			return err
		}

		// parse one rule in r
		fieldList := map[string]int{}
		rule, _ := ConstructOperandListHelper(&r.RuleContent, fieldList)
		SaveRuleToRegister(rule, r.Name, fieldList)
	}

	// at closing bracket
	if _, err = decoder.Token(); err != nil {
		return err
	}
	return nil
}

