package domain

import (
	"encoding/json"
	"errors"
	"testing"
)

// transactionLimitSchema mirrors the seeded transaction_limit value_schema:
// two-limit pair + x-rules lte (authorization_limit <= transaction_limit).
const transactionLimitSchema = `{
  "type": "object",
  "required": ["transaction_limit", "authorization_limit", "currency"],
  "properties": {
    "transaction_limit":   { "type": "number", "minimum": 0 },
    "authorization_limit": { "type": "number", "minimum": 0 },
    "currency": { "type": "string", "enum": ["IDR"] },
    "scope": { "type": "string", "enum": ["per_transaction", "per_day", "per_month"] }
  },
  "x-rules": [
    { "op": "lte", "left": "authorization_limit", "right": "transaction_limit",
      "message": "Authorization limit must be <= transaction limit" }
  ]
}`

func asSchemaErr(t *testing.T, err error) *SchemaValidationError {
	t.Helper()
	if err == nil {
		t.Fatalf("expected a validation error, got nil")
	}
	var sErr *SchemaValidationError
	if !errors.As(err, &sErr) {
		t.Fatalf("expected *SchemaValidationError, got %T: %v", err, err)
	}
	return sErr
}

func TestValidateValue_NilSchemaSkips(t *testing.T) {
	if err := ValidateValue(nil, json.RawMessage(`{"anything":1}`)); err != nil {
		t.Fatalf("nil schema should skip validation, got %v", err)
	}
	if err := ValidateValue(json.RawMessage(`null`), json.RawMessage(`{}`)); err != nil {
		t.Fatalf("null schema should skip validation, got %v", err)
	}
}

func TestValidateValue_TwoLimitValid(t *testing.T) {
	value := `{"transaction_limit":100000000,"authorization_limit":25000000,"currency":"IDR","scope":"per_transaction"}`
	if err := ValidateValue(json.RawMessage(transactionLimitSchema), json.RawMessage(value)); err != nil {
		t.Fatalf("expected valid two-limit payload to pass, got %v", err)
	}
}

func TestValidateValue_XRuleViolation(t *testing.T) {
	// authorization_limit (30jt) > transaction_limit (25jt) violates lte.
	value := `{"transaction_limit":25000000,"authorization_limit":30000000,"currency":"IDR"}`
	err := ValidateValue(json.RawMessage(transactionLimitSchema), json.RawMessage(value))
	sErr := asSchemaErr(t, err)
	found := false
	for _, fe := range sErr.Errors {
		if fe.Field == "value.authorization_limit" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected an x-rule violation on authorization_limit, got %+v", sErr.Errors)
	}
}

func TestValidateValue_MissingRequired(t *testing.T) {
	value := `{"transaction_limit":100}`
	err := ValidateValue(json.RawMessage(transactionLimitSchema), json.RawMessage(value))
	sErr := asSchemaErr(t, err)
	if len(sErr.Errors) == 0 {
		t.Fatalf("expected required-field violations")
	}
}

func TestValidateValue_EnumAndType(t *testing.T) {
	value := `{"transaction_limit":100,"authorization_limit":50,"currency":"USD"}`
	err := ValidateValue(json.RawMessage(transactionLimitSchema), json.RawMessage(value))
	sErr := asSchemaErr(t, err)
	found := false
	for _, fe := range sErr.Errors {
		if fe.Field == "value.currency" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected currency enum violation, got %+v", sErr.Errors)
	}
}

func TestValidateValue_Minimum(t *testing.T) {
	value := `{"transaction_limit":-1,"authorization_limit":0,"currency":"IDR"}`
	err := ValidateValue(json.RawMessage(transactionLimitSchema), json.RawMessage(value))
	sErr := asSchemaErr(t, err)
	found := false
	for _, fe := range sErr.Errors {
		if fe.Field == "value.transaction_limit" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected minimum violation on transaction_limit, got %+v", sErr.Errors)
	}
}

// arraySchema exercises the detail-rows array-of-object validation path.
const arraySchema = `{
  "type": "object",
  "required": ["tiers"],
  "properties": {
    "tiers": {
      "type": "array",
      "minItems": 1,
      "items": {
        "type": "object",
        "required": ["tenor_months", "rate"],
        "properties": {
          "tenor_months": { "type": "integer", "minimum": 0 },
          "rate": { "type": "number", "minimum": 0 }
        }
      }
    }
  }
}`

func TestValidateValue_ArrayItemsValid(t *testing.T) {
	value := `{"tiers":[{"tenor_months":3,"rate":4.5},{"tenor_months":6,"rate":5.0}]}`
	if err := ValidateValue(json.RawMessage(arraySchema), json.RawMessage(value)); err != nil {
		t.Fatalf("expected valid tiers to pass, got %v", err)
	}
}

func TestValidateValue_ArrayItemViolation(t *testing.T) {
	// Second tier missing rate, and first has a non-integer tenor.
	value := `{"tiers":[{"tenor_months":3.5,"rate":4.5},{"tenor_months":6}]}`
	err := ValidateValue(json.RawMessage(arraySchema), json.RawMessage(value))
	sErr := asSchemaErr(t, err)
	var sawInteger, sawRequired bool
	for _, fe := range sErr.Errors {
		if fe.Field == "value.tiers[0].tenor_months" {
			sawInteger = true
		}
		if fe.Field == "value.tiers[1].rate" {
			sawRequired = true
		}
	}
	if !sawInteger || !sawRequired {
		t.Fatalf("expected per-item violations, got %+v", sErr.Errors)
	}
}

func TestValidateValue_EmptyArrayMinItems(t *testing.T) {
	value := `{"tiers":[]}`
	err := ValidateValue(json.RawMessage(arraySchema), json.RawMessage(value))
	asSchemaErr(t, err)
}
