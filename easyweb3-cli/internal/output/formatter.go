package output

import (
	"encoding/json"
	"fmt"
	"io"
)

type Format string

const (
	FormatJSON     Format = "json"
	FormatText     Format = "text"
	FormatMarkdown Format = "markdown"
)

func Write(w io.Writer, format Format, v any) error {
	switch format {
	case FormatText, FormatMarkdown:
		// For MVP, just JSON-encode even for text/markdown.
		// We can implement richer formatting later.
		fallthrough
	case FormatJSON, "":
		b, err := json.MarshalIndent(v, "", "  ")
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(w, string(b))
		return err
	default:
		b, err := json.MarshalIndent(v, "", "  ")
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(w, string(b))
		return err
	}
}
