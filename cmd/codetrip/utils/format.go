package utils

import (
	"encoding/json"
	"fmt"

	"github.com/mengshi02/codetrip"
)

// FormatValue formats a query result value for display.
func FormatValue(v any) string {
	switch val := v.(type) {
	case map[string]any:
		if label, ok := val["label"].(string); ok {
			if name, ok := val["name"].(string); ok {
				if fp, ok := val["filePath"].(string); ok {
					return fmt.Sprintf("(%s:%s %s)", label, name, fp)
				}
				return fmt.Sprintf("(%s:%s)", label, name)
			}
		}
		if typ, ok := val["type"].(string); ok {
			return fmt.Sprintf("-[%s]->", typ)
		}
		b, _ := json.Marshal(val)
		return string(b)
	case nil:
		return "NULL"
	default:
		return fmt.Sprintf("%v", v)
	}
}

// FormatResultAsText formats a Cypher result into plain text for MCP responses.
func FormatResultAsText(result *codetrip.CypherResult) string {
	if len(result.Rows) == 0 {
		return "(no results)"
	}

	var out string
	for _, row := range result.Rows {
		vals := make([]string, 0, len(result.Columns))
		for _, col := range result.Columns {
			v := row[col]
			vals = append(vals, FormatValue(v))
		}
		out += fmt.Sprintf("%s\n", vals)
	}
	out += fmt.Sprintf("\n(%d rows)", len(result.Rows))
	return out
}