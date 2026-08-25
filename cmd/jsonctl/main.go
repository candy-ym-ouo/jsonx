package main
import (
	"bytes"
	stdjson "encoding/json"
	"flag"
	"fmt"
	"io"
	"jsonx/jsonx"
	"jsonx/parser"
	"os"
	"strings"
	"time"
)
func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "parse":
		err = parseCommand(os.Args[2:])
	case "format":
		err = formatCommand(os.Args[2:])
	case "validate":
		err = validateCommand(os.Args[2:])
	case "path":
		err = pathCommand(os.Args[2:])
	case "bench":
		err = benchCommand(os.Args[2:])
	case "version":
		fmt.Println(jsonx.Version())
		return
	default:
		usage()
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func usage() {
	fmt.Fprintln(os.Stderr, "usage: jsonctl <parse|format|validate|path|bench|version> [options] file")
}
func readInput(name string) ([]byte, error) {
	if name == "" || name == "-" {
		return io.ReadAll(os.Stdin)
	}
	return os.ReadFile(name)
}
func fileLast(args []string) []string {
	if len(args) < 2 || (args[0] != "-" && strings.HasPrefix(args[0], "-")) {
		return args
	}
	out := append([]string(nil), args[1:]...)
	return append(out, args[0])
}
func parseCommand(args []string) error {
	args = fileLast(args)
	fs := flag.NewFlagSet("parse", flag.ContinueOnError)
	comments := fs.Bool("comments", false, "allow comments")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return fmt.Errorf("parse requires one file")
	}
	data, err := readInput(fs.Arg(0))
	if err != nil {
		return err
	}
	start := time.Now()
	v, err := jsonx.Parse(data, jsonx.AllowComments(*comments))
	if err != nil {
		return err
	}
	nodes, depth := parser.Stats(v)
	fmt.Printf("valid: nodes=%d depth=%d bytes=%d duration=%s\n", nodes, depth, len(data), time.Since(start))
	return nil
}
func formatCommand(args []string) error {
	args = fileLast(args)
	fs := flag.NewFlagSet("format", flag.ContinueOnError)
	spaces := fs.Int("i", 2, "indent spaces; 0 compacts")
	sortKeys := fs.Bool("sort-keys", false, "sort object keys")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return fmt.Errorf("format requires one file")
	}
	data, err := readInput(fs.Arg(0))
	if err != nil {
		return err
	}
	indent := ""
	if *spaces > 0 {
		indent = string(bytes.Repeat([]byte(" "), *spaces))
	}
	out, err := jsonx.Format(data, indent, jsonx.SortKeys(*sortKeys))
	if err != nil {
		return err
	}
	fmt.Println(string(out))
	return nil
}
func validateCommand(args []string) error {
	args = fileLast(args)
	fs := flag.NewFlagSet("validate", flag.ContinueOnError)
	schemaFile := fs.String("schema", "", "schema file")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return fmt.Errorf("validate requires one file")
	}
	data, err := readInput(fs.Arg(0))
	if err != nil {
		return err
	}
	if *schemaFile == "" {
		err = jsonx.Validate(data)
	} else {
		schema, readErr := os.ReadFile(*schemaFile)
		if readErr != nil {
			return readErr
		}
		err = jsonx.ValidateSchema(data, schema)
	}
	if err == nil {
		fmt.Println("valid")
	}
	return err
}
func pathCommand(args []string) error {
	if len(args) != 2 {
		return fmt.Errorf("path requires file and JSONPath")
	}
	data, err := readInput(args[0])
	if err != nil {
		return err
	}
	v, err := jsonx.PathGet(data, args[1])
	if err != nil {
		return err
	}
	out, err := jsonx.Marshal(v)
	if err != nil {
		return err
	}
	fmt.Println(string(out))
	return nil
}
func benchCommand(args []string) error {
	data := []byte(`{"name":"jsonx","values":[1,2,3],"ok":true}`)
	if len(args) > 0 {
		var err error
		data, err = readInput(args[0])
		if err != nil {
			return err
		}
	}
	const iterations = 10000
	start := time.Now()
	for range iterations {
		if _, err := jsonx.Parse(data); err != nil {
			return err
		}
	}
	jsonxDuration := time.Since(start)
	start = time.Now()
	for range iterations {
		var v any
		if err := stdjson.Unmarshal(data, &v); err != nil {
			return err
		}
	}
	standardDuration := time.Since(start)
	fmt.Printf("iterations=%d jsonx=%s encoding/json=%s ratio=%.2fx\n", iterations, jsonxDuration, standardDuration, float64(standardDuration)/float64(jsonxDuration))
	return nil
}
