package domain

import (
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"strings"
)

// valueSchema is the subset of JSON Schema that policy7 interprets when
// validating a parameter's `value` JSONB against its category's value_schema
// (Wave C, PLAN-WC-XUI-CONVENTION). The `x-ui` extension is intentionally NOT
// decoded here — it is a presentation hint consumed by the frontend adapter and
// must be ignored by the authoritative backend validator. The `x-rules`
// extension (cross-field validation) IS decoded and enforced.
type valueSchema struct {
	Type       string                  `json:"type"`
	Required   []string                `json:"required"`
	Properties map[string]*valueSchema `json:"properties"`
	Items      *valueSchema            `json:"items"`
	Enum       []json.RawMessage       `json:"enum"`
	Minimum    *float64                `json:"minimum"`
	Maximum    *float64                `json:"maximum"`
	MinLength  *int                    `json:"minLength"`
	MaxLength  *int                    `json:"maxLength"`
	MinItems   *int                    `json:"minItems"`
	MaxItems   *int                    `json:"maxItems"`
	Pattern    string                  `json:"pattern"`
	XRules     []xRule                 `json:"x-rules"`
}

// xRule is one cross-field rule. For comparison ops (lte/gte/lt/gt/eq) Left and
// Right name two sibling properties. For required-if, Field is required when
// Right (the condition field) is present/non-empty.
type xRule struct {
	Op      string `json:"op"`
	Left    string `json:"left"`
	Right   string `json:"right"`
	Field   string `json:"field"`
	Message string `json:"message"`
}

// FieldError is one validation failure, keyed by the (possibly nested) field path.
type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// SchemaValidationError is returned when a parameter value fails validation
// against its category's value_schema. Handlers map it to HTTP 422.
type SchemaValidationError struct {
	Errors []FieldError
}

func (e *SchemaValidationError) Error() string {
	parts := make([]string, 0, len(e.Errors))
	for _, fe := range e.Errors {
		parts = append(parts, fmt.Sprintf("%s: %s", fe.Field, fe.Message))
	}
	return "value schema validation failed: " + strings.Join(parts, "; ")
}

// CategoryError is returned when a parameter references a category that does not
// exist (or is inactive) in parameter_categories. Category validity is fully
// data-driven: a category is valid iff an active row exists for the org — there
// is no hardcoded allowlist. Handlers map this to HTTP 422 (INVALID_CATEGORY).
type CategoryError struct {
	Code   string
	Reason string
}

func (e *CategoryError) Error() string {
	return fmt.Sprintf("category %q: %s", e.Code, e.Reason)
}

// ValidateValue validates a parameter value against a category value_schema.
//
// It enforces the JSON-Schema subset documented in PLAN-WC-XUI-CONVENTION
// (type, required, enum, minimum/maximum, minLength/maxLength, pattern, array
// items, min/maxItems) plus the `x-rules` cross-field extension (lte/gte/lt/gt/
// eq/required-if). `x-ui` is ignored.
//
// A nil/empty schema means "no schema defined" → no validation (returns nil).
// On failure it returns a *SchemaValidationError listing every violation.
func ValidateValue(schema, value json.RawMessage) error {
	if len(strings.TrimSpace(string(schema))) == 0 || string(schema) == "null" {
		return nil
	}

	var sch valueSchema
	if err := json.Unmarshal(schema, &sch); err != nil {
		// A malformed schema is a configuration problem, not a value problem;
		// fail open so a bad schema cannot block all writes.
		return nil
	}

	var v interface{}
	if len(value) == 0 || string(value) == "null" {
		v = nil
	} else if err := json.Unmarshal(value, &v); err != nil {
		return &SchemaValidationError{Errors: []FieldError{{Field: "value", Message: "value is not valid JSON"}}}
	}

	var errs []FieldError
	validateNode(&sch, v, "value", &errs)

	if len(errs) > 0 {
		return &SchemaValidationError{Errors: errs}
	}
	return nil
}

func validateNode(sch *valueSchema, v interface{}, path string, errs *[]FieldError) {
	switch sch.Type {
	case "object", "":
		obj, ok := v.(map[string]interface{})
		if v != nil && !ok {
			if sch.Type == "object" {
				*errs = append(*errs, FieldError{Field: path, Message: "expected an object"})
				return
			}
		}
		if obj == nil {
			obj = map[string]interface{}{}
		}
		for _, req := range sch.Required {
			val, present := obj[req]
			if !present || isEmpty(val) {
				*errs = append(*errs, FieldError{Field: childPath(path, req), Message: "is required"})
			}
		}
		for name, propSchema := range sch.Properties {
			if val, present := obj[name]; present && val != nil {
				validateNode(propSchema, val, childPath(path, name), errs)
			}
		}
		applyXRules(sch.XRules, obj, path, errs)

	case "array":
		arr, ok := v.([]interface{})
		if !ok {
			*errs = append(*errs, FieldError{Field: path, Message: "expected an array"})
			return
		}
		if sch.MinItems != nil && len(arr) < *sch.MinItems {
			*errs = append(*errs, FieldError{Field: path, Message: fmt.Sprintf("must have at least %d item(s)", *sch.MinItems)})
		}
		if sch.MaxItems != nil && len(arr) > *sch.MaxItems {
			*errs = append(*errs, FieldError{Field: path, Message: fmt.Sprintf("must have at most %d item(s)", *sch.MaxItems)})
		}
		if sch.Items != nil {
			for i, item := range arr {
				validateNode(sch.Items, item, fmt.Sprintf("%s[%d]", path, i), errs)
			}
		}

	case "string":
		s, ok := v.(string)
		if !ok {
			*errs = append(*errs, FieldError{Field: path, Message: "expected a string"})
			return
		}
		if sch.MinLength != nil && len(s) < *sch.MinLength {
			*errs = append(*errs, FieldError{Field: path, Message: fmt.Sprintf("must be at least %d characters", *sch.MinLength)})
		}
		if sch.MaxLength != nil && len(s) > *sch.MaxLength {
			*errs = append(*errs, FieldError{Field: path, Message: fmt.Sprintf("must be at most %d characters", *sch.MaxLength)})
		}
		if sch.Pattern != "" {
			if re, err := regexp.Compile(sch.Pattern); err == nil && !re.MatchString(s) {
				*errs = append(*errs, FieldError{Field: path, Message: "does not match required pattern"})
			}
		}
		checkEnum(sch, v, path, errs)

	case "number", "integer":
		f, ok := toFloat(v)
		if !ok {
			*errs = append(*errs, FieldError{Field: path, Message: "expected a number"})
			return
		}
		if sch.Type == "integer" && f != math.Trunc(f) {
			*errs = append(*errs, FieldError{Field: path, Message: "expected an integer"})
		}
		if sch.Minimum != nil && f < *sch.Minimum {
			*errs = append(*errs, FieldError{Field: path, Message: fmt.Sprintf("must be >= %v", *sch.Minimum)})
		}
		if sch.Maximum != nil && f > *sch.Maximum {
			*errs = append(*errs, FieldError{Field: path, Message: fmt.Sprintf("must be <= %v", *sch.Maximum)})
		}
		checkEnum(sch, v, path, errs)

	case "boolean":
		if _, ok := v.(bool); !ok {
			*errs = append(*errs, FieldError{Field: path, Message: "expected a boolean"})
		}
		checkEnum(sch, v, path, errs)
	}
}

// applyXRules enforces the cross-field x-rules against the decoded object.
func applyXRules(rules []xRule, obj map[string]interface{}, path string, errs *[]FieldError) {
	for _, rule := range rules {
		switch rule.Op {
		case "lte", "gte", "lt", "gt", "eq":
			left, lok := toFloat(obj[rule.Left])
			right, rok := toFloat(obj[rule.Right])
			if !lok || !rok {
				continue // a missing/non-numeric operand is handled by per-field validation
			}
			if !compare(rule.Op, left, right) {
				*errs = append(*errs, FieldError{Field: childPath(path, rule.Left), Message: ruleMessage(rule)})
			}
		case "required-if":
			cond, present := obj[rule.Right]
			if present && !isEmpty(cond) {
				if val, ok := obj[rule.Field]; !ok || isEmpty(val) {
					*errs = append(*errs, FieldError{Field: childPath(path, rule.Field), Message: ruleMessage(rule)})
				}
			}
		}
	}
}

func compare(op string, l, r float64) bool {
	switch op {
	case "lte":
		return l <= r
	case "gte":
		return l >= r
	case "lt":
		return l < r
	case "gt":
		return l > r
	case "eq":
		return l == r
	}
	return true
}

func ruleMessage(rule xRule) string {
	if rule.Message != "" {
		return rule.Message
	}
	if rule.Op == "required-if" {
		return fmt.Sprintf("is required when %s is set", rule.Right)
	}
	return fmt.Sprintf("must be %s %s", rule.Op, rule.Right)
}

func checkEnum(sch *valueSchema, v interface{}, path string, errs *[]FieldError) {
	if len(sch.Enum) == 0 {
		return
	}
	target, err := json.Marshal(v)
	if err != nil {
		return
	}
	for _, e := range sch.Enum {
		if jsonEqual(e, target) {
			return
		}
	}
	*errs = append(*errs, FieldError{Field: path, Message: "is not an allowed value"})
}

// jsonEqual compares two JSON fragments by canonical re-encoding so that
// whitespace and numeric formatting differences do not cause false negatives.
func jsonEqual(a, b json.RawMessage) bool {
	var av, bv interface{}
	if json.Unmarshal(a, &av) != nil || json.Unmarshal(b, &bv) != nil {
		return false
	}
	ab, _ := json.Marshal(av)
	bb, _ := json.Marshal(bv)
	return string(ab) == string(bb)
}

func toFloat(v interface{}) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case json.Number:
		f, err := n.Float64()
		return f, err == nil
	case int:
		return float64(n), true
	}
	return 0, false
}

func isEmpty(v interface{}) bool {
	switch s := v.(type) {
	case nil:
		return true
	case string:
		return strings.TrimSpace(s) == ""
	}
	return false
}

func childPath(parent, child string) string {
	if parent == "" {
		return child
	}
	return parent + "." + child
}
