package override

import (
	"fmt"
	"net/url"
	"strings"
)

// parseSlackURL extracts the channel ID and message timestamp from a Slack
// message URL of the form:
//
//	https://<workspace>.slack.com/archives/<channelID>/p<ts_no_dot>
//
// Timestamp reconstruction uses string manipulation: strip the 'p' prefix and
// insert '.' before the last 6 digits. No float parsing to avoid precision loss.
func parseSlackURL(rawURL string) (channel, ts string, err error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", "", fmt.Errorf("invalid slack url: %w", err)
	}

	// Find the /archives/ segment.
	parts := strings.Split(strings.TrimPrefix(u.Path, "/"), "/")
	archivesIdx := -1
	for i, p := range parts {
		if p == "archives" {
			archivesIdx = i
			break
		}
	}
	if archivesIdx < 0 || len(parts) < archivesIdx+2 {
		return "", "", fmt.Errorf("invalid slack url: missing /archives/ path")
	}

	channelSeg := parts[archivesIdx+1]
	if len(channelSeg) == 0 || (channelSeg[0] != 'C' && channelSeg[0] != 'D') {
		return "", "", fmt.Errorf("invalid slack url: channel must start with C or D")
	}

	if len(parts) < archivesIdx+3 || parts[archivesIdx+2] == "" {
		return "", "", fmt.Errorf("invalid slack url: missing timestamp segment")
	}

	tsSeg := parts[archivesIdx+2]
	// Must start with 'p' and have at least 8 chars total (p + at least 7 digits).
	if len(tsSeg) < 8 || tsSeg[0] != 'p' {
		return "", "", fmt.Errorf("invalid slack url: timestamp segment too short")
	}

	// Strip 'p' and insert '.' before the last 6 digits.
	digits := tsSeg[1:]
	ts = digits[:len(digits)-6] + "." + digits[len(digits)-6:]

	return channelSeg, ts, nil
}
