// Command instance-setup provisions private managed services before local startup.
package main

import (
	"context"
	"fmt"
	"github.com/pramaan-oss/cold-email/internal/pramaan"
	"os"
	"time"
)

func main() {
	var client pramaan.Caller
	c, e := pramaan.FromEnv()
	if e == nil {
		client = c
	} else if e != pramaan.ErrNotConfigured {
		fmt.Fprintln(os.Stderr, "Pramaan setup needs a valid platform address.")
		os.Exit(1)
	}
	dir := os.Getenv("PRAMAAN_MANAGED_STATE")
	if dir == "" {
		dir = "/runtime"
	}
	app := os.Getenv("APP_URL")
	if app == "" {
		app = "http://localhost:5173"
	}
	setup := pramaan.ManagedSetup{Client: client, Directory: dir, AppURL: app, Notice: func(s string) { fmt.Println(s) }}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if e = setup.Database(ctx); e == nil {
		e = setup.SignIn(ctx)
	}
	if e != nil {
		fmt.Fprintln(os.Stderr, e.Error())
		os.Exit(1)
	}
}
