# A general-purpose project scaffolding library and tool inspired by [cookiecutter]

[![stability-experimental](https://img.shields.io/badge/stability-experimental-orange.svg)](https://github.com/mkenney/software-guides/blob/master/STABILITY-BADGES.md#experimental) [![Go Reference](https://pkg.go.dev/badge/github.com/TBD54566975/scaffolder.svg)](https://pkg.go.dev/github.com/TBD54566975/scaffolder)

Scaffolder evaluates the scaffolding files at the given destination against
ctx:

- Both path names and file contents are evaluated.
- If a file name ends with `.tmpl`, the `.tmpl` suffix is removed.
- If a file or directory name evalutes to the empty string it will be excluded.

[cookiecutter]: https://github.com/cookiecutter/cookiecutter
