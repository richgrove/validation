#  JSON Data Validation Implementation 

## 1. Usage

- Requirement:   Go 1.7+ and github.com/go-chi/chi
- Build: under the validation folder, run
```
  go build
```
- It needs the `rules.json` file to load the validation rules.
  
- Validation REST API end-point: POST action
```
  :8000/api/validation
```

## 2. Design 
### 2.1 Validation Rule Engine 
The validation service is a rule-based processing engine.  The system has the pre-loaded validation rules.  Those rules are the internal functional building blocks, called as `operand` for a set of given JSON fields.  When a input JSON data is collected, the JSON fields with the values will be triggered to execute the proper operand.  The evaluation of field value called the executor will be collected for the final validation result.

**Rule**:  a named operand for a given field name

**Operand**: a executable function, and able to be evaluated with the field value.  And, it can have the embedded operands.

Defined as:
```
// rule operator evaluates []interface{} data to 
// generate a single value, interface{}
type OperatorFn func([]interface{}) (interface{}, error)
```
The system built-in operators are: 
```
const (
    LengthOperator      OperatorType = "LENGTH"
    EqualToOperator     OperatorType = "EQUAL_TO"
    GreaterThanOperator OperatorType = "GREATER_THAN"
    OrOperator          OperatorType = "OR"
    AndOperator         OperatorType = "AND"
    RegexMatchOperator  OperatorType = "REGEX_MATCH"
)
```

```
// Operand has the capability to be evaluated by Evaluate() function,
// it can be either a terminated operand like FieldOperand/ValueOperand
// or recursive defined operand like TermOperand.
type Operand interface {
    GetOperator() *OperatorFn
    GetOperands() []Operand
    Evaluate(EvalContext) (interface{}, error)
}
```

There are three operands in the rule engine:

**FieldOperand:** evaluate as the run-time field value, no further operands

**ValueOperand:** evaluate as the literal value, no further operands

**TermOperand:** evaluate by its `OperatorFn` on given `[]Operand` list

### 2.2 Rule Parser
The rule parser is designed to use the `type Term struct` to record the **field/value/term-operand** processing result. The rule JSON definition block is not well-formed JSON data structure.  The Golang standard JSON decoding library can't properly unmarshal. It has to implement a customized `UnmarshalJSON()` to handle the parse.

The keywords in a rule are defined in the `FieldOperand, ValueOperand, TermOperand` and are used in customized `UnmarshalJSON()` when the ahead parsing look knows the operand type. But, their parsed results are stored in the `Term`.  Late, the `Term.Value` has to be stripped out.

This trick works, and is a work-around solution in the short time of implementation. It may need to re-visit a better scheme.

### 2.3 Rule Execution Context
When a TermOperand is evaluated, it needs to know the value of field.  This is implemented in a evaluation context as:
```
// evaluation context: keep the run-time state 
type EvalContext interface {
     GetFieldValue() interface{}
}
```
The current rule engine can only allow a single field to be referred in the rule content. The above interface is available during `FieldOperand` evaluation.

### 2.4 Validation API Service Data Flow

The API service is implementation to have the following steps:
- System Initialization: load the system rules, and initialize built-in operators
- HTTP server at port 8000 routes the POST request at **/api/validation** to HTTP handler, `ValidateInputJSONByRules`
- Handler does:
  1. Read the incoming JSON data, and processes all data fields with their values (include the nested JSON block)
  2. Check the registered rules for each field name, and create the run-time context for found field rules.
  3. Evaluate each context of the collections created in Step 2.
  4. Collect the evaluation for all JSON data fields, and generate the service response data
- HTTP server responds with the result data.

## 3. Implementation Notes

### 3.1 Unit Test
There is a small Go test program `rule_api_test.go` to run the API service unit test.  It uses the Go testing package with httptest.  For some reason, it can't start the httptest HTTP service to run the test cases. I hard-coded the service end-point, and requires to start the API service server.

### 3.2 Built-in Operators and Validation Rules
The supported operators are pre-defined in the `RegisteredOperators map[OperatorType]OperatorFn`.  The `OperatorFn` is the piece of codes to be executed with evaluated operand's values. The validation rules are loaded from the `./rule/rules.json` file at the system initialization. The JSON file loading uses the Go file stream read to retrieve each rule definition, then execute the rule parse before store into the internal rule registry.

The REST API, `/admin/rule` handles the rule CREATE, DELETE, etc. manipulation.

Since the internal rule registry is implemented by the Go map data structure, which is not concurrent safe.  Add the sync.RWMutex as the R/W lock to control the rule registry reader lock/unlock and writer lock/unlock. Only implemented the rule CREATE operation.

### 3.3 Scalability and Performance
When the rule registry becomes very huge, the internal Go map will affect the system performance.  This is due to the map internal implementation, when the Go garbage collector will be triggered, it will touch every map item during the mark and scan phase. It needs to consider the alternate approach.

The JSON data validation evaluation may impact the performance when the JSON data fields are big.  I added the `rule_proc_concurrent.go` to implement the fan-out concurrent execution.

This open topic may be concerned during the system scalability test result to nail down the system characteristics.  In the overview of system integration, it can try to use the server mesh technology in the early deployment.



