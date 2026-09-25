package resourcefilter

import (
	"context"
	"encoding/json"
	"regexp"
	"strings"
	"testing"

	"github.com/SigNoz/signoz/pkg/querybuilder"
	qbtypes "github.com/SigNoz/signoz/pkg/types/querybuildertypes/querybuildertypesv5"
	"github.com/SigNoz/signoz/pkg/types/telemetrytypes"
	"github.com/huandu/go-sqlbuilder"
	"github.com/stretchr/testify/require"
)

// clickHouseLike mirrors ClickHouse LIKE: % is any run, _ is one character and
// a backslash escapes the next character.
func clickHouseLike(pattern, s string) bool {
	var b strings.Builder
	b.WriteString("(?s)^")
	runes := []rune(pattern)
	for i := 0; i < len(runes); i++ {
		switch r := runes[i]; r {
		case '%':
			b.WriteString(".*")
		case '_':
			b.WriteString(".")
		case '\\':
			if i+1 < len(runes) {
				i++
				b.WriteString(regexp.QuoteMeta(string(runes[i])))
			} else {
				b.WriteString(regexp.QuoteMeta(`\`))
			}
		default:
			b.WriteString(regexp.QuoteMeta(string(r)))
		}
	}
	b.WriteString("$")
	return regexp.MustCompile(b.String()).MatchString(s)
}

// The labels column holds the collector's json.Marshal of the resource map, so
// every positive LIKE hint must match the stored text of a row whose value
// satisfies the filter, even when JSON escapes characters of that value.
func TestPositiveIndexHintsMatchJSONEscapedValues(t *testing.T) {
	cases := []struct {
		name   string
		key    string
		op     qbtypes.FilterOperator
		value  any
		stored string
	}{
		{"equal with backslashes", "process.executable.path", qbtypes.FilterOperatorEqual, `C:\app\svc.exe`, `C:\app\svc.exe`},
		{"equal with quotes", "team", qbtypes.FilterOperatorEqual, `say "hi"`, `say "hi"`},
		{"equal with html characters", "team", qbtypes.FilterOperatorEqual, `r&d <core>`, `r&d <core>`},
		{"in with quotes", "team", qbtypes.FilterOperatorIn, []any{`say "hi"`}, `say "hi"`},
		{"contains with quotes", "team", qbtypes.FilterOperatorContains, `"hi"`, `say "hi"`},
		{"like with quotes", "team", qbtypes.FilterOperatorLike, `say "%`, `say "hi"`},
	}
	storage := newStorage()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			raw, err := json.Marshal(map[string]string{"service.name": "svc", tc.key: tc.stored})
			require.NoError(t, err)
			labels := string(raw)
			key := &telemetrytypes.TelemetryFieldKey{Name: tc.key, FieldContext: telemetrytypes.FieldContextResource}
			sb := sqlbuilder.NewSelectBuilder()
			conds, _, err := querybuilder.Conditions(context.Background(), qbtypes.QueryInfo{}, storage, key, tc.op, tc.value, map[string][]*telemetrytypes.TelemetryFieldKey{tc.key: {key}}, false, sb)
			require.NoError(t, err)
			sb.Where(conds...)
			sql, args := sb.BuildWithFlavor(sqlbuilder.ClickHouse)
			parts := strings.Split(sql, "?")
			checked := 0
			for i := 0; i < len(parts)-1; i++ {
				switch {
				case strings.HasSuffix(parts[i], "LOWER(labels) LIKE LOWER("):
					hint := strings.ToLower(args[i].(string))
					require.True(t, clickHouseLike(hint, strings.ToLower(labels)), "LOWER(labels) LIKE LOWER(%q) misses %s", args[i], labels)
					checked++
				case strings.HasSuffix(parts[i], "labels LIKE "):
					require.True(t, clickHouseLike(args[i].(string), labels), "labels LIKE %q misses %s", args[i], labels)
					checked++
				}
			}
			require.Positive(t, checked)
		})
	}
}
