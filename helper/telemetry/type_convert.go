package telemetry

import (
	"go.opentelemetry.io/otel/attribute"

	"fmt"
)

func convertTypeToAttribute(value interface{}) attribute.Value {
	switch v := value.(type) {
	case string:
		return attribute.StringValue(v)
	case int:
		return attribute.IntValue(v)
	case int8:
		return attribute.IntValue(int(v))
	case int16:
		return attribute.IntValue(int(v))
	case int32:
		return attribute.IntValue(int(v))
	case int64:
		return attribute.Int64Value(v)
	case uint:
		return attribute.Int64Value(int64(v))
	case uint8:
		return attribute.Int64Value(int64(v))
	case uint16:
		return attribute.Int64Value(int64(v))
	case uint32:
		return attribute.Int64Value(int64(v))
	case uint64:
		return attribute.Int64Value(int64(v))
	case float32:
		return attribute.Float64Value(float64(v))
	case float64:
		return attribute.Float64Value(v)
	case bool:
		return attribute.BoolValue(v)
	default:
		return attribute.StringValue(fmt.Sprintf("%v", v))
	}
}
