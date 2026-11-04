package fake

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// mdkb shares the runner record format but uses scratch marker files as notes.
func mdkb() bool {
	if filepath.Base(os.Args[0]) != "mdkb" {
		return false
	}
	if message := os.Getenv("FAKE_MDKB_FAIL"); message != "" {
		fmt.Fprintln(os.Stderr, message)
		os.Exit(1)
	}
	root := os.Getenv("MDKB_ROOT")
	args := os.Args[1:]
	if len(args) >= 2 && args[0] == "--root" {
		root = args[1]
		args = args[2:]
	}
	if len(args) > 0 && args[0] == "--fixture" {
		return false
	}
	if len(args) == 0 {
		os.Exit(2)
	}
	if delay := os.Getenv("FAKE_MDKB_" + strings.ToUpper(args[0]) + "_SLEEP"); delay != "" {
		duration, err := time.ParseDuration(delay)
		if err != nil {
			os.Exit(1)
		}
		time.Sleep(duration)
	}
	switch args[0] {
	case "version":
		fmt.Println("mdkb fake-v1")
	case "info":
		if _, err := os.Stat(root); err != nil {
			os.Exit(2)
		}
		_, err := os.Stat(filepath.Join(root, ".mdkb"))
		return json.NewEncoder(os.Stdout).Encode(struct {
			Root        string `json:"root"`
			Initialised bool   `json:"initialized"`
		}{root, err == nil}) == nil
	case "cat":
		if len(args) < 2 {
			os.Exit(2)
		}
		if _, err := os.Stat(filepath.Join(root, ".mdkb")); err != nil {
			os.Exit(3)
		}
		data, err := os.ReadFile(filepath.Join(root, args[1]+".md"))
		if os.IsNotExist(err) {
			os.Exit(2)
		}
		if err != nil {
			os.Exit(1)
		}
		fmt.Print(string(data))
	case "new":
		if _, err := os.Stat(filepath.Join(root, ".mdkb")); err != nil {
			os.Exit(3)
		}
		if len(args) < 2 {
			os.Exit(2)
		}
		dir, body := "", ""
		for i := 2; i+1 < len(args); i += 2 {
			switch args[i] {
			case "--dir":
				dir = args[i+1]
			case "--body":
				body = args[i+1]
			}
		}
		path := filepath.Join(root, dir, strings.ToLower(args[1])+".md")
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			os.Exit(1)
		}
		f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err != nil {
			os.Exit(1)
		}
		_, err = f.WriteString(body)
		closeErr := f.Close()
		if err != nil || closeErr != nil {
			os.Exit(1)
		}
	default:
		os.Exit(2)
	}
	return true
}
