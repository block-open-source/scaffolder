package main

import (
	"encoding/json"
	"html/template"
	"os"
	"reflect"
	"strings"

	"github.com/alecthomas/kong"
	"github.com/iancoleman/strcase"

	"github.com/TBD54566975/scaffolder"
)

var cli struct {
	JSON *os.File `help:"JSON file containing the context to use."`
	Dir  string   `arg:"" help:"Directory to scaffold."`
}

func main() {
	kctx := kong.Parse(&cli)
	context := json.RawMessage{}
	if cli.JSON != nil {
		if err := json.NewDecoder(cli.JSON).Decode(&context); err != nil {
			kctx.FatalIfErrorf(err, "failed to decode JSON")
		}
	}
	err := scaffolder.Scaffold(cli.Dir, context, scaffolder.Functions(template.FuncMap{
		"snake":          strcase.ToSnake,
		"screamingSnake": strcase.ToScreamingSnake,
		"camel":          strcase.ToCamel,
		"lowerCamel":     strcase.ToLowerCamel,
		"kebab":          strcase.ToKebab,
		"screamingKebab": strcase.ToScreamingKebab,
		"upper":          strings.ToUpper,
		"lower":          strings.ToLower,
		"title":          strings.Title,
		"typename": func(v any) string {
			return reflect.Indirect(reflect.ValueOf(v)).Type().Name()
		},
	}))
	kctx.FatalIfErrorf(err)
}
