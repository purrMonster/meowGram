// This probe is compiled inside the actual v2.0.0 server module exported by git archive.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"meowgram/server/internal/model"
	"os"
)

var sourceCommit string

func main() {
	if len(os.Args) == 2 && os.Args[1] == "source" {
		fmt.Println(sourceCommit)
		return
	}
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: legacy-decoder message|history|live|source")
		os.Exit(2)
	}
	if err := decode(os.Args[1], os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "invalid fixture input")
		os.Exit(1)
	}
}
func decode(mode string, input io.Reader, output io.Writer) error {
	decoder := json.NewDecoder(io.LimitReader(input, 1024*1024))
	var value any
	switch mode {
	case "message":
		value = &model.Message{}
	case "history":
		value = &[]model.Message{}
	case "live":
		value = &model.WSMessage{}
	default:
		return fmt.Errorf("unknown mode")
	}
	if err := decoder.Decode(value); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return fmt.Errorf("expected one JSON value")
	}
	return json.NewEncoder(output).Encode(value)
}
