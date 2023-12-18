# A general-purpose project scaffolding library and tool inspired by [cookiecutter]

[![stability-experimental](https://img.shields.io/badge/stability-experimental-orange.svg)](https://github.com/mkenney/software-guides/blob/master/STABILITY-BADGES.md#experimental) [![Go Reference](https://pkg.go.dev/badge/github.com/TBD54566975/scaffolder.svg)](https://pkg.go.dev/github.com/TBD54566975/scaffolder)

Scaffolder evaluates the scaffolding files at the given destination against
ctx using the following rules:

- Templates are evaluated using the Go template engine.
- Both path names and file contents are evaluated.
- If a file name ends with `.tmpl`, the `.tmpl` suffix is removed.
- If a file or directory name evalutes to the empty string it will be excluded.
- If a file named `template.js` exists in the root of the template directory,
  all functions defined in this file will be available as Go template functions.
- Directory names in templates can be expanded into multiple directories by
  using the `dir` function. This function takes two arguments, the directory name
  and the context to use when evaluating templates under the directory.

## Examples

### Multiple directories

```gotemplate
template/
  {{ range .Modules }}{{ dir .Name  . }}{{ end }}/
    file.txt
```

[cookiecutter]: https://github.com/cookiecutter/cookiecutter
