module github.com/TBD54566975/scaffolder/cmd/scaffolder

go 1.21.5

require (
	github.com/TBD54566975/scaffolder v0.6.2
	github.com/iancoleman/strcase v0.3.0
)

require (
	github.com/dlclark/regexp2 v1.7.0 // indirect
	github.com/dop251/goja v0.0.0-20231027120936-b396bb4c349d // indirect
	github.com/go-sourcemap/sourcemap v2.1.3+incompatible // indirect
	github.com/google/pprof v0.0.0-20230207041349-798e818bf904 // indirect
	golang.org/x/text v0.3.8 // indirect
)

require (
	github.com/TBD54566975/scaffolder/extensions/javascript v0.0.0-00010101000000-000000000000
	github.com/alecthomas/kong v0.8.1
)

replace github.com/TBD54566975/scaffolder/extensions/javascript => ../../extensions/javascript

replace github.com/TBD54566975/scaffolder => ../..
