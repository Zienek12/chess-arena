package uci

import (
	"strconv"
	"strings"
)

// Info is the search telemetry from one "info" line.
type Info struct {
	Depth     int      `json:"depth,omitempty"`
	SelDepth  int      `json:"seldepth,omitempty"`
	ScoreCP   *int     `json:"score_cp,omitempty"`
	ScoreMate *int     `json:"score_mate,omitempty"`
	Nodes     int64    `json:"nodes,omitempty"`
	NPS       int64    `json:"nps,omitempty"`
	TimeMs    int64    `json:"time_ms,omitempty"`
	PV        []string `json:"pv,omitempty"`
}

// ParseInfo parses a UCI "info" line. It returns false for other lines and for
// "info string", which carries free text instead of search data.
func ParseInfo(line string) (Info, bool) {
	fields := strings.Fields(line)
	if len(fields) < 2 || fields[0] != "info" || fields[1] == "string" {
		return Info{}, false
	}
	var info Info
	for index := 1; index < len(fields); index++ {
		next := func() string {
			if index+1 < len(fields) {
				index++
				return fields[index]
			}
			return ""
		}
		switch fields[index] {
		case "depth":
			info.Depth, _ = strconv.Atoi(next())
		case "seldepth":
			info.SelDepth, _ = strconv.Atoi(next())
		case "nodes":
			info.Nodes, _ = strconv.ParseInt(next(), 10, 64)
		case "nps":
			info.NPS, _ = strconv.ParseInt(next(), 10, 64)
		case "time":
			info.TimeMs, _ = strconv.ParseInt(next(), 10, 64)
		case "score":
			kind, value := next(), next()
			if parsed, err := strconv.Atoi(value); err == nil {
				switch kind {
				case "cp":
					info.ScoreCP = &parsed
				case "mate":
					info.ScoreMate = &parsed
				}
			}
		case "pv":
			// pv is always the last field of a line.
			info.PV = append([]string(nil), fields[index+1:]...)
			index = len(fields)
		case "string":
			// Text after "string" is free-form and runs to the end of the line.
			index = len(fields)
		}
	}
	return info, true
}

// hasSearchData reports whether the line describes a search result rather than
// progress such as "info currmove e2e4 currmovenumber 1".
func (info Info) hasSearchData() bool {
	return info.Depth > 0 || info.ScoreCP != nil || info.ScoreMate != nil
}
