// schema-check gives local fixtures the production client's strict schema check.
package main

import (
	"encoding/json"
	"fmt"
	"github.com/pramaan-oss/cold-email/internal/pramaan"
	"io"
	"os"
)

func main() {
	var r struct {
		Capability string          `json:"capability"`
		Input      json.RawMessage `json:"input"`
	}
	if e := json.NewDecoder(io.LimitReader(os.Stdin, 2<<20)).Decode(&r); e != nil {
		fmt.Fprintln(os.Stderr, "Invalid fixture request.")
		os.Exit(1)
	}
	if e := pramaan.ValidateInput(r.Capability, r.Input); e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
}
