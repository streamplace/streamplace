//go:build linux && cgo && stationslate

package stationdb

/*
#include <stdlib.h>
#include <string.h>

// Provided by go-sqlite3's bundled SQLite (or its system SQLite library).
extern int sqlite3_complete(const char *);

static size_t station_statement_end(char *sql) {
	for (char *p = sql; (p = strchr(p, ';')) != NULL; ++p) {
		char after = p[1];
		if (after == '\0') return 0;
		p[1] = '\0';
		int complete = sqlite3_complete(sql);
		p[1] = after;
		if (complete == 1) return (size_t)(p + 1 - sql);
	}
	return 0;
}
*/
import "C"

import (
	"errors"
	"strings"
	"unsafe"
)

// The native Prepare API only executes the first statement and hides its tail.
// Reject batches rather than silently skip SQL; SQLite handles trigger bodies.
func singleStatement(query string) (string, error) {
	if strings.IndexByte(query, 0) >= 0 {
		return "", errors.New("station SQLite statement contains NUL")
	}
	query = sqlBody(query)
	if pos := strings.IndexByte(query, ';'); pos >= 0 && pos < len(query)-1 {
		sql := C.CString(query)
		end := int(C.station_statement_end(sql))
		C.free(unsafe.Pointer(sql))
		if end != 0 {
			if sqlBody(query[end:]) != "" {
				return "", errors.New("station SQLite accepts one statement per call")
			}
			query = query[:end]
		}
	}
	return query, nil
}

func sqlBody(query string) string {
	for {
		query = strings.TrimLeft(query, " \t\n\r\f;")
		switch {
		case strings.HasPrefix(query, "\ufeff"):
			query = query[len("\ufeff"):]
		case strings.HasPrefix(query, "--"):
			if i := strings.IndexByte(query, '\n'); i >= 0 {
				query = query[i+1:]
				continue
			}
			return ""
		case strings.HasPrefix(query, "/*"):
			if i := strings.Index(query[2:], "*/"); i >= 0 {
				query = query[i+4:]
				continue
			}
			return ""
		default:
			return query
		}
	}
}
